#!/usr/bin/env pwsh

<#
.SYNOPSIS
    Automated test script for the Podman Windows MSI installer.

.DESCRIPTION
    This script provides automated end-to-end tests for the Podman Windows MSI
    installer. It supports testing installation, update, and configuration
    scenarios for both user and machine scopes, as well as for different
    virtualization providers (WSL and Hyper-V).
    The script validates the presence and correctness of installed files,
    registry keys, and configuration files.

.PARAMETER scenario
    The test scenario to execute. Supported values:
        - test-objects-exist
        - test-objects-exist-not
        - installation-green-field
        - installation-skip-config-creation-flag
        - installation-with-pre-existing-podman-exe
        - update-from-current-to-next
        - update-from-current-to-next-with-modified-config
        - update-from-current-to-next-with-removed-config
        - update-from-legacy-to-current
        - all

.PARAMETER msiPath
    Path to the current Podman MSI installer to test.

.PARAMETER nextMsiPath
    Path to the next version Podman MSI installer (for update scenarios).

.PARAMETER previousSetupExePath
    Path to the legacy Podman setup EXE installer (for legacy update scenarios).

.PARAMETER provider
    The virtualization provider to test with. Supported values: 'wsl', 'hyperv'. Default: 'wsl'.

.PARAMETER skipConfigFileCreation
    Switch to skip creation of the configuration file during installation.

.PARAMETER scope
    Installation scope. Supported values: 'user', 'machine'. Default: 'user'.

.EXAMPLE
    .\test.ps1 -scenario all -msiPath "C:\path\to\podman.msi" -nextMsiPath "C:\path\to\next\podman.msi" -scope user

.NOTES
    - When using '-scope machine', this script must be run as Administrator.
    - This script is intended for use by Podman developers and CI pipelines to validate installer behavior.
    - See build_windows.md for more information and usage examples.
#>

param (
    [Parameter(Mandatory)]
    [ValidateSet('test-objects-exist', 'test-objects-exist-not', 'installation-green-field', 'installation-skip-config-creation-flag', 'installation-with-pre-existing-podman-exe',
        #"update-from-prev-to-current", "update-from-prev-to-current-with-modified-config", "update-from-prev-to-current-with-removed-config",
        'update-from-current-to-next', 'update-from-current-to-next-with-modified-config', 'update-from-current-to-next-with-removed-config',
        'update-from-legacy-to-current',
        'all')]
    [string]$scenario,
    [ValidateScript({ Test-Path $_ -PathType Leaf })]
    [string]$msiPath,
    # [ValidateScript({Test-Path $_ -PathType Leaf})]
    # [string]$previousMsiPath,
    [ValidateScript({ Test-Path $_ -PathType Leaf })]
    [string]$nextMsiPath,
    [ValidateScript({ Test-Path $_ -PathType Leaf })]
    [string]$previousSetupExePath,
    [ValidateSet('wsl', 'hyperv')]
    [string]$provider = 'wsl',
    [switch]$skipConfigFileCreation = $false,
    [ValidateSet('machine', 'user')]
    [string]$scope = 'user'
)

. $PSScriptRoot\utils.ps1

# Check if running as administrator when testing machine scope installation
if ($scope -eq 'machine') {
    $currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    $isAdmin = $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    if (-not $isAdmin) {
        throw 'The -scope machine parameter requires the script to be run as Administrator. Please run PowerShell as Administrator and try again.'
    }
}

# Get the architecture of the current OS
$osArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
$arch = switch ($osArch) {
    'X64' { 'amd64' }
    'Arm64' { 'arm64' }
    default { throw "Unsupported architecture: $osArch" }
}

