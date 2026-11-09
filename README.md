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
