% podman-completion 1

## NAME
podman\-completion - Generate shell completion scripts

## SYNOPSIS
**podman completion** [*options*]   *bash* | *zsh* | *fish* | *powershell*

## DESCRIPTION
**podman completion** generates shell completion scripts for a variety of shells. Supported shells are *bash*, *zsh*, *fish* and *powershell*.

These script are used by the shell to provide suggestions and complete commands when the command is typed and `[TAB]` is pressed.

Usually these scripts are automatically installed via the package manager.

## OPTIONS
#### **--file**, **-f**=*file*

Write the generated output to a file.

#### **--no-desc**

Do not provide description in the completions.\
The default is **false**.

## Installation

### BASH
`bash-completion` has to be installed on the system.

To load the completion script into the current session run:
```
source <(podman completion bash)
```

To make it available for all bash sessions run:
```
podman completion -f /etc/bash_completion.d/podman bash
```


### ZSH
Shell completion needs to be already enabled in the environment. The following can be executed:
```
echo "autoload -U compinit; compinit" >> ~/.zshrc
```

To make it available for all zsh sessions run:
```
podman completion -f "${fpath[1]}/_podman" zsh
```

Once the shell is reloaded the auto-completion works.


### FISH
To load the completion script into the current session run:
```
podman completion fish | source
```

To make it available for all fish sessions run:
```
podman completion -f ~/.config/fish/completions/podman.fish fish
```

### POWERSHELL
To load the completion script into the current session run:
```
podman.exe completion powershell | Out-String | Invoke-Expression
```

To make it available in all powershell sessions that a user has, write the
completion output to a file and source that to the user's powershell profile.
More information about profiles is available with **Get-Help about_Profiles**.

## SEE ALSO
**[podman(1)](podman.1.md)**, **zsh(1)**, **fish(1)**, **powershell(1)**
