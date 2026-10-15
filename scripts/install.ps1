<#
.SYNOPSIS
Installs BepInEx, the UnbeatableRPC plugin, the unbeatable-rpc daemon and its
settings app for UNBEATABLE on Windows. The Windows counterpart of install.sh.

.DESCRIPTION
Everything goes in your user account, so it doesn't need administrator rights,
unless the Steam library itself is locked down:

  daemon, settings app, this script   %LOCALAPPDATA%\Programs\unbeatable-rpc
  installer state, daemon log         %LOCALAPPDATA%\unbeatable-rpc
  settings                            %APPDATA%\unbeatable-rpc
  start at login                      HKCU\...\CurrentVersion\Run, "unbeatable-rpc"
  app menu                            Start menu, "UNBEATABLE RPC"

Once installed, the settings app's Uninstall button, or this script with
-Uninstall, removes it again.

.PARAMETER GameDir
UNBEATABLE's install folder. Default: found through Steam's libraries.

.PARAMETER BepInExZip
A downloaded BepInEx_win_x64 zip to use instead of fetching it.

.PARAMETER Uninstall
Remove the plugin, the daemon, the settings app and its settings.

.PARAMETER All
With -Uninstall: also remove BepInEx, if this installer added it.

.PARAMETER Yes
Don't ask questions.

.PARAMETER WaitPid
Wait for this process to exit first. The settings app passes its own, since
its files can't be removed while it runs.
#>
[CmdletBinding()]
param(
    [string]$GameDir,
    [string]$BepInExZip,
    [switch]$Uninstall,
    [switch]$All,
    [switch]$Yes,
    [int]$WaitPid = 0
)

Set-StrictMode -Version 3
$ErrorActionPreference = 'Stop'

$AppId = '2240620'
$BepInExVersion = '5.4.23.5'
$BepInExUrl = "https://github.com/BepInEx/BepInEx/releases/download/v$BepInExVersion/BepInEx_win_x64_$BepInExVersion.zip"
$BepInExSha256 = '82F9878551030F54657792C0740D9D51A09500EEAE1FBA21106B0C441E6732C4'
$BepInExRootFiles = @('winhttp.dll', 'doorstop_config.ini', '.doorstop_version', 'changelog.txt')
$PluginConfig = 'xyz.diaduck.unbeatable-rpc.cfg'

$Daemon = 'unbeatable-rpc.exe'
$SettingsApp = 'unbeatable-rpc-settings.exe'
$Here = $PSScriptRoot
$ProgramsDir = Join-Path $env:LOCALAPPDATA 'Programs\unbeatable-rpc'
$StateDir = Join-Path $env:LOCALAPPDATA 'unbeatable-rpc'
$StateFile = Join-Path $StateDir 'install.json'
$SettingsDir = Join-Path $env:APPDATA 'unbeatable-rpc'
$WebviewDir = Join-Path $env:APPDATA $SettingsApp
$RunKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$RunValue = 'unbeatable-rpc'
$Shortcut = Join-Path ([Environment]::GetFolderPath('Programs')) 'UNBEATABLE RPC.lnk'

function Say([string]$Text) { Write-Host $Text }
function Step([string]$Text) { Write-Host ''; Write-Host "==> $Text" -ForegroundColor Cyan }

function Done([int]$Code) {
    if ($WaitPid -ne 0) { Read-Host 'Press Enter to close' | Out-Null }
    exit $Code
}

function Die([string]$Text) {
    Write-Host "install.ps1: $Text" -ForegroundColor Red
    Done 1
}

function Confirm([string]$Question) {
    if ($Yes) { return $true }
    $reply = Read-Host "$Question [Y/n]"
    return ($reply -eq '' -or $reply -match '^[Yy]')
}