# 99-podman-machine-provider.conf file path
$MachineConfPathPerMachine = "$env:ProgramData\containers\containers.conf.d\99-podman-machine-provider.conf"
$MachineConfPathPerMachineLegacy = $MachineConfPathPerMachine
$MachineConfPathPerUser = "$env:AppData\containers\containers.conf.d\99-podman-machine-provider.conf"
# Podman application folder path
$PodmanFolderPathPerMachine = "$env:ProgramFiles\Podman"
$PodmanFolderPathPerMachineLegacy = "$env:ProgramFiles\RedHat\Podman"
$PodmanFolderPathPerUser = "$env:LocalAppData\Programs\Podman"
# Podman.executable file path
$PodmanExePathPerMachine = "$PodmanFolderPathPerMachine\podman.exe"
$PodmanExePathPerMachineLegacy = "$PodmanFolderPathPerMachineLegacy\podman.exe"
$PodmanExePathPerUser = "$PodmanFolderPathPerUser\podman.exe"

$WindowsPathsToTestPerMachine = @($PodmanExePathPerMachine,
    "$PodmanFolderPathPerMachine\win-sshproxy.exe",
    'HKLM:\SOFTWARE\Podman')
$WindowsPathsToTestLegacy = @($PodmanExePathPerMachineLegacy,
    "$PodmanFolderPathPerMachineLegacy\win-sshproxy.exe",
    'HKLM:\SOFTWARE\Red Hat\Podman')
$WindowsPathsToTestPerUser = @($PodmanExePathPerUser,
    "$PodmanFolderPathPerUser\win-sshproxy.exe",
    'HKCU:\SOFTWARE\Podman')

# Install-Podman-Bundle installs the legacy Podman installer that is no longer
# released but we want to keep it to test the update from a version of Podman
# installed with it.
function Install-Podman-Bundle {
    param (
        [ValidateScript({ Test-Path $_ -PathType Leaf })]
        [string]$setupExePath = $script:previousSetupExePath
    )
    if ($script:skipConfigFileCreation) { $skipConfigFileCreationVar = '1' } else { $skipConfigFileCreationVar = '0' }

    Write-Host "Running the installer ($setupExePath)..."
    Write-Host "(provider=`"$script:provider`", SkipConfigFileCreation=`"$skipConfigFileCreationVar`")"
    $ret = Start-Process -Wait `
        -PassThru "$setupExePath" `
        -ArgumentList "/install /quiet `
                                        MachineProvider=${script:provider} `
                                        SkipConfigFileCreation=${skipConfigFileCreationVar} `
                                        /log $PSScriptRoot\podman-setup.log"
    if ($ret.ExitCode -ne 0) {
        Write-Host 'Install failed, dumping log'
        Get-Content $PSScriptRoot\podman-setup.log
        throw "Exit code is $($ret.ExitCode)"
    }
    Write-Host "Installation completed successfully!`n"
}

function Install-Podman-Package {
    param (
        [Parameter(Mandatory)]
        [ValidateScript({ Test-Path $_ -PathType Leaf })]
        [string]$msiPath,
        [string]$msiProperties,
        [switch]$shouldFail = $false
    )

    Write-Host "Running the installer ($msiPath)..."
    if ($msiProperties) {
        Write-Host "MSI Properties: $msiProperties"
    }
    else {
        Write-Host 'MSI Properties: none'
    }
    $msiExecArgs = "/package $msiPath /quiet /l*v $PSScriptRoot\podman-msi.log $msiProperties"
    Write-Host "msiexec $msiExecArgs"
    $ret = Start-Process -Wait `
        -PassThru 'msiexec' `
        -ArgumentList $msiExecArgs
    if ($ret.ExitCode -ne 0 -and -not $shouldFail) {
        Write-Host 'Install failed, dumping log'
        Get-Content $PSScriptRoot\podman-msi.log
        throw "Exit code is $($ret.ExitCode)"
    }
    if ($ret.ExitCode -eq 0 -and $shouldFail) {
        Write-Host 'Install completed successfully but a failure was expected, dumping log'
        Get-Content $PSScriptRoot\podman-msi.log
        throw "Exit code is $($ret.ExitCode) but a failure was expected"
    }
    if ($shouldFail) {
        Write-Host "Installation failed as expected!`n"
    }
    else {
        Write-Host "Installation completed successfully!`n"
    }
}

