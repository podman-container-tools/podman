#!/usr/bin/env powershell

# This powershell script is intended to be "dot-sourced" by other scripts.
# It's purpose is identical to that of the `lib.sh` script for Linux environments.

# Behave similar to `set -e` in bash, but ONLY for powershell commandlets!
# For all legacy, program, and script calls use Run-Command() or Check-Exit()
$ErrorActionPreference = 'Stop'

# Any golang compilation needs to know what it's building for.
$Env:GOOS = "windows"

# Unnecessary and intrusive.  They claim parameter/variable
# values aren't collected, but there could be a bug leading
# to a concern over leaking of some sensitive-value.  Stop this.
$Env:POWERSHELL_TELEMETRY_OPTOUT = "true"

# Unnecessary and potentially disruptive.  Powershell will
# never ever be updated during automation execution.  Stop this.
$Env:POWERSHELL_UPDATECHECK = "off"

# Color in output may confuse tooling and makes logs hard to read.
# TODO: There are probably other places where color needs to be disabled
# in a slightly different way :(
$Env:NO_COLOR = "true"

# Items only relevant in a CI environment.
if ($Env:CI -eq "true") {
    # Defined by .cirrus.yml for use by all the linux tasks.
    # Drop all global envs which have unix paths, defaults are fine.
    Remove-Item Env:\GOPATH -ErrorAction:Ignore
    Remove-Item Env:\GOSRC -ErrorAction:Ignore
    Remove-Item Env:\GOCACHE -ErrorAction:Ignore

    # Defined by Cirrus-CI
    # Drop large known env variables (an env > 32k will break MSI/ICE validation)
    Remove-Item Env:\CIRRUS_COMMIT_MESSAGE -ErrorAction:Ignore
    Remove-Item Env:\CIRRUS_CHANGE_MESSAGE -ErrorAction:Ignore
    Remove-Item Env:\CIRRUS_PR_BODY -ErrorAction:Ignore
}

function Invoke-Logformatter {
    param (
        [Collections.ArrayList] $unformattedLog
    )

    Write-Host "Invoking Logformatter"
    $logFormatterInput = @('/define.gitCommit=' + $(git rev-parse HEAD)) + $unformattedLog
    $logformatterPath = "$PSScriptRoot\logformatter"
    if ($Env:TEST_FLAVOR) {
        $logformatterArg = "$Env:TEST_FLAVOR-podman-windows-rootless-host-sqlite"
    } else {
        $logformatterArg = "podman-windows-rootless-host-sqlite"
    }
    $null =  $logFormatterInput | perl $logformatterPath $logformatterArg
    $logformatterGeneratedFile = "$logformatterArg.log.html"
    if (Test-Path $logformatterGeneratedFile) {
        Move-Item $logformatterGeneratedFile .. -Force
    } else {
        Write-Host "Logformatter did not generate the expected file: $logformatterGeneratedFile"
    }
}

# Non-powershell commands do not halt execution on error!  This helper
# should be called after every critical operation to check and halt on a
# non-zero exit code.  Be careful not to use this for powershell commandlets
# (builtins)!  They set '$?' to "True" (failed) or "False" success so calling
# this would mask failures.  Rely on $ErrorActionPreference = 'Stop' instead.
function Check-Exit {
    param (
        [int] $stackPos = 1,
        [string] $command = 'command',
        [string] $exitCode = $LASTEXITCODE # WARNING: might not be a number!
    )

    if ( ($exitCode -ne $null) -and ($exitCode -ne 0) ) {
        # https://learn.microsoft.com/en-us/dotnet/api/system.management.automation.callstackframe
        $caller = (Get-PSCallStack)[$stackPos]
        throw "Exit code = '$exitCode' running $command at $($caller.ScriptName):$($caller.ScriptLineNumber)"
    }
}

# Small helper to avoid needing to write 'Check-Exit' after every
# non-powershell instruction.  It simply prints then executes the _QUOTED_
# argument followed by Check-Exit.
# N/B: Escape any nested quotes with back-tick ("`") characters.
# WARNING: DO NOT use this with powershell builtins! It will not do what you expect!
function Run-Command {
    param (
        [string] $command
    )

    Write-Host $command

    # The command output is saved into the variable $unformattedLog to be
    # processed by `logformatter` later. The alternative is to redirect the
    # command output to logformatter using a pipeline (`|`). But this approach
    # doesn't work as the command exit code would be overridden by logformatter.
    # It isn't possible to get a behavior of bash `pipefail` on Windows.
    Invoke-Expression $command -OutVariable unformattedLog | Write-Output

    $exitCode = $LASTEXITCODE

    if ($Env:CIRRUS_CI -eq "true") {
        Invoke-Logformatter $unformattedLog
    }

    Check-Exit 2 "'$command'" "$exitCode"
}