function Get-SteamRoots {
    $roots = @()
    foreach ($key in 'HKCU:\Software\Valve\Steam', 'HKLM:\SOFTWARE\WOW6432Node\Valve\Steam', 'HKLM:\SOFTWARE\Valve\Steam') {
        $props = Get-ItemProperty -Path $key -ErrorAction SilentlyContinue
        if (-not $props) { continue }
        foreach ($name in 'SteamPath', 'InstallPath') {
            if ($props.PSObject.Properties[$name]) {
                $roots += [IO.Path]::GetFullPath($props.$name)
            }
        }
    }
    $roots | Where-Object { Test-Path (Join-Path $_ 'steamapps\libraryfolders.vdf') } | Sort-Object -Unique
}

function Get-SteamLibraries {
    $libs = @()
    foreach ($root in Get-SteamRoots) {
        $libs += $root
        $vdf = Get-Content -LiteralPath (Join-Path $root 'steamapps\libraryfolders.vdf')
        foreach ($line in $vdf) {
            if ($line -match '^\s*"path"\s*"(.*)"\s*$') {
                $libs += [IO.Path]::GetFullPath($Matches[1].Replace('\\', '\'))
            }
        }
    }
    $libs | Sort-Object -Unique
}

function Find-Game {
    foreach ($lib in Get-SteamLibraries) {
        $manifest = Join-Path $lib "steamapps\appmanifest_$AppId.acf"
        if (-not (Test-Path -LiteralPath $manifest)) { continue }
        foreach ($line in Get-Content -LiteralPath $manifest) {
            if ($line -match '^\s*"installdir"\s*"(.*)"\s*$') {
                return Join-Path $lib "steamapps\common\$($Matches[1])"
            }
        }
    }
    return $null
}

function Test-GameRunning { [bool](Get-Process -Name 'UNBEATABLE' -ErrorAction SilentlyContinue) }

function Stop-Program([string]$Exe) {
    $procs = Get-Process -Name ([IO.Path]::GetFileNameWithoutExtension($Exe)) -ErrorAction SilentlyContinue
    if ($procs) {
        $procs | Stop-Process -Force
        $procs | Wait-Process -Timeout 5 -ErrorAction SilentlyContinue
    }
}

function Read-State {
    $state = [ordered]@{ gameDir = ''; bepinexInstalled = $false }
    if (Test-Path -LiteralPath $StateFile) {
        $saved = Get-Content -LiteralPath $StateFile -Raw | ConvertFrom-Json
        if ($saved.PSObject.Properties['gameDir']) { $state.gameDir = [string]$saved.gameDir }
        if ($saved.PSObject.Properties['bepinexInstalled']) { $state.bepinexInstalled = [bool]$saved.bepinexInstalled }
    }
    return $state
}

function Write-State($State) {
    New-Item -ItemType Directory -Force -Path $StateDir | Out-Null
    [IO.File]::WriteAllText($StateFile, (ConvertTo-Json $State), (New-Object Text.UTF8Encoding $false))
}

# Expand-Zip extracts without overwriting existing files, like unzip -n.
function Expand-Zip([string]$Zip, [string]$Dest) {
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $archive = [IO.Compression.ZipFile]::OpenRead($Zip)
    try {
        foreach ($entry in $archive.Entries) {
            $target = [IO.Path]::GetFullPath((Join-Path $Dest $entry.FullName))
            if ($entry.FullName.EndsWith('/')) {
                New-Item -ItemType Directory -Force -Path $target | Out-Null
            } elseif (-not (Test-Path -LiteralPath $target)) {
                New-Item -ItemType Directory -Force -Path (Split-Path $target) | Out-Null
                [IO.Compression.ZipFileExtensions]::ExtractToFile($entry, $target)
            }
        }
    } finally {
        $archive.Dispose()
    }
}

function Find-GameDir($State) {
    Step 'Finding UNBEATABLE'
    $dir = $GameDir
    if (-not $dir) { $dir = Find-Game }
    if (-not $dir) { Die "couldn't find UNBEATABLE (app $AppId) in any Steam library; pass -GameDir" }
    if (-not (Test-Path -LiteralPath (Join-Path $dir 'UNBEATABLE.exe'))) { Die "no UNBEATABLE.exe in $dir" }
    Say "Game: $dir"
    if (Test-GameRunning) { Die 'UNBEATABLE is running; quit it first' }
    if (-not (Test-Path -LiteralPath (Join-Path $dir 'UNBEATABLE_Data\Managed\Assembly-CSharp.dll'))) {
        Die "$dir isn't the Mono build this plugin was made for"
    }
    $probe = Join-Path $dir '.unbeatable-rpc-write-test'
    try {
        [IO.File]::WriteAllText($probe, '')
        Remove-Item -LiteralPath $probe
    } catch {
        Die "can't write to $dir. Run install.cmd as administrator (right-click > Run as administrator)."
    }
    $State.gameDir = $dir
}

function Install-BepInEx($State) {
    Step 'BepInEx'
    $game = $State.gameDir
    if ((Test-Path -LiteralPath (Join-Path $game 'BepInEx\core\BepInEx.dll')) -and (Test-Path -LiteralPath (Join-Path $game 'winhttp.dll'))) {
        Say 'Already installed, leaving it as is.'
        return
    }
    $zip = $BepInExZip
    $download = $null
    if (-not $zip) {
        $download = Join-Path ([IO.Path]::GetTempPath()) "BepInEx_win_x64_$BepInExVersion-$PID.zip"
        $zip = $download
        Say "Downloading BepInEx $BepInExVersion"
        # Windows PowerShell 5.1 doesn't offer TLS 1.2, which GitHub needs, by default
        [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
        $ProgressPreference = 'SilentlyContinue'
        Invoke-WebRequest -UseBasicParsing -Uri $BepInExUrl -OutFile $zip
    }
    try {
        if ((Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash -ne $BepInExSha256) {
            Die "$zip isn't BepInEx_win_x64_$BepInExVersion.zip (checksum mismatch)"
        }
        Expand-Zip $zip $game
    } finally {
        if ($download) { Remove-Item -LiteralPath $download -ErrorAction SilentlyContinue }
    }
    $State.bepinexInstalled = $true
    Say "Installed BepInEx $BepInExVersion into the game folder."
}

function Install-Files($State) {
    Step 'Plugin, daemon and settings app'
    # downloaded files are marked as from the internet, which SmartScreen blocks
    Get-ChildItem -LiteralPath $Here -File | Unblock-File
    Stop-Program $Daemon
    Stop-Program $SettingsApp

    $pluginDir = Join-Path $State.gameDir 'BepInEx\plugins\UnbeatableRPC'
    New-Item -ItemType Directory -Force -Path $pluginDir | Out-Null
    Copy-Item -LiteralPath (Join-Path $Here 'UnbeatableRPC.dll') -Destination $pluginDir -Force

    New-Item -ItemType Directory -Force -Path $ProgramsDir | Out-Null
    foreach ($f in $Daemon, $SettingsApp, 'install.ps1') {
        $src = Join-Path $Here $f
        if ((Test-Path -LiteralPath $src) -and ($Here -ne $ProgramsDir)) {
            Copy-Item -LiteralPath $src -Destination $ProgramsDir -Force
        }
    }
    Say "Installed the plugin into the game's BepInEx\plugins and the programs into $ProgramsDir."

    $daemonPath = Join-Path $ProgramsDir $Daemon
    New-ItemProperty -Path $RunKey -Name $RunValue -Value "`"$daemonPath`"" -PropertyType String -Force | Out-Null
    Start-Process -FilePath $daemonPath
    Say 'Started the daemon; it starts at login from now on.'

    $settingsPath = Join-Path $ProgramsDir $SettingsApp
    if (Test-Path -LiteralPath $settingsPath) {
        $shell = New-Object -ComObject WScript.Shell
        $link = $shell.CreateShortcut($Shortcut)
        $link.TargetPath = $settingsPath
        $link.WorkingDirectory = $ProgramsDir
        $link.Description = 'Discord Rich Presence settings for UNBEATABLE'
        $link.IconLocation = '%SystemRoot%\System32\shell32.dll,-16826'
        $link.Save()
        Say 'Added "UNBEATABLE RPC" to the Start menu.'
    }
}

function Invoke-Install {
    foreach ($f in $Daemon, 'UnbeatableRPC.dll') {
        if (-not (Test-Path -LiteralPath (Join-Path $Here $f))) {
            Die "$Daemon and UnbeatableRPC.dll must be next to install.ps1 (run it from the release folder)"
        }
    }
    $state = Read-State
    if (-not $GameDir -and $state.gameDir -and (Test-Path -LiteralPath $state.gameDir)) { $script:GameDir = $state.gameDir }
    Find-GameDir $state
    Install-BepInEx $state
    Install-Files $state
    Write-State $state

    Say ''
    Say 'Installed. Launch UNBEATABLE and your Discord status will follow along.'
    $settingsPath = Join-Path $ProgramsDir $SettingsApp
    if ((Test-Path -LiteralPath $settingsPath) -and -not $Yes) {
        Say ''
        if (Confirm 'Open the settings app now? (pick a style, check Discord is connected)') {
            Start-Process -FilePath $settingsPath
        }
        Say 'Open it any time from the Start menu: UNBEATABLE RPC.'
    }
}

function Invoke-Uninstall {
    if ($WaitPid -ne 0) {
        Say 'Waiting for the settings app to close...'
        Wait-Process -Id $WaitPid -Timeout 30 -ErrorAction SilentlyContinue
    }
    # this script may be in the folder about to go
    Set-Location -LiteralPath ([IO.Path]::GetTempPath())

    $state = Read-State
    $game = $GameDir
    if (-not $game) { $game = $state.gameDir }
    if (-not $game) { $game = Find-Game }
    if ($game -and (Test-GameRunning)) { Die 'UNBEATABLE is running; quit it first' }

    Step 'Daemon and settings app'
    Stop-Program $Daemon
    Stop-Program $SettingsApp
    Remove-ItemProperty -Path $RunKey -Name $RunValue -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $Shortcut -ErrorAction SilentlyContinue
    foreach ($f in $Daemon, $SettingsApp) {
        Remove-Item -LiteralPath (Join-Path $ProgramsDir $f) -ErrorAction SilentlyContinue
    }
    foreach ($d in $SettingsDir, $WebviewDir) {
        Remove-Item -LiteralPath $d -Recurse -Force -ErrorAction SilentlyContinue
    }
    foreach ($f in 'daemon.log', 'daemon.log.old') {
        Remove-Item -LiteralPath (Join-Path $StateDir $f) -ErrorAction SilentlyContinue
    }
    Say 'Removed the daemon, the settings app and its settings.'

    Step 'Plugin'
    if ($game -and (Test-Path -LiteralPath $game)) {
        Remove-Item -LiteralPath (Join-Path $game 'BepInEx\plugins\UnbeatableRPC') -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath (Join-Path $game "BepInEx\config\$PluginConfig") -ErrorAction SilentlyContinue
        Say "Removed the plugin and its config from $game."
    } else {
        Say "Couldn't find the game, so there's no plugin to remove (pass -GameDir if it moved)."
    }

    if ($All) {
        Step 'BepInEx'
        if ($state.bepinexInstalled -and $game -and (Test-Path -LiteralPath $game)) {
            foreach ($f in $BepInExRootFiles) {
                Remove-Item -LiteralPath (Join-Path $game $f) -Force -ErrorAction SilentlyContinue
            }
            Remove-Item -LiteralPath (Join-Path $game 'BepInEx') -Recurse -Force -ErrorAction SilentlyContinue
            $state.bepinexInstalled = $false
            Say 'Removed BepInEx from the game folder.'
        } else {
            Say "BepInEx wasn't installed by this installer, leaving it."
        }
    }

    if ($state.bepinexInstalled) {
        Write-State $state
        Say ''
        Say "BepInEx is still in place; run '$(Join-Path $ProgramsDir 'install.ps1') -Uninstall -All' to remove it too."
    } else {
        Remove-Item -LiteralPath $ProgramsDir -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $StateDir -Recurse -Force -ErrorAction SilentlyContinue
    }
    Say ''
    Say 'Uninstalled.'
}

if ($Uninstall) { Invoke-Uninstall } else { Invoke-Install }
Done 0