function Install-Podman-Package-with-Explicit-Properties {
    param (
        [ValidateScript({ Test-Path $_ -PathType Leaf })]
        [string]$msiPath = $script:msiPath,
        [ValidateSet('machine', 'user')]
        [string]$scope = $script:scope,
        [switch]$skipConfigFileCreation = $script:skipConfigFileCreation
    )

    # MACHINE_PROVIDER
    $MACHINE_PROVIDER_PROP = "MACHINE_PROVIDER=$script:provider"
    # SKIP_CONFIG_FILE_CREATION
    if ($skipConfigFileCreation) {
        $SKIP_CONFIG_FILE_CREATION_PROP = 'SKIP_CONFIG_FILE_CREATION=1'
    }
    else {
        $SKIP_CONFIG_FILE_CREATION_PROP = 'SKIP_CONFIG_FILE_CREATION=0'
    }
    # ALLUSERS or MSIINSTALLPERUSER
    if ($scope -eq 'machine') {
        $ALLUSERS_OR_MSIINSTALLPERUSER_PROP = 'ALLUSERS=1'
    }
    else {
        $ALLUSERS_OR_MSIINSTALLPERUSER_PROP = 'MSIINSTALLPERUSER=1'
    }

    Install-Podman-Package -msiPath $msiPath -msiProperties "${MACHINE_PROVIDER_PROP} ${ALLUSERS_OR_MSIINSTALLPERUSER_PROP} ${SKIP_CONFIG_FILE_CREATION_PROP}"
}

function Update-Podman-Package {
    param (
        [ValidateSet('From-Previous', 'To-Next', 'From-Previous-Legacy', 'To-Next-Scope-Switch')]
        [string]$mode = 'From-Previous',
        [ValidateSet('none', 'switch-provider', 'delete-config-file')]
        [string]$configurationUpdate = 'none'
    )

    # Step 1: Install the initial version
    switch ($mode) {
        # There is no "previous package" yet so this scenario cannot be tested.
        # This block, and those in STEP 3 and 5, should be commented out after the
        # release of dual scope MSI.
        # 'From-Previous' {
        #     Install-Podman-Package-with-Explicit-Properties -msiPath $msiPreviousPath
        #     Test-Installation -scope $script:scope
        # }
        'To-Next' {
            Install-Podman-Package-with-Explicit-Properties
            Test-Installation
        }
        'From-Previous-Legacy' {
            Install-Podman-Bundle
            Test-Installation -scope 'machine-legacy'
        }
        'To-Next-Scope-Switch' {
            if ($script:scope -eq 'machine') {
                $newScope = 'user'
            }
            else {
                $newScope = 'machine'
            }
            Install-Podman-Package-with-Explicit-Properties -scope $newScope
            Test-Installation -scope $newScope
        }
    }

    # Step 2: Make some changes to the configration file if requested
    $newProvider = $script:provider
    $configFileRemoved = $false
    switch ($configurationUpdate) {
        'switch-provider' {
            $newProvider = Switch-Podman-Machine-Conf-Content -scope $script:scope
        }
        'delete-config-file' {
            Remove-Podman-Machine-Conf -scope $script:scope
            $configFileRemoved = $true
        }
    }

    # Step 3: Install the next version
    $msiProperties = ''
    if ($script:scope -eq 'machine') {
        $msiProperties = 'ALLUSERS=1'
    }
    switch ($mode) {
        # 'From-Previous' {
        #     Install-Podman-Package -msiPath $script:msiPath
        #     Test-Installation -scope $script:scope -expectedConfiguredProvider $newProvider -configFileExistNot:$configFileRemoved
        # }
        'To-Next' {
            Install-Podman-Package -msiPath $script:nextMsiPath -msiProperties $msiProperties
            Test-Installation -scope $script:scope -expectedConfiguredProvider $newProvider -configFileExistNot:$configFileRemoved
        }
        'From-Previous-Legacy' {
            Install-Podman-Package -shouldFail -msiPath $script:msiPath -msiProperties $msiProperties
        }
        'To-Next-Scope-Switch' {
            Install-Podman-Package -shouldFail -msiPath $script:nextMsiPath -msiProperties $msiProperties
        }
    }

    # Step 4: Check that the changes to the configuration file are persisted
    switch ($configurationUpdate) {
        'switch-provider' {
            Test-Podman-Machine-Conf-Content -expected $newProvider -scope $script:scope
        }
        'delete-config-file' {
            Test-Podman-Machine-Conf-Exist-Not -scope $script:scope -configFileRemoved:$configFileRemoved
        }
    }

    # Step 5: Uninstall
    switch ($mode) {
        # 'From-Previous' {
        #     Uninstall-Podman-Package -msiPath $msiPreviousPath -scope $script:scope
        #     Test-Uninstallation -scope $script:scope
        # }
        'To-Next' {
            Uninstall-Podman-Package -msiPath $script:nextMsiPath
            Test-Uninstallation -scope $script:scope
        }
        'From-Previous-Legacy' {
            Uninstall-Podman-Bundle -setupExePath $script:previousSetupExePath
            Test-Uninstallation -scope 'machine-legacy'
        }
        'To-Next-Scope-Switch' {
            if ($script:scope -eq 'machine') {
                $newScope = 'user'
            }
            else {
                $newScope = 'machine'
            }
            Uninstall-Podman-Package -msiPath $script:msiPath
            Test-Uninstallation -scope $newScope
        }
    }
}

