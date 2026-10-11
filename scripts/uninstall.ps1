#Requires -Version 5.1
[CmdletBinding()]
param(
    [switch]$PurgeState,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

if ($Help) {
    Write-Host "Usage: .\scripts\uninstall.ps1 [-PurgeState]"
    exit 0
}

function Get-EnvOrDefault([string]$Name, [string]$DefaultValue) {
    $value = [Environment]::GetEnvironmentVariable($Name, "Process")
    if ($value) { return $value }
    return $DefaultValue
}
function Assert-SafeRemovalRoot([string]$Path, [string]$ExpectedLeaf) {
    $fullPath = [IO.Path]::GetFullPath($Path).TrimEnd('\')
    $driveRoot = [IO.Path]::GetPathRoot($fullPath).TrimEnd('\')
    $profileRoot = [IO.Path]::GetFullPath($env:USERPROFILE).TrimEnd('\')
    if (-not $fullPath -or $fullPath -ieq $driveRoot -or $fullPath -ieq $profileRoot -or
        [IO.Path]::GetFileName($fullPath) -ine $ExpectedLeaf) {
        throw "refusing unsafe removal target: $Path"
    }
    return $fullPath
}

$defaultInstallRoot = Join-Path $env:LOCALAPPDATA "claude-kiro"
$InstallRoot = Get-EnvOrDefault "CLAUDE_KIRO_INSTALL_ROOT" $defaultInstallRoot
$StateRoot = Get-EnvOrDefault "CLAUDE_KIRO_STATE_ROOT" (Join-Path $env:USERPROFILE ".claude-kiro")
$UserBinDir = Get-EnvOrDefault "CLAUDE_KIRO_BIN_DIR" (Join-Path $env:USERPROFILE ".local\bin")

$InstallRoot = Assert-SafeRemovalRoot $InstallRoot "claude-kiro"
foreach ($name in @("claude-kiro.cmd", "claude-kiro.ps1", "claude-kiro-launcher.json", "ck.cmd", "ck.ps1")) {
    $path = Join-Path $UserBinDir $name
    if (Test-Path -LiteralPath $path) {
        Remove-Item -LiteralPath $path -Force
    }
}
if (Test-Path -LiteralPath $InstallRoot) {
    Remove-Item -LiteralPath $InstallRoot -Recurse -Force
}

if ($PurgeState) {
    $StateRoot = Assert-SafeRemovalRoot $StateRoot ".claude-kiro"
    if (Test-Path -LiteralPath $StateRoot) {
        Remove-Item -LiteralPath $StateRoot -Recurse -Force
    }
} else {
    Write-Host "Preserved state: $StateRoot"
}

Write-Host "Removed ClaudeCode Kiro runtime. Official Claude Code was not touched."
