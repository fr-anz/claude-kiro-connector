#Requires -Version 5.1
[CmdletBinding()]
param(
    [switch]$Help
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot

function Get-EnvOrDefault([string]$Name, [string]$DefaultValue) {
    $value = [Environment]::GetEnvironmentVariable($Name, "Process")
    if ($value) { return $value }
    return $DefaultValue
}

function Get-Sha256Hex([string]$Path) {
    $hashCommand = Get-Command Get-FileHash -ErrorAction SilentlyContinue
    if ($hashCommand) {
        return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash
    }
    # Fallback for systems whose Utility module lacks Get-FileHash.
    $stream = [IO.File]::OpenRead($Path)
    try {
        $bytes = [Security.Cryptography.SHA256]::Create().ComputeHash($stream)
    } finally {
        $stream.Close()
    }
    return ([BitConverter]::ToString($bytes) -replace '-', '')
}

function Write-Usage {
    @"
Usage: .\scripts\install.ps1

  Installs KiroCC + an isolated claude-kiro launcher using official Claude Code.

Environment overrides:
  CLAUDE_KIRO_INSTALL_ROOT
  CLAUDE_KIRO_STATE_ROOT
  CLAUDE_KIRO_BIN_DIR
  KIROCC_PORT
  KIROCC_DB_PATH
  KIRO_API_KEY       Kiro upstream API key; makes Kiro CLI optional.
  KIRO_API_REGION
"@ | Write-Host
}

if ($Help) {
    Write-Usage
    exit 0
}

$InstallRoot = Get-EnvOrDefault "CLAUDE_KIRO_INSTALL_ROOT" (Join-Path $env:LOCALAPPDATA "claude-kiro")
$StateRoot = Get-EnvOrDefault "CLAUDE_KIRO_STATE_ROOT" (Join-Path $env:USERPROFILE ".claude-kiro")
$UserBinDir = Get-EnvOrDefault "CLAUDE_KIRO_BIN_DIR" (Join-Path $env:USERPROFILE ".local\bin")
$GatewayPortText = Get-EnvOrDefault "KIROCC_PORT" "3457"

$GatewayPort = 0
if (-not [int]::TryParse($GatewayPortText, [ref]$GatewayPort) -or $GatewayPort -lt 1 -or $GatewayPort -gt 65535) {
    throw "KIROCC_PORT must be between 1 and 65535"
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "missing prerequisite: go"
}

$goCommand = Get-Command go -CommandType Application -ErrorAction Stop |
    Select-Object -First 1
$goPath = if ($goCommand.Path) { $goCommand.Path } else { $goCommand.Source }
$goVersionText = (& $goPath version 2>$null | Select-Object -First 1)
$goVersionMatch = [regex]::Match([string]$goVersionText, '\bgo(?<version>[0-9]+(?:\.[0-9]+){1,2})')
if ($LASTEXITCODE -ne 0 -or -not $goVersionMatch.Success) {
    throw "unable to determine the Go version from '$goPath' (reported '$goVersionText')"
}
$installedGoVersion = [Version]$goVersionMatch.Groups["version"].Value
$goDirective = Select-String -LiteralPath (Join-Path $ProjectRoot "go.mod") `
    -Pattern '^\s*go\s+(?<version>[0-9]+(?:\.[0-9]+){1,2})\s*$' |
    Select-Object -First 1
if (-not $goDirective) {
    throw "unable to read the required Go version from go.mod"
}
$requiredGoVersion = [Version]$goDirective.Matches[0].Groups["version"].Value
if ($installedGoVersion -lt $requiredGoVersion) {
    throw @"
Go $installedGoVersion at '$goPath' is too old. This project requires Go $requiredGoVersion or newer.

Upgrade Go, open a new PowerShell window, and verify the active executable:

  winget upgrade --id GoLang.Go -e
  # If Go was not installed through winget:
  winget install --id GoLang.Go -e
  where.exe go
  go version

If where.exe lists more than one go.exe, remove the older entry that appears
first in PATH, then rerun this installer.
"@
}
Write-Host "Go prerequisite: $goVersionText ($goPath)"

$OfficialClaudeCommand = Get-Command claude -CommandType Application,ExternalScript -ErrorAction SilentlyContinue |
    Select-Object -First 1
if (-not $OfficialClaudeCommand) {
    throw "official Claude Code command not found"
}
$OfficialClaudePath = if ($OfficialClaudeCommand.Path) {
    $OfficialClaudeCommand.Path
} else {
    $OfficialClaudeCommand.Source
}
$OfficialClaudePath = [IO.Path]::GetFullPath($OfficialClaudePath)
$OfficialClaudeHash = Get-Sha256Hex $OfficialClaudePath

$kiroDatabaseCandidates = if ($env:KIROCC_DB_PATH) {
    @($env:KIROCC_DB_PATH)
} else {
    @(
        (Join-Path $env:USERPROFILE ".local\share\kiro-cli\data.sqlite3"),
        (Join-Path $env:LOCALAPPDATA "kiro-cli\data.sqlite3"),
        (Join-Path $env:APPDATA "kiro-cli\data.sqlite3")
    ) | Where-Object { $_ }
}
$kiroDatabasePath = $kiroDatabaseCandidates | Where-Object {
    Test-Path -LiteralPath $_ -PathType Leaf
} | Select-Object -First 1
if ($env:KIRO_API_KEY) {
    Write-Host "Kiro authentication: API-key mode (Kiro CLI is not required)."
} elseif ($kiroDatabasePath) {
    $kiroCli = Get-Command kiro-cli -ErrorAction SilentlyContinue
    if ($kiroCli) {
        & kiro-cli whoami *> $null
        if ($LASTEXITCODE -ne 0) {
            throw @"
Kiro CLI database exists, but kiro-cli whoami did not confirm a logged-in profile.
Run:

  kiro-cli login
  kiro-cli whoami

Then rerun this installer. Request traffic will still be sent directly by the
kirocc gateway; this check only validates the credential it will read.
"@
        }
        Write-Host "Kiro authentication: existing login confirmed (whoami output redacted)."
    } else {
        Write-Host "Kiro authentication: existing login database found (Kiro CLI command unavailable; live status not checked)."
    }
} else {
    $kiroCliState = if (Get-Command kiro-cli -ErrorAction SilentlyContinue) {
        "Kiro CLI is installed, but its login database was not found."
    } else {
        "Kiro CLI is not installed and no existing login database was found."
    }
    $expectedDatabase = if ($kiroDatabaseCandidates.Count -gt 0) {
        $kiroDatabaseCandidates -join "; "
    } else {
        "<no database candidate>"
    }
    throw @"
No usable Kiro credential source found.
$kiroCliState

Kiro CLI is used only to create the local login credential; request traffic is
sent directly by the kirocc gateway. Install/login with:

  irm 'https://cli.kiro.dev/install.ps1' | iex
  kiro-cli login
  kiro-cli whoami

Or set KIRO_API_KEY (and optionally KIRO_API_REGION) before running this
installer and whenever claude-kiro is launched.
Expected database: $expectedDatabase
"@
}

New-Item -ItemType Directory -Force -Path $InstallRoot | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $InstallRoot "bin") | Out-Null
New-Item -ItemType Directory -Force -Path $StateRoot | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $StateRoot "claude-config") | Out-Null
New-Item -ItemType Directory -Force -Path $UserBinDir | Out-Null

$TemporaryRoot = Join-Path $env:TEMP "claude-kiro-$([Guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Force -Path $TemporaryRoot | Out-Null

try {
    Write-Host "Building patched kirocc..."
    $temporaryGateway = Join-Path $TemporaryRoot "kirocc-native-websearch.exe"
    $oldGoExperiment = $env:GOEXPERIMENT
    try {
        $env:GOEXPERIMENT = "jsonv2"
        & go build -trimpath -o $temporaryGateway (Join-Path $ProjectRoot "cmd\kirocc")
        if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
    } finally {
        $env:GOEXPERIMENT = $oldGoExperiment
    }
    $installedGateway = Join-Path $InstallRoot "bin\kirocc-native-websearch.exe"
    Copy-Item -LiteralPath $temporaryGateway -Destination $installedGateway -Force

    $isolatedConfigDir = Join-Path $StateRoot "claude-config"
    foreach ($configName in @("CLAUDE.md", "settings.json")) {
        $destination = Join-Path $isolatedConfigDir $configName
        if (-not (Test-Path -LiteralPath $destination)) {
            Copy-Item -LiteralPath (Join-Path $ProjectRoot "config\$configName") -Destination $destination
        }
    }

    $launcherScript = Join-Path $UserBinDir "claude-kiro.ps1"
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot "windows\claude-kiro.ps1") -Destination $launcherScript -Force
    $launcherConfig = [ordered]@{
        schemaVersion = 2
        installRoot = [IO.Path]::GetFullPath($InstallRoot)
        stateRoot = [IO.Path]::GetFullPath($StateRoot)
        defaultPort = $GatewayPort
        runtimeCommand = $OfficialClaudePath
    }
    $launcherConfig | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $UserBinDir "claude-kiro-launcher.json") -Encoding UTF8
    $launcherCmd = @'
@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0claude-kiro.ps1" %*
exit /b %ERRORLEVEL%
'@
    Set-Content -LiteralPath (Join-Path $UserBinDir "claude-kiro.cmd") -Value $launcherCmd.TrimEnd() -Encoding ASCII
    Set-Content -LiteralPath (Join-Path $UserBinDir "ck.cmd") -Value $launcherCmd.TrimEnd() -Encoding ASCII
    $ckScript = @'
# ck is a short alias for claude-kiro; the real launcher is claude-kiro.ps1.
& (Join-Path $PSScriptRoot "claude-kiro.ps1") @args
'@
    Set-Content -LiteralPath (Join-Path $UserBinDir "ck.ps1") -Value $ckScript.TrimEnd() -Encoding UTF8

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $pathEntries = @($userPath -split ';' | Where-Object { $_ })
    if (-not ($pathEntries | Where-Object { $_.TrimEnd('\') -ieq $UserBinDir.TrimEnd('\') })) {
        $newUserPath = if ($userPath) { "$UserBinDir;$userPath" } else { $UserBinDir }
        [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
    }
    if (-not (($env:Path -split ';') | Where-Object { $_.TrimEnd('\') -ieq $UserBinDir.TrimEnd('\') })) {
        $env:Path = "$UserBinDir;$env:Path"
    }

    $officialAfter = Get-Sha256Hex $OfficialClaudePath
    if ($officialAfter -ne $OfficialClaudeHash) {
        throw "official Claude Code changed during installation; refusing to report success"
    }

    Write-Host ""
    Write-Host "Installed successfully:" -ForegroundColor Green
    Write-Host "  launcher: $(Join-Path $UserBinDir 'claude-kiro.cmd')"
    Write-Host "  gateway:  $installedGateway"
    Write-Host "  runtime:  $OfficialClaudePath"
    Write-Host "  config:   $isolatedConfigDir"
    Write-Host ""
    Write-Host "The official claude command was not modified by this installer."
} finally {
    Remove-Item -LiteralPath $TemporaryRoot -Recurse -Force -ErrorAction SilentlyContinue
}