function Test-Podman-Objects-Exist {
    param (
        [ValidateSet('machine-legacy', 'machine', 'user')]
        [string]$scope = $script:scope
    )
    Write-Host "Verifying that podman files, folders and registry entries exist...(scope=$scope)"
    if ($scope -eq 'machine') {
        $WindowsPathsToTest = $WindowsPathsToTestPerMachine
    }
    elseif ($scope -eq 'machine-legacy') {
        $WindowsPathsToTest = $WindowsPathsToTestLegacy
    }
    else {
        $WindowsPathsToTest = $WindowsPathsToTestPerUser
    }
    $WindowsPathsToTest | ForEach-Object {
        if (! (Test-Path -Path $_) ) {
            throw "Expected $_ but doesn't exist"
        }
    }
    Write-Host "Verification was successful!`n"
}

function Test-Podman-Machine-Conf-Exist {
    param (
        [ValidateSet('machine-legacy', 'machine', 'user')]
        [string]$scope = $script:scope
    )
    if ($scope -eq 'machine') {
        $MachineConfPath = $MachineConfPathPerMachine
    }
    elseif ($scope -eq 'machine-legacy') {
        $MachineConfPath = $MachineConfPathPerMachineLegacy
    }
    else {
        $MachineConfPath = $MachineConfPathPerUser
    }
    Write-Host "Verifying that $MachineConfPath exist...(scope=$scope)"
    if (! (Test-Path -Path $MachineConfPath) ) {
        throw "Expected $MachineConfPath but doesn't exist"
    }
    Write-Host "Verification was successful!`n"
}

function Test-Podman-Machine-Conf-Content {
    [CmdletBinding(PositionalBinding = $false)]
    param (
        [ValidateSet('wsl', 'hyperv')]
        [string]$expected = $script:provider,
        [ValidateSet('machine-legacy', 'machine', 'user')]
        [string]$scope = $script:scope
    )
    Write-Host "Verifying that the machine provider configuration is correct...(scope=$scope)"
    if ($scope -eq 'machine') {
        $MachineConfPath = $MachineConfPathPerMachine
    }
    elseif ($scope -eq 'machine-legacy') {
        $MachineConfPath = $MachineConfPathPerMachineLegacy
    }
    else {
        $MachineConfPath = $MachineConfPathPerUser
    }
    $machineProvider = Get-Content $MachineConfPath | Select-Object -Skip 1 | ConvertFrom-StringData | ForEach-Object { $_.provider }
    if ( $machineProvider -ne "`"$expected`"" ) {
        throw "Expected `"$expected`" as default machine provider but got $machineProvider"
    }
    Write-Host "Verification was successful!`n"
}

