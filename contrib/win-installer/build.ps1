#!/usr/bin/env pwsh

<#
.SYNOPSIS
    Build Podman Windows MSI installer

.DESCRIPTION
    This script builds a Podman MSI installer for Windows using WiX Toolset.
    The release artifacts (podman.exe, gvproxy.exe, win-sshproxy.exe and
    docs) are either downloaded from GitHub or copied from a local directory,
    extracted, signed (if signing credentials are available), and packaged
    into an MSI installer.

.PARAMETER Version
    Podman version to build (e.g., "5.6.2" or "v5.6.2"). This will be the
    version of the MSI installer to be built.

.PARAMETER Architecture
    Target architecture for the installer. Valid values: 'amd64', 'arm64'.
    Default: 'amd64'

.PARAMETER LocalReleaseDirPath
    Optional path to a local directory containing the release zip file
    (podman-remote-release-windows_<arch>.zip). If not specified, the
    release will be downloaded from GitHub.

.PARAMETER RemoteReleaseRepoUrl
    URL of the Podman release repository to download from. Default is the
    official Podman GitHub repository. This parameter is primarily used by
    the Podman release scripts.

.EXAMPLE
    .\build.ps1 -Version 5.6.2
    Download and build amd64 MSI from the official GitHub release

.EXAMPLE
    .\build.ps1 -Version 5.6.2 -Architecture arm64
    Download and build arm64 MSI from the official GitHub release

.EXAMPLE
    .\build.ps1 -Version 5.6.2 -LocalReleaseDirPath .\current
    Build an MSI from a pre-downloaded release in the current directory

.EXAMPLE
    .\build.ps1 -Version 5.6.2 -Architecture arm64 `
        -LocalReleaseDirPath .\current
    Build an arm64 MSI from a local release directory

.NOTES
    Requirements:
      - .NET SDK (https://dotnet.microsoft.com/download)
      - WiX Toolset (install: dotnet tool install --global wix --version 5.0.2)

    Environment Variables for signing (optional):
      $ENV:VAULT_ID $ENV:APP_ID $ENV:TENANT_ID
      $ENV:CLIENT_SECRET $ENV:CERT_NAME
#>

param(
    [Parameter(
        Mandatory = $true,
        ParameterSetName = 'LocalReleaseDirPath',
        Position = 0,
        HelpMessage = 'Podman version to build (MSI installer version)'
    )]
    [ValidateNotNullOrEmpty()]
    [ValidatePattern('^v?([0-9]+\.[0-9]+\.[0-9]+)(-.*)?$')]
    [string]$Version,

    [Parameter(
        Mandatory = $false,
        Position = 1,
        HelpMessage = 'Target Architecture'
    )]
    [ValidateSet('amd64', 'arm64')]
    [string]$Architecture = 'amd64',

    [Parameter(
        Mandatory = $false,
        Position = 2,
        HelpMessage = 'Path to pre-downloaded release zip file'
    )]
    [ValidateScript({ Test-Path $_ -PathType Container })]
    [string]$LocalReleaseDirPath = '',

    [Parameter(
        Mandatory = $false,
        Position = 3,
        HelpMessage = 'URL of the Podman release repository'
    )]
    [string]$RemoteReleaseRepoUrl = 'https://github.com/containers/podman'
)

. $PSScriptRoot\utils.ps1

# Strip leading 'v' from version if present
$Version = $Version.TrimStart('v')

################################################################################
# REQUIREMENTS CHECK
################################################################################
Write-Host 'Checking requirements (dotnet and wix)'

# Check if .NET SDK is installed
if (! (Get-Command 'dotnet' -errorAction SilentlyContinue)) {
    Write-Error ("Required dep `".NET SDK`" is not installed. " `
        + 'Please install it from https://dotnet.microsoft.com/download')
    Exit 1
}

# Check if WiX Toolset is installed
Invoke-Expression 'dotnet tool list --global wix' `
    -ErrorAction SilentlyContinue | Out-Null
if ($LASTEXITCODE -ne 0) {
    Write-Error ("Required dep `"Wix Toolset`" is not installed. " `
        + 'Please install it with the command ' `
        + "`"dotnet tool install --global wix --version 5.0.2`"")
    Exit 1
}

