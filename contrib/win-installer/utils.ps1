#!/usr/bin/env pwsh
function Get-Latest-Podman-Setup-From-GitHub {
    param(
        [ValidateSet("amd64", "arm64")]
        [string] $arch = "amd64"
    )
    # Starting from v6.0 we stopped publishing the setup
    return Get-Podman-Setup-From-GitHub "v5.8.3" $arch
}

function Get-Podman-Setup-From-GitHub {
    param(
        [Parameter(Mandatory)]
        [string] $version,
        [ValidateSet("amd64", "arm64")]
        [string] $arch = "amd64"
    )

    Write-Host "Downloading the $arch $version Podman windows setup from GitHub..."
    $apiUrl = "https://api.github.com/repos/podman-container-tools/podman/releases/tags/$version"
    $headers = @{"User-Agent"="PowerShell"}
    if ($env:GITHUB_TOKEN) {
        $headers["Authorization"] = "Bearer $env:GITHUB_TOKEN"
    }
    $response = Invoke-RestMethod -Uri $apiUrl -Headers $headers -ErrorAction Stop
    $latestTag = $response.tag_name
    Write-Host "Looking for an asset named ""podman-installer-windows-$arch.exe"""
    $downloadAsset = $response.assets | Where-Object { $_.name -eq "podman-installer-windows-$arch.exe" } | Select-Object -First 1
    if (-not $downloadAsset) {
        # remove the first char from $latestTag if it is a "v"
        if ($latestTag[0] -eq "v") {
            $newLatestTag = $latestTag.Substring(1)
        }
        Write-Host "Not found. Looking for an asset named ""podman-$newLatestTag-setup.exe"""
        $downloadAsset = $response.assets | Where-Object { $_.name -eq "podman-$newLatestTag-setup.exe" } | Select-Object -First 1
    }
    $downloadUrl = $downloadAsset.browser_download_url
    Write-Host "Downloading URL: $downloadUrl"
    $destinationPath = "$PSScriptRoot\podman-${latestTag}-setup.exe"
    Write-Host "Destination Path: $destinationPath"
    Invoke-WebRequest -Uri $downloadUrl -OutFile $destinationPath
    Write-Host "Command completed successfully!`n"
    return $destinationPath
}

function DownloadReleaseFile {
    param(
        [Parameter(Mandatory)]
        [string]$url,
        [Parameter(Mandatory)]
        [string]$outputFile,
        [Parameter(Mandatory=$false, Position=3, HelpMessage="Fail if `Invoke-WebRequest` fails, silently continue if not specified")]
        [switch]$failOnError = $false
    )
    $ProgressPreference = 'SilentlyContinue';
    try {
        Invoke-WebRequest -UseBasicParsing -ErrorAction Stop -Uri $url -OutFile $outputFile
    } Catch {
        if ($FailOnError) {
            if ($_.Exception.Response.StatusCode -eq 404) {
                Write-Error "URL not available $url"
                Exit 2
            }

            throw $_.Exception
        }
    }
}

function ExitOnError() {
    if ($LASTEXITCODE -ne 0) {
        Exit 1
    }
}

function SignItem() {
    param(
        [Parameter(Mandatory)]
        [string[]]$fileNames
    )

    foreach ($val in $ENV:APP_ID, $ENV:TENANT_ID, $ENV:CLIENT_SECRET, $ENV:CERT_NAME) {
        if (!$val) {
            Write-Host 'Skipping signing (no config)'
            Return
        }
    }

    # Check if AzureSignTool is installed
    if (! (Get-Command 'AzureSignTool.exe' -errorAction SilentlyContinue)) {
        Write-Error "Required dep `"AzureSignTool`" is not installed. "
        Exit 1
    }

    AzureSignTool.exe sign -du 'https://github.com/containers/podman' `
        -kvu "https://$ENV:VAULT_ID.vault.azure.net" `
        -kvi $ENV:APP_ID `
        -kvt $ENV:TENANT_ID `
        -kvs $ENV:CLIENT_SECRET `
        -kvc $ENV:CERT_NAME `
        -tr http://timestamp.digicert.com $fileNames

    ExitOnError
}

function Get-Current-Architecture {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
    if ($arch -eq 'X64') {
        return 'amd64'
    } elseif ($arch -eq 'Arm64') {
        return 'arm64'
    } else {
        throw "Unsupported architecture: $arch"
    }
}

# Pre-set to standard locations in-case build env does not refresh paths
$Env:Path = "$Env:Path;" + `
    'C:\Users\micro\mingw64\bin;' + `
    'C:\ProgramData\chocolatey\lib\mingw\tools\install\mingw64\bin;' + `
    ';C:\Program Files\Go\bin;' + `
    'C:\Program Files\dotnet'