function Uninstall-Podman-Bundle {
    param (
        # [Parameter(Mandatory)]
        [ValidateScript({ Test-Path $_ -PathType Leaf })]
        [string]$setupExePath
    )
    Write-Host "Running the uninstaller ($setupExePath)..."
    $ret = Start-Process -Wait `
        -PassThru "$setupExePath" `
        -ArgumentList "/uninstall `
                         /quiet /log $PSScriptRoot\podman-setup-uninstall.log"
    if ($ret.ExitCode -ne 0) {
        Write-Host 'Uninstall failed, dumping log'
        Get-Content $PSScriptRoot\podman-setup-uninstall.log
        throw "Exit code is $($ret.ExitCode)"
    }
    Write-Host "The uninstallation completed successfully!`n"
}

function Uninstall-Podman-Package {
    param (
        [Parameter(Mandatory)]
        [ValidateScript({ Test-Path $_ -PathType Leaf })]
        [string]$msiPath
    )
    Write-Host "Running the uninstaller ($msiPath)..."
    $ret = Start-Process -Wait `
        -PassThru 'msiexec' `
        -ArgumentList "/uninstall $msiPath /quiet /l*v $PSScriptRoot\podman-msi-uninstall.log"
    if ($ret.ExitCode -ne 0) {
        Write-Host 'Uninstall failed, dumping log'
        Get-Content $PSScriptRoot\podman-msi-uninstall.log
        throw "Exit code is $($ret.ExitCode)"
    }
    Write-Host "The uninstallation completed successfully!`n"
}

function Test-Podman-Objects-Exist-Not {
    param (
        [ValidateSet('machine-legacy', 'machine', 'user')]
        [string]$scope = $script:scope
    )
    Write-Host "Verifying that podman files, folders and registry entries don't exist...(scope=$scope)"
    if ($scope -eq 'machine') {
        $WindowsPathsToTest = $WindowsPathsToTestPerMachine
    }
    elseif ($scope -eq 'machine-legacy') {
        $WindowsPathsToTest = $WindowsPathsToTestLegacy
    }
    else {
        $WindowsPathsToTest = $WindowsPathsToTestPerUser
    }
    $WindowsPathsToTest | ForEach-Object {
        if ( Test-Path -Path $_ ) {
            throw "Path $_ is present"
        }
    }
    Write-Host "Verification was successful!`n"
}

function Test-Podman-Machine-Conf-Exist-Not {
    param (
        [ValidateSet('machine-legacy', 'machine', 'user')]
        [string]$scope = $script:scope
    )
    if ($scope -eq 'machine') {
        $MachineConfPath = $MachineConfPathPerMachine
    }
    elseif ($scope -eq 'machine-legacy') {
        $MachineConfPath = $MachineConfPathPerMachineLegacy
    }
    else {
        $MachineConfPath = $MachineConfPathPerUser
    }
    Write-Host "Verifying that $MachineConfPath doesn't exist...(scope=$scope)"
    if ( Test-Path -Path $MachineConfPath ) {
        throw "Path $MachineConfPath is present"
    }
    Write-Host "Verification was successful!`n"
}

function New-Fake-Podman-Exe {
    param (
        [ValidateSet('machine-legacy', 'machine', 'user')]
        [string]$scope = $script:scope
    )
    if ($scope -eq 'machine') {
        $PodmanFolderPath = $PodmanFolderPathPerMachine
        $PodmanExePath = $PodmanExePathPerMachine
    }
    elseif ($scope -eq 'machine-legacy') {
        $PodmanFolderPath = $PodmanFolderPathPerMachineLegacy
        $PodmanExePath = $PodmanExePathPerMachineLegacy
    }
    else {
        $PodmanFolderPath = $PodmanFolderPathPerUser
        $PodmanExePath = $PodmanExePathPerUser
    }
    Write-Host "Creating a fake $PodmanExePath...(scope=$scope)"
    New-Item -ItemType Directory -Path $PodmanFolderPath -Force -ErrorAction Stop | out-null
    New-Item -ItemType File -Path $PodmanExePath -ErrorAction Stop | out-null
    Write-Host "Creation successful!`n"
}

