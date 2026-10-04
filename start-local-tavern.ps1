param(
    [string]$RisuaiPath = (Join-Path (Split-Path $PSScriptRoot -Parent) 'risuai'),
    [string]$BackendUrl = 'http://localhost:3000',
    [string]$PortalUrl = 'http://localhost:3000',
    [int]$Port = 3200,
    [switch]$Build
)

$ErrorActionPreference = 'Stop'
$upstream = (Resolve-Path -LiteralPath $RisuaiPath).Path
if (-not (Test-Path -LiteralPath (Join-Path $upstream 'server\node\new-api.cjs'))) {
    throw 'The RisuAI New API integration source is missing.'
}
if (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) {
    throw "Port $Port is already in use. No existing process was stopped."
}
$runtime = Join-Path $PSScriptRoot ".local-tests\risuai-runtime-$Port"
New-Item -ItemType Directory -Path $runtime -Force | Out-Null

$env:NEW_API_URL = $BackendUrl
$env:NEW_API_PORTAL_URL = $PortalUrl
$env:NEW_API_PUBLIC_URL = "http://$(([uri]$PortalUrl).Host):$Port"
$env:NEW_API_DATA_DIR = Join-Path $runtime 'data'
$env:VITE_NEW_API_BRIDGE = '1'
$env:VITE_NEW_API_PORTAL_URL = $PortalUrl
# This launcher is for private loopback testing, not public hosting.
$env:VITE_RISU_LEGAL_CONFIGURED = 'TRUE'
$env:PORT = "$Port"
$env:NODE_TLS_REJECT_UNAUTHORIZED = '1'

Push-Location $upstream
try {
    if ($Build) {
        & node.exe node_modules/vite/bin/vite.js build
        if ($LASTEXITCODE -ne 0) { throw 'RisuAI frontend build failed.' }
    }
    if (-not (Test-Path -LiteralPath 'dist\index.html')) {
        throw 'Build RisuAI first with -Build.'
    }
    $process = Start-Process -FilePath (Get-Command node.exe).Source `
        -ArgumentList 'server/node/server.cjs' -WorkingDirectory $upstream -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $runtime 'server.log') `
        -RedirectStandardError (Join-Path $runtime 'server-error.log') -PassThru
    Write-Output "RisuAI PID: $($process.Id)"
    Write-Output "$($env:NEW_API_PUBLIC_URL)/?connect=1"
} finally {
    Pop-Location
}
