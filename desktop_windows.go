package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	desktopTimeout = 30 * time.Second
	taskTimeout    = 15 * time.Second
)

type desktopRequest struct {
	Executable string
	Arguments  []string
	Directory  string
	Deadline   time.Time
}

type desktopResponse struct {
	Result launchResult
	Error  string
}

type desktopTask struct {
	XMLName                    xml.Name         `xml:"Task"`
	XMLNS                      string           `xml:"xmlns,attr"`
	Version                    string           `xml:"version,attr"`
	Principal                  desktopPrincipal `xml:"Principals>Principal"`
	MultipleInstancesPolicy    string           `xml:"Settings>MultipleInstancesPolicy"`
	DisallowStartIfOnBatteries bool             `xml:"Settings>DisallowStartIfOnBatteries"`
	StopIfGoingOnBatteries     bool             `xml:"Settings>StopIfGoingOnBatteries"`
	AllowStartOnDemand         bool             `xml:"Settings>AllowStartOnDemand"`
	Enabled                    bool             `xml:"Settings>Enabled"`
	ExecutionTimeLimit         string           `xml:"Settings>ExecutionTimeLimit"`
	Action                     desktopAction    `xml:"Actions"`
}

type desktopPrincipal struct {
	ID        string `xml:"id,attr"`
	UserID    string `xml:"UserId"`
	LogonType string `xml:"LogonType"`
	RunLevel  string `xml:"RunLevel"`
}

type desktopAction struct {
	Context   string `xml:"Context,attr"`
	Command   string `xml:"Exec>Command"`
	Arguments string `xml:"Exec>Arguments"`
}

func startDesktop(executable string, arguments []string) (result launchResult, err error) {
	executable, err = exec.LookPath(executable)
	if err != nil {
		return result, fmt.Errorf("find executable: %w", err)
	}

	executable, err = filepath.Abs(executable)
	if err != nil {
		return result, fmt.Errorf("resolve executable: %w", err)
	}

	directory, err := os.Getwd()
	if err != nil {
		return result, fmt.Errorf("get working directory: %w", err)
	}

	helper, err := os.Executable()
	if err != nil {
		return result, fmt.Errorf("locate desktop helper: %w", err)
	}

	principal, err := currentDesktopPrincipal()
	if err != nil {
		return result, err
	}

	scheduler, err := taskSchedulerPath()
	if err != nil {
		return result, err
	}

	workspace, err := os.MkdirTemp("", "detach-desktop-")
	if err != nil {
		return result, fmt.Errorf("create desktop handoff directory: %w", err)
	}

	defer func() {
		cleanupErr := os.RemoveAll(workspace)
		if cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove desktop handoff directory %s: %w", workspace, cleanupErr))
		}
	}()

	request := desktopRequest{
		Executable: executable,
		Arguments:  arguments,
		Directory:  directory,
		Deadline:   time.Now().Add(desktopTimeout),
	}

	err = writeDesktopJSON(filepath.Join(workspace, "request.json"), request)
	if err != nil {
		return result, fmt.Errorf("write desktop request: %w", err)
	}

	taskPath := filepath.Join(workspace, "task.xml")

	err = writeDesktopTask(taskPath, helper, workspace, principal)
	if err != nil {
		return result, fmt.Errorf("write desktop task: %w", err)
	}

	taskName := "detach-desktop-" + rand.Text()

	var (
		registered            bool
		registrationUncertain bool
	)

	defer func() {
		// Also attempt deletion after an interrupted registration, whose outcome may be unknown.
		cleanupErr := runTaskScheduler(scheduler, "/Delete", "/TN", taskName, "/F")
		if cleanupErr != nil && (registered || registrationUncertain) {
			err = errors.Join(err, fmt.Errorf("remove temporary task %s: %w", taskName, cleanupErr))
		}
	}()

	err = runTaskScheduler(scheduler, "/Create", "/TN", taskName, "/XML", taskPath)
	if err != nil {
		registrationUncertain = errors.Is(err, context.DeadlineExceeded)

		return result, fmt.Errorf("register desktop task for the current user: %w", err)
	}

	registered = true

	err = runTaskScheduler(scheduler, "/Run", "/TN", taskName)
	if err != nil {
		return result, fmt.Errorf("run desktop task: %w", err)
	}

	return waitDesktopResponse(workspace, request.Deadline)
}

func runDesktopHelper(arguments []string) error {
	if len(arguments) != 1 {
		return fmt.Errorf("invalid desktop helper arguments")
	}

	workspace := arguments[0]

	data, err := os.ReadFile(filepath.Join(workspace, "request.json"))
	if err != nil {
		return fmt.Errorf("read desktop request: %w", err)
	}

	var request desktopRequest

	err = json.Unmarshal(data, &request)
	if err != nil {
		return fmt.Errorf("decode desktop request: %w", err)
	}

	response := launchDesktopRequest(request)

	return writeDesktopJSON(filepath.Join(workspace, "response.json"), response)
}