function Switch-Podman-Machine-Conf-Content {
    param (
        [ValidateSet('machine-legacy', 'machine', 'user')]
        [string]$scope = $script:scope
    )
    $currentProvider = $script:provider
    if ( $currentProvider -eq 'wsl' ) { $newProvider = 'hyperv' } else { $newProvider = 'wsl' }
    if ($scope -eq 'machine') {
        $MachineConfPath = $MachineConfPathPerMachine
    }
    elseif ($scope -eq 'machine-legacy') {
        $MachineConfPath = $MachineConfPathPerMachineLegacy
    }
    else {
        $MachineConfPath = $MachineConfPathPerUser
    }
    Write-Host "Editing $MachineConfPath content (was $currentProvider, will be $newProvider)..."
    "[machine]`nprovider=`"$newProvider`"" | Out-File -FilePath $MachineConfPath -ErrorAction Stop
    Write-Host "Edit successful!`n"
    return $newProvider
}

function Remove-Podman-Machine-Conf {
    param (
        [ValidateSet('machine-legacy', 'machine', 'user')]
        [string]$scope = $script:scope
    )
    if ($scope -eq 'machine') {
        $MachineConfPath = $MachineConfPathPerMachine
    }
    elseif ($scope -eq 'machine-legacy') {
        $MachineConfPath = $MachineConfPathPerMachineLegacy
    }
    else {
        $MachineConfPath = $MachineConfPathPerUser
    }
    Write-Host "Deleting $MachineConfPath..."
    Remove-Item -Path $MachineConfPath -ErrorAction Stop | out-null
    Write-Host "Deletion successful!`n"
}

function Test-Installation {
    [CmdletBinding(PositionalBinding = $false)]
    param (
        [ValidateSet('wsl', 'hyperv')]
        [string]$expectedConfiguredProvider,
        [ValidateSet('machine-legacy', 'machine', 'user')]
        [string]$scope = $script:scope,
        [switch]$configFileExistNot = $false
    )

    Test-Podman-Objects-Exist -scope $scope

    if ($configFileExistNot) {
        Test-Podman-Machine-Conf-Exist-Not -scope $scope
    }
    else {
        Test-Podman-Machine-Conf-Exist -scope $scope
        if ($expectedConfiguredProvider) {
            Test-Podman-Machine-Conf-Content -expected $expectedConfiguredProvider -scope $scope
        }
        else {
            Test-Podman-Machine-Conf-Content -scope $scope
        }
    }
}

function Test-Installation-No-Config {
    param (
        [ValidateSet('machine-legacy', 'machine', 'user')]
        [string]$scope = $script:scope
    )
    Test-Podman-Objects-Exist -scope $scope
    Test-Podman-Machine-Conf-Exist-Not -scope $scope
}

function Test-Uninstallation {
    param (
        [ValidateSet('machine-legacy', 'machine', 'user')]
        [string]$scope = $script:scope
    )
    Test-Podman-Objects-Exist-Not -scope $scope
    Test-Podman-Machine-Conf-Exist-Not -scope $scope
}

# SCENARIOS
function Start-Scenario-Installation-Green-Field {
    Write-Host "`n==========================================="
    Write-Host ' Running scenario: Installation-Green-Field'
    Write-Host '==========================================='
    Install-Podman-Package-with-Explicit-Properties -msiPath $script:msiPath -scope $script:scope
    Test-Installation
    Uninstall-Podman-Package -msiPath $script:msiPath
    Test-Uninstallation
}

function Start-Scenario-Installation-Skip-Config-Creation-Flag {
    Write-Host "`n========================================================="
    Write-Host ' Running scenario: Installation-Skip-Config-Creation-Flag'
    Write-Host '========================================================='
    Install-Podman-Package-with-Explicit-Properties -msiPath $script:msiPath -scope $script:scope -skipConfigFileCreation:$true
    Test-Installation-No-Config
    Uninstall-Podman-Package -msiPath $script:msiPath
    Test-Uninstallation
}

