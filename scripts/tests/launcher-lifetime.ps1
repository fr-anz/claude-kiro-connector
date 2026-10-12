param([Parameter(Mandatory=$true)][string]$Gateway)
$ErrorActionPreference = 'Stop'
$testRoot = Join-Path ([IO.Path]::GetTempPath()) ("claude-kiro-lifetime-" + [Guid]::NewGuid())
New-Item -ItemType Directory -Path $testRoot | Out-Null
$port = 45680
$url = "http://127.0.0.1:$port"
try {
    try { Invoke-WebRequest "$url/health" -UseBasicParsing -TimeoutSec 1 | Out-Null; throw 'test port occupied' } catch {
        if ($_.Exception.Message -eq 'test port occupied') { throw }
    }
    $runtime = Join-Path $testRoot 'runtime.cmd'
    Set-Content -LiteralPath $runtime -Encoding ASCII -Value "@echo off`r`necho lifetime-runtime-ok`r`nexit /b 0"
    $config = Join-Path $testRoot 'launcher.json'
    @{installRoot=$testRoot;stateRoot=$testRoot;runtimeCommand=$runtime;defaultPort=$port} | ConvertTo-Json | Set-Content $config
    $env:CLAUDE_KIRO_LAUNCHER_CONFIG = $config
    $env:CLAUDE_KIRO_STATE_ROOT = $testRoot
    $env:CLAUDE_KIRO_AUTH_CACHE = Join-Path $testRoot 'auth-cache.json'
    $env:CLAUDE_KIRO_BIN = (Resolve-Path $Gateway).Path
    $env:CLAUDE_KIRO_RUNTIME_BIN = $runtime
    $env:CLAUDE_KIRO_PORT = "$port"
    $env:CLAUDE_KIRO_URL = $url
    $env:CLAUDE_KIRO_API_KEY = 'local_lifetime_placeholder'
    $env:KIRO_API_KEY = 'ksk_lifetime_placeholder'
    $launcher = Join-Path $PSScriptRoot '../windows/claude-kiro.ps1'
    for ($session = 0; $session -lt 2; $session++) {
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $launcher --version
        if ($LASTEXITCODE -ne 0) { throw 'launcher failed' }
        $probe = [System.Net.WebRequest]::Create("$url/health")
        $probe.Proxy = $null; $probe.Timeout = 2000
        $response = $probe.GetResponse(); $response.Close()
    }
    Write-Output 'gateway survives launcher exit and is reused'
} finally {
    $pidFile = Join-Path $testRoot "gateway-$port.pid"
    if (Test-Path -LiteralPath $pidFile) {
        $gatewayPid = [int](Get-Content -LiteralPath $pidFile)
        Stop-Process -Id $gatewayPid -Force -ErrorAction SilentlyContinue
    }
    $resolvedRoot = [IO.Path]::GetFullPath($testRoot)
    $resolvedTemp = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\') + '\'
    if (-not $resolvedRoot.StartsWith($resolvedTemp, [StringComparison]::OrdinalIgnoreCase) -or (Split-Path $resolvedRoot -Leaf) -notlike 'claude-kiro-lifetime-*') { throw 'unsafe test cleanup path' }
    Remove-Item -LiteralPath $resolvedRoot -Recurse -Force -ErrorAction SilentlyContinue
}