################################################################################
# COPY OR DOWNLOAD RELEASE ARTIFACTS AND SIGN THEM
################################################################################
Write-Host 'Deleting old directories if they exist (''docs'', ''artifacts'' ' `
    'and ''fetch'')'
foreach ($dir in 'docs', 'artifacts', 'fetch') {
    Remove-Item -Force -Recurse -Path $PSScriptRoot\$dir `
        -ErrorAction SilentlyContinue | Out-Null
}

Write-Host 'Creating empty ''fetch'' and ''artifacts'' directories'
foreach ($dir in 'fetch', 'artifacts') {
    New-Item $PSScriptRoot\$dir -ItemType Directory | Out-Null
}

$ProgressPreference = 'SilentlyContinue';

if (!$LocalReleaseDirPath) {
    $releaseZipUrl = "$RemoteReleaseRepoUrl/releases/download/" `
        + "v$Version/podman-remote-release-windows_$Architecture.zip"
    Write-Host "Downloading $releaseZipUrl"
    DownloadReleaseFile -url $releaseZipUrl `
        -outputFile "$PSScriptRoot\fetch\release.zip" `
        -failOnError
    DownloadReleaseFile `
        -url "$RemoteReleaseRepoUrl/releases/download/v$Version/shasums" `
        -outputFile "$PSScriptRoot\shasums"
}
else {
    $sourceZip = "$LocalReleaseDirPath\" `
        + "podman-remote-release-windows_$Architecture.zip"
    Write-Host "Copying $sourceZip to $PSScriptRoot\fetch\release.zip"
    Copy-Item -Path $sourceZip `
        -Destination "$PSScriptRoot\fetch\release.zip" `
        -ErrorAction Stop
}

Write-Host "Expanding the podman release zip file to $PSScriptRoot\fetch"
Expand-Archive -Path $PSScriptRoot\fetch\release.zip `
    -DestinationPath $PSScriptRoot\fetch `
    -ErrorAction Stop
ExitOnError

Write-Host -NoNewline 'Copying artifacts: '
Foreach ($fileName in 'podman.exe', 'win-sshproxy.exe', 'gvproxy.exe') {
    Write-Host -NoNewline "$fileName, "
    Get-ChildItem -Path "$PSScriptRoot\fetch\" `
        -Filter "$fileName" `
        -Recurse | `
        Copy-Item -Container:$false `
        -Destination "$PSScriptRoot\artifacts\" `
        -ErrorAction Stop
    ExitOnError
}

Write-Host 'docs'
Get-ChildItem -Path $PSScriptRoot\fetch\ -Filter 'docs' -Recurse | `
    Copy-Item -Recurse `
    -Container:$false `
    -Destination "$PSScriptRoot\docs" `
    -ErrorAction Stop
ExitOnError

Write-Host 'Deleting the ''fetch'' folder'
Remove-Item -Force -Recurse -Path $PSScriptRoot\fetch `
    -ErrorAction SilentlyContinue | Out-Null

SignItem @("$PSScriptRoot\artifacts\win-sshproxy.exe",
    "$PSScriptRoot\artifacts\podman.exe",
    "$PSScriptRoot\artifacts\gvproxy.exe")
ExitOnError

################################################################################
# BUILD THE MSI
################################################################################
dotnet clean $PSScriptRoot\wix\podman.wixproj
Remove-Item $PSScriptRoot\wix\obj -Recurse -Force -Confirm:$false `
    -ErrorAction 'Ignore'

$archMap = @{
    'amd64' = 'x64'
    'arm64' = 'arm64'
}
$installerPlatform = $archMap[$Architecture]

Write-Host 'Building the MSI...'
dotnet build $PSScriptRoot\wix\podman.wixproj `
    /property:DefineConstants="VERSION=$Version" `
    /property:InstallerPlatform="$installerPlatform" `
    /property:OutputName="podman-$Version" `
    -o $PSScriptRoot
ExitOnError

$msiName = "podman-$Version.msi"

SignItem @("$PSScriptRoot\$msiName")

if (Test-Path -Path $PSScriptRoot\shasums) {
    $hash = (Get-FileHash -Algorithm SHA256 `
            $PSScriptRoot\$msiName).Hash.ToLower()
    Write-Output "$hash  $msiName" | `
        Out-File -Append -FilePath $PSScriptRoot\shasums
}

Write-Host 'Complete'
Get-ChildItem "$PSScriptRoot\$msiName"
