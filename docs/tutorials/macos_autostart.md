# Running Podman on macOS startup with launchd

On macOS, Podman requires a virtual machine to run containers. By default, you must manually start this VM with `podman machine start` each time you log in. This tutorial shows how to use macOS's `launchd` service manager to automatically start your Podman machine at login.

## Prerequisites

- macOS with Podman installed (see the [installation instructions](https://podman.io/docs/installation))
- A fully working Podman machine: initialized (`podman machine init`) and able to start manually (`podman machine start`)

## Overview

macOS uses `launchd` to manage services. User-level services are defined by property list (plist) files placed in `~/Library/LaunchAgents/`. These `LaunchAgents` run when you log in and stop when you log out.

## Step 1: Find your Podman binary path

launchd does not inherit your shell's `PATH`, so the plist must use the full path to the `podman` binary. Determine it with:

```
$ which podman
/opt/podman/bin/podman
```

If you installed Podman with Homebrew, the path is typically `/opt/homebrew/bin/podman`. If you installed Podman using the installer from the [release page](https://github.com/containers/podman/releases), the path is `/opt/podman/bin/podman`.

## Step 2: Create the LaunchAgent plist

Create the file `~/Library/LaunchAgents/com.podman.machine.default.plist` with the following content. Replace `/opt/podman/bin/podman` with the path from Step 1 if it differs on your system.

> **Note:** Running `podman machine start` without specifying a machine name will automatically start the default machine. The example below explicitly uses `podman-machine-default`; replace it with the name of your machine if you are not using the default.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.podman.machine.default</string>

    <key>ProgramArguments</key>
    <array>
        <string>/opt/podman/bin/podman</string>
        <string>machine</string>
        <string>start</string>
        <string>podman-machine-default</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>AbandonProcessGroup</key>
    <true/>
</dict>
</plist>
```

### Explanation of the plist keys

| Key | Purpose |
|-----|---------|
| `Label` | A unique identifier for the service. |
| `ProgramArguments` | The command and arguments to run. Each argument is a separate array element. |
| `RunAtLoad` | Start the command as soon as the agent is loaded (at login). |
| `AbandonProcessGroup` | When the main process (`podman machine start`) exits, launchd leaves its child processes (e.g., the VM and networking) running instead of killing them. **Required** for this use case because `podman machine start` is a one-shot command that starts the VM and then exits. Do **not** use `KeepAlive` here: that would make launchd restart the process after it exits, find the machine already running, and create a restart loop. |

## Step 3: Load the LaunchAgent

Load the agent so it takes effect without logging out and back in. The recommended commands (since macOS 10.10) are `bootstrap` and `bootout`; the older `load` and `unload` subcommands are deprecated.

```
$ launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.podman.machine.default.plist
```

The Podman machine should start. You can verify with:

```
$ podman machine list
NAME                    VM TYPE     CREATED      LAST UP             CPUS    MEMORY      DISK SIZE
podman-machine-default  applehv     2 days ago   Currently running   4       2GiB        100GiB
```

## Managing the LaunchAgent

### Stop the machine from auto-starting

To prevent the Podman machine from starting at login, boot out the agent:

```
$ launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.podman.machine.default.plist
```

### Re-enable auto-starting

```
$ launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.podman.machine.default.plist
```

### Remove the LaunchAgent entirely

```
$ launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.podman.machine.default.plist
$ rm ~/Library/LaunchAgents/com.podman.machine.default.plist
```

### Graceful stop before shutdown or logout

When you log out or shut down, the system will terminate the VM processes. If you prefer to stop the Podman machine gracefully (e.g. to avoid a hard kill), run before shutting down or logging out:

```
$ podman machine stop
```

> **Note:** There is no built-in way to run actions on logout, shutdown, or restart in macOS. The [`LogoutHook`](https://superuser.com/questions/295924/how-to-run-a-script-at-login-logout-in-os-x) can be used, but it is deprecated.

## Troubleshooting

### The machine does not start or exits immediately

1. Ensure `AbandonProcessGroup` is set to `true` in the plist. Without it, launchd kills the process group when `podman machine start` exits, which stops the VM and related processes.
2. Check that the `podman` path in the plist is correct (`which podman`).
3. Verify the machine starts correctly when running `podman machine start` manually in a terminal.
4. Ensure the machine has been initialized with `podman machine init`.

### The machine enters a restart loop

This typically occurs when `KeepAlive` is set to `true` in the plist. Edit the plist file, remove the `KeepAlive` key and its value, then boot out and bootstrap again:

```
$ launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.podman.machine.default.plist
$ launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.podman.machine.default.plist
```

### Debugging with logs

To capture stdout and stderr from the launchd job, add these keys to the plist (create the log paths if needed):

```xml
<key>StandardOutPath</key>
<string>/tmp/podman-machine-start.log</string>
<key>StandardErrorPath</key>
<string>/tmp/podman-machine-start.err.log</string>
```

Then inspect the files:

```
$ cat /tmp/podman-machine-start.log
$ cat /tmp/podman-machine-start.err.log
```

## SEE ALSO

**[podman-machine-start(1)](../source/markdown/podman-machine-start.1.md.in)**, **[podman-machine(1)](../source/markdown/podman-machine.1.md)**

Apple's [launchd documentation](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html)
