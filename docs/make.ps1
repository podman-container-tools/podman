function Get-Podman-Commands-List{
    param (
        [string]$podmanClient,
        [string]$command
    );
    if(!$podmanClient) {
        $podmanClient="$PSScriptRoot\..\bin\windows\podman.exe"
    }
    if($command) {
        $podmanHelpCommand="help $command"
        Write-Host "Retrieving the list of ""podman $command"" subcommands."
    } else {
        $podmanHelpCommand="help"
        Write-Host "Retrieving the list of ""podman"" commands."
    }

    # Retrieve the list of subcommands of $command
    # e.g. "podman help machine" returns the list of
    #     "podman machine" subcommands: info, init, etc...
    $subCommands = @()
    $subCommands = Invoke-Expression "$podmanClient $podmanHelpCommand" |
        Select-String -Pattern "^\s*Available Commands:" -Context 0, 1000 | Out-String -Stream |
        Select-String -Pattern "^\s+$" -Context 1000, 0 | Out-String -Stream |
        Select-String -Pattern ">\s*Available Commands:|^>\s*$|^\s*$" -NotMatch | Out-String -Stream |
        ForEach-Object { $_ -replace '^\s*(\w+)\s+.*$', '$1' } | Where-Object { $_ -ne "" }

    if ($command) {
        $subCommands = $subCommands | ForEach-Object { "$command $_" }
    }

    # Recursively get the list of sub-subcommands for each subcommand
    foreach ($subCommand in $subCommands) {

        $subSubCommands = @()
        $subSubCommands = Get-Podman-Commands-List -podmanClient "$podmanClient" -command "${subCommand}"

        if ($subSubCommands) {
            $subCommands += $subSubCommands
        }
    }

    return $subCommands
}

function Build-Podman-For-Windows-HTML-Page{
    $srcFolder = "$PSScriptRoot\tutorials"
    $srcFile = "$srcFolder\podman-for-windows.md"
    $destFolder = "$PSScriptRoot\build\remote"
    $destFile = "$destFolder\podman-for-windows.html"
    $cssFile = "$PSScriptRoot\standalone-styling.css"
    $pandocOptions = "--ascii --from markdown-smart -c $cssFile --standalone " +
                     "--embed-resources --metadata title=""Podman for Windows"" " +
                     "-V title="

    Write-Host -NoNewline "Generating $destFile from $srcFile..."
    Push-Location $srcFolder
    New-Item -ItemType Directory -Force -Path $destFolder | Out-Null
    Invoke-Expression "pandoc $pandocOptions $srcFile > $destFile"
    Pop-Location
    Write-Host "done."
}

function Build-Podman-Remote-HTML-Page{
    $markdownFolder = "$PSScriptRoot\source\markdown"
    # Look for all podman-remote*.md files in the markdown folder
    Get-ChildItem -Path "$markdownFolder" -Filter "podman-remote*.md" | ForEach-Object {
        # Extract the command name from the file name
        $command = $_.Name -replace '^podman-(.*).1.md$','$1'
        # Generate the documentation HTML page
        Build-Podman-Command-HTML-Page -command $command
    }
}

function Find-Podman-Command-Markdown-File{
    param (
        [string]$command
    );
    # A podman command documentation can be in one of the following files
    $markdownFolder = "$PSScriptRoot\source\markdown"
    $srcFileMdIn = "$markdownFolder\podman-$command.1.md.in"
    $srcFileMd = "$markdownFolder\podman-$command.1.md"
    $linkFile = "$markdownFolder\links\podman-$command.1"

    if (Test-Path -Path $srcFileMdIn -PathType Leaf) {
        return $srcFileMdIn
    } elseif (Test-Path -Path $srcFileMd -PathType Leaf) {
        return $srcFileMd
    } elseif (Test-Path -Path $linkFile -PathType Leaf) {
        # In $linkFile there is a link to a markdown file
        $srcFile = Get-Content -Path $linkFile
        # $srcFile is something like ".so man1/podman-attach.1"
        # and the markdown file is "podman-attach.1.md"
        $srcFile = $srcFile -replace ".so man1/", ""
        $srcFileMdIn = "$markdownFolder\$srcFile.md.in"
        $srcFileMd = "$markdownFolder\$srcFile.md"
        if (Test-Path -Path "$srcFileMdIn" -PathType Leaf) {
            return "$srcFileMdIn"
        } elseif (Test-Path -Path $srcFileMd -PathType Leaf) {
            return "$srcFileMd"
        }
    }
    return $null
}

function Build-Podman-Command-HTML-Page{
    param (
        [string]$command
    );

    $destFile = "$PSScriptRoot\build\remote\podman-$command.html"
    $srcFile = Find-Podman-Command-Markdown-File -command $command

    if (!$srcFile) {
        Write-Host "Couldn't find the documentation source file for $command. Skipping."
        continue
    }

    $pandocOptions = "--ascii --standalone --from markdown-smart " +
                     "--lua-filter=$PSScriptRoot\links-to-html.lua " +
                     "--lua-filter=$PSScriptRoot\use-pagetitle.lua"

    Write-Host -NoNewline "Generating $command documentation..."
    Invoke-Expression "pandoc $pandocOptions -o $destFile $srcFile" | Out-Null
    Write-Host "done."
}

# Generate podman-for-windows.html
Build-Podman-For-Windows-HTML-Page

# Generate podman-remote*.html
Build-Podman-Remote-HTML-Page

# Get the list of podman commands on Windows
if ($args[1]) {
    $commands = Get-Podman-Commands-List "-podmanClient $args[1]"
}
else {
    $commands = Get-Podman-Commands-List
}

# Generate podman commands documentation
foreach ($command in $commands) {
    # Replace spaces with hyphens in the command name
    # e.g. machine os apply becomes machine-os-apply
    $command = $command -replace ' ', '-'
    Build-Podman-Command-HTML-Page -command $command
}