function Start-Scenario-Installation-With-Pre-Existing-Podman-Exe {
    Write-Host "`n============================================================"
    Write-Host ' Running scenario: Installation-With-Pre-Existing-Podman-Exe'
    Write-Host '============================================================'
    New-Fake-Podman-Exe
    Install-Podman-Package-with-Explicit-Properties -msiPath $script:msiPath -scope $script:scope
    Test-Installation-No-Config
    Uninstall-Podman-Package -msiPath $script:msiPath
    Test-Uninstallation

    # remove the Podman folder created by `New-Fake-Podman-Exe` that
    # otherwise would remain after the uninstallation
    if ($script:scope -eq 'machine') {
        Remove-Item -Path $PodmanFolderPathPerMachine -Recurse -Force
    }
    elseif ($script:scope -eq 'machine-legacy') {
        Remove-Item -Path $PodmanFolderPathPerMachineLegacy -Recurse -Force
    }
    else {
        Remove-Item -Path $PodmanFolderPathPerUser -Recurse -Force
    }
}

# function Start-Scenario-Update-From-Prev-To-Current {
#     Write-Host "`n======================================================"
#     Write-Host " Running scenario: Update-From-Prev-To-Current"
#     Write-Host "======================================================"
#     Update-Podman-Package -mode "From-Previous" -configurationUpdate "none"
# }

# function Start-Scenario-Update-From-Prev-To-Current-With-Modified-Config {
#     Write-Host "`n=============================================================="
#     Write-Host " Running scenario: Update-From-Prev-To-Current-With-Modified-Config"
#     Write-Host "=============================================================="
#     Update-Podman-Package -mode "From-Previous" -configurationUpdate "switch-provider"
# }

# function Start-Scenario-Update-From-Prev-To-Current-With-Removed-Config {
#     Write-Host "`n=============================================================="
#     Write-Host " Running scenario: Update-From-Prev-To-Current-With-Removed-Config"
#     Write-Host "=============================================================="
#     Update-Podman-Package -mode "From-Previous" -configurationUpdate "delete-config-file"
# }

function Start-Scenario-Update-From-Current-To-Next {
    Write-Host "`n======================================================"
    Write-Host ' Running scenario: Update-From-Current-To-Next'
    Write-Host '======================================================'
    Update-Podman-Package -mode 'To-Next' -configurationUpdate 'none'
}

function Start-Scenario-Update-From-Current-To-Next-With-Modified-Config {
    Write-Host "`n=============================================================="
    Write-Host ' Running scenario: Update-From-Current-To-Next-With-Modified-Config'
    Write-Host '=============================================================='
    Update-Podman-Package -mode 'To-Next' -configurationUpdate 'switch-provider'
}

function Start-Scenario-Update-From-Current-To-Next-With-Removed-Config {
    Write-Host "`n=============================================================="
    Write-Host ' Running scenario: Update-From-Current-To-Next-With-Removed-Config'
    Write-Host '=============================================================='
    Update-Podman-Package -mode 'To-Next' -configurationUpdate 'delete-config-file'
}

function Start-Scenario-Update-From-Legacy-To-Current {
    Write-Host "`n======================================================"
    Write-Host ' Running scenario: Update-From-Legacy-To-Current'
    Write-Host '======================================================'
    Update-Podman-Package -mode 'From-Previous-Legacy' -configurationUpdate 'none'
}