func currentDesktopPrincipal() (desktopPrincipal, error) {
	process, err := syscall.GetCurrentProcess()
	if err != nil {
		return desktopPrincipal{}, fmt.Errorf("get current process: %w", err)
	}

	var token syscall.Token

	err = syscall.OpenProcessToken(process, syscall.TOKEN_QUERY, &token)
	if err != nil {
		return desktopPrincipal{}, fmt.Errorf("open current user token: %w", err)
	}

	defer token.Close()

	user, err := token.GetTokenUser()
	if err != nil {
		return desktopPrincipal{}, fmt.Errorf("get current user: %w", err)
	}

	sid, err := user.User.Sid.String()
	if err != nil {
		return desktopPrincipal{}, fmt.Errorf("get current user SID: %w", err)
	}

	var (
		elevated uint32
		returned uint32
	)

	err = syscall.GetTokenInformation(token, syscall.TokenElevation, (*byte)(unsafe.Pointer(&elevated)), uint32(unsafe.Sizeof(elevated)), &returned)
	if err != nil {
		return desktopPrincipal{}, fmt.Errorf("get current elevation: %w", err)
	}

	principal := desktopPrincipal{
		ID:        "user",
		UserID:    sid,
		LogonType: "InteractiveToken",
		RunLevel:  "LeastPrivilege",
	}

	if elevated != 0 {
		principal.RunLevel = "HighestAvailable"
	}

	return principal, nil
}

func taskSchedulerPath() (string, error) {
	// Resolve the Windows directory through the OS, not the caller's PATH or environment.
	getSystemDirectory := syscall.NewLazyDLL("kernel32.dll").NewProc("GetSystemDirectoryW")

	var buffer [syscall.MAX_PATH + 1]uint16

	length, _, callErr := getSystemDirectory.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if length == 0 {
		return "", fmt.Errorf("get system directory: %w", callErr)
	}

	if length >= uintptr(len(buffer)) {
		return "", fmt.Errorf("system directory path is too long")
	}

	return filepath.Join(syscall.UTF16ToString(buffer[:length]), "schtasks.exe"), nil
}

func writeDesktopTask(path string, helper string, workspace string, principal desktopPrincipal) error {
	task := desktopTask{
		XMLNS:                      "http://schemas.microsoft.com/windows/2004/02/mit/task",
		Version:                    "1.2",
		Principal:                  principal,
		MultipleInstancesPolicy:    "IgnoreNew",
		DisallowStartIfOnBatteries: false,
		StopIfGoingOnBatteries:     false,
		AllowStartOnDemand:         true,
		Enabled:                    true,
		ExecutionTimeLimit:         "PT1M",
		Action: desktopAction{
			Context:   principal.ID,
			Command:   helper,
			Arguments: "--desktop-helper " + syscall.EscapeArg(workspace),
		},
	}

	data, err := xml.Marshal(task)
	if err != nil {
		return err
	}

	// Task Scheduler consumes UTF-16 XML; include a BOM for schtasks' file reader.
	words := utf16.Encode([]rune(string(data)))
	encoded := make([]byte, 2, 2+len(words)*2)
	encoded[0] = 0xff
	encoded[1] = 0xfe

	for _, word := range words {
		encoded = binary.LittleEndian.AppendUint16(encoded, word)
	}

	return os.WriteFile(path, encoded, 0o600)
}

func runTaskScheduler(scheduler string, arguments ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), taskTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, scheduler, arguments...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}

	output, err := cmd.CombinedOutput()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

func launchDesktopRequest(request desktopRequest) desktopResponse {
	if !time.Now().Before(request.Deadline) {
		return desktopResponse{Error: "desktop launch request expired"}
	}

	err := os.Chdir(request.Directory)
	if err != nil {
		return desktopResponse{Error: fmt.Sprintf("set desktop working directory: %v", err)}
	}

	result, err := startProcess(request.Executable, request.Arguments)
	response := desktopResponse{Result: result}

	if err != nil {
		response.Error = err.Error()
	}

	return response
}

func waitDesktopResponse(workspace string, deadline time.Time) (launchResult, error) {
	responsePath := filepath.Join(workspace, "response.json")
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		data, err := os.ReadFile(responsePath)
		if err == nil {
			var response desktopResponse

			err = json.Unmarshal(data, &response)
			if err != nil {
				return launchResult{}, fmt.Errorf("decode desktop response: %w", err)
			}

			if response.Error != "" {
				return response.Result, errors.New(response.Error)
			}

			if response.Result.PID <= 0 || response.Result.Log == "" {
				return launchResult{}, fmt.Errorf("desktop helper returned an invalid launch result")
			}

			return response.Result, nil
		}

		if !errors.Is(err, os.ErrNotExist) {
			return launchResult{}, fmt.Errorf("read desktop response: %w", err)
		}

		select {
		case <-timer.C:
			return launchResult{}, fmt.Errorf("desktop launch was not confirmed within %s; make sure the current user is logged into a Windows desktop and can run interactive scheduled tasks", desktopTimeout)
		case <-ticker.C:
		}
	}
}

func writeDesktopJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	// Publish only complete messages so the polling process never sees partial JSON.
	temporary := path + ".tmp"

	err = os.WriteFile(temporary, data, 0o600)
	if err != nil {
		return err
	}

	return os.Rename(temporary, path)
}
