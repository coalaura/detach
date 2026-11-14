# detach

Run a command in the background, detached from your terminal. Supports Linux and Windows.

## Install

Download the binary for your platform from [GitHub Releases](https://github.com/coalaura/detach/releases/latest) and place it in a directory on your `PATH`.

## Usage

```sh
detach <executable> [arguments...]
detach python worker.py
```

Prints the process ID and a temporary log file path, then exits immediately. Both stdout and stderr go to the log; stdin is disconnected. The child inherits your current directory and environment.

Commands run directly, without a shell. To use pipes or redirection, invoke a shell explicitly.

```sh
detach version
```

### Windows desktop session

```powershell
detach --desktop "C:\Program Files\uvnc bvba\UltraVNC\winvnc.exe"
```

`--desktop` launches in an existing interactive Windows session belonging to the account running `detach`. This is useful over SSH when the same user is already logged into the desktop. The executable is resolved using the caller's path and the child uses the caller's working directory and the desktop session's environment. Arguments are passed directly, without a shell. The PID and log path are reported after the desktop helper starts the child.

The launch uses Windows Task Scheduler with a temporary, password-free interactive task. Normal callers request normal privileges; elevated callers request the highest available privileges for that user. `detach` does not prompt for elevation. The caller is responsible for running it elevated if required by the program or local policy. Task Scheduler must be available and the user must be permitted to register and run their own interactive tasks. If the account has multiple interactive sessions, Windows chooses the session.

The temporary task and handoff files are removed on success or handled failure; the output log is retained. A desktop launch that is not confirmed within 30 seconds reports an error; its launch status is unknown, so check before retrying. A crash or power loss can leave a `detach-desktop-*` task or temporary directory behind; cleanup failures are reported. The executable and working directory must be accessible from the desktop session.

This mode cannot create a desktop before login. A locked session still exists, but access to the lock screen or UAC prompts depends on the launched program; use UltraVNC's service mode when that access is needed. `--desktop` is Windows-only. Use `detach -- <executable> [arguments...]` to launch an executable whose name matches a `detach` option or `version`.