switch ($script:scenario) {
    'test-objects-exist' {
        Test-Podman-Objects-Exist
    }
    'test-objects-exist-not' {
        Test-Podman-Objects-Exist-Not
    }
    'installation-green-field' {
        if (!$script:msiPath) {
            throw "Current installer path is not defined. Use '-msiPath <msi-path>' to define it."
        }
        Start-Scenario-Installation-Green-Field
    }
    'installation-skip-config-creation-flag' {
        if (!$script:msiPath) {
            throw "Current installer path is not defined. Use '-msiPath <msi-path>' to define it."
        }
        Start-Scenario-Installation-Skip-Config-Creation-Flag
    }
    'installation-with-pre-existing-podman-exe' {
        if (!$script:msiPath) {
            throw "Current installer path is not defined. Use '-msiPath <msi-path>' to define it."
        }
        Start-Scenario-Installation-With-Pre-Existing-Podman-Exe
    }
    # 'update-from-prev-to-current' {
    #    if (!$script:msiPath) {
    #        throw "Current installer path is not defined. Use '-msiPath <msi-path>' to define it."
    #    }
    #     if (!$script:previousMsiPath) {
    #         $script:previousMsiPath = Get-Latest-Podman-MSI-From-GitHub -arch $script:arch
    #     }
    #     Start-Scenario-Update-From-Prev-To-Current
    # }
    # 'update-from-prev-to-current-with-modified-config' {
    #    if (!$script:msiPath) {
    #        throw "Current installer path is not defined. Use '-msiPath <msi-path>' to define it."
    #    }
    #     if (!$script:previousMsiPath) {
    #         $script:previousMsiPath = Get-Latest-Podman-MSI-From-GitHub -arch $script:arch
    #     }
    #     Start-Scenario-Update-From-Prev-To-Current-With-Modified-Config
    # }
    # 'update-from-prev-to-current-with-removed-config' {
    #    if (!$script:msiPath) {
    #        throw "Current installer path is not defined. Use '-msiPath <msi-path>' to define it."
    #    }
    #     if (!$script:previousMsiPath) {
    #         $script:previousMsiPath = Get-Latest-Podman-MSI-From-GitHub -arch $script:arch
    #     }
    #     Start-Scenario-Update-From-Prev-To-Current-With-Removed-Config
    # }
    'update-from-current-to-next' {
        if (!$script:msiPath) {
            throw "Current installer path is not defined. Use '-msiPath <msi-path>' to define it."
        }
        if (!$script:nextMsiPath) {
            throw "Next version installer path is not defined. Use '-nextMsiPath <msi-path>' to define it."
        }
        Start-Scenario-Update-From-Current-To-Next
    }
    'update-from-current-to-next-with-modified-config' {
        if (!$script:msiPath) {
            throw "Current installer path is not defined. Use '-msiPath <msi-path>' to define it."
        }
        if (!$script:nextMsiPath) {
            throw "Next version installer path is not defined. Use '-nextMsiPath <msi-path>' to define it."
        }
        Start-Scenario-Update-From-Current-To-Next-With-Modified-Config
    }
    'update-from-current-to-next-with-removed-config' {
        if (!$script:msiPath) {
            throw "Current installer path is not defined. Use '-msiPath <msi-path>' to define it."
        }
        if (!$script:nextMsiPath) {
            throw "Next version installer path is not defined. Use '-nextMsiPath <msi-path>' to define it."
        }
        Start-Scenario-Update-From-Current-To-Next-With-Removed-Config
    }
    'update-from-legacy-to-current' {
        if (!$script:msiPath) {
            throw "Current installer path is not defined. Use '-msiPath <msi-path>' to define it."
        }
        if (!$script:previousSetupExePath) {
            $script:previousSetupExePath = Get-Latest-Podman-Setup-From-GitHub -arch $script:arch
        }
        Start-Scenario-Update-From-Legacy-To-Current
    }
    'all' {
        if (!$script:msiPath) {
            throw "Current installer path is not defined. Use '-msiPath <msi-path>' to define it."
        }
        if (!$script:nextMsiPath) {
            throw "Next version installer path is not defined. Use '-nextMsiPath <msi-path>' to define it."
        }
        # if (!$script:previousMsiPath) {
        #     $script:previousMsiPath = Get-Latest-Podman-MSI-From-GitHub -arch $script:arch
        # }
        if (!$script:previousSetupExePath) {
            $script:previousSetupExePath = Get-Latest-Podman-Setup-From-GitHub -arch $script:arch
        }
        Start-Scenario-Installation-Green-Field
        Start-Scenario-Installation-Skip-Config-Creation-Flag
        Start-Scenario-Installation-With-Pre-Existing-Podman-Exe
        # Start-Scenario-Update-From-Prev-To-Current
        # Start-Scenario-Update-From-Prev-To-Current-With-Modified-Config
        # Start-Scenario-Update-From-Prev-To-Current-With-Removed-Config
        Start-Scenario-Update-From-Current-To-Next
        Start-Scenario-Update-From-Current-To-Next-With-Modified-Config
        Start-Scenario-Update-From-Current-To-Next-With-Removed-Config
        Start-Scenario-Update-From-Legacy-To-Current
    }
}
