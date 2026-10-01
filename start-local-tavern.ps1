param(
    [string]$AgnaiPath = (Join-Path (Split-Path $PSScriptRoot -Parent) 'agnai'),
    [string]$BackendUrl = 'http://localhost:3000',
    [string]$PortalUrl = 'http://localhost:3107',
    [int]$Port = 3200,
    [string]$DatabaseName = 'new_api_agnai',
    [switch]$Build
)

$ErrorActionPreference = 'Stop'
$upstream = (Resolve-Path -LiteralPath $AgnaiPath).Path
if (-not (Test-Path -LiteralPath (Join-Path $upstream 'srv\new-api.ts'))) {
    throw 'The AgnAI New API integration source is missing.'
}
if (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) {
    throw "Port $Port is already in use. No existing process was stopped."
}
$runtime = Join-Path $PSScriptRoot ".local-tests\agnai-runtime-$Port"
$assets = Join-Path $runtime 'assets'
New-Item -ItemType Directory -Path $runtime, $assets -Force | Out-Null

$env:NEW_API_URL = $BackendUrl
$env:NEW_API_PORTAL_URL = $PortalUrl
$env:NEW_API_PUBLIC_URL = "http://$(([uri]$PortalUrl).Host):$Port"
$env:NEW_API_BRIDGE = '1'
$env:PORT = "$Port"
$env:APP_HOST = '127.0.0.1'
$env:DB_HOST = '127.0.0.1'
$env:DB_PORT = '27028'
$env:DB_NAME = $DatabaseName
$env:REDIS_HOST = '127.0.0.1'
$env:REDIS_PORT = '6388'
$env:BROADCAST_HOST = '127.0.0.1'
$env:BROADCAST_PORT = '6388'
$env:SKIP_MODEL_LISTS = '1'
$env:ADAPTERS = 'openai'
$env:JSON_FOLDER = $runtime
$env:ASSET_FOLDER = $assets
$env:AUTH_URLS = $PortalUrl
$env:NODE_TLS_REJECT_UNAUTHORIZED = '1'

Push-Location $upstream
try {
    if ($Build) {
        & pnpm.cmd exec tsc -p srv.tsconfig.json --pretty false
        if ($LASTEXITCODE -ne 0) { throw 'AgnAI server build failed.' }
        & pnpm.cmd exec parcel build web/index.html --no-optimize --log-level warn
        if ($LASTEXITCODE -ne 0) { throw 'AgnAI frontend build failed.' }
    }
    if (-not (Test-Path -LiteralPath 'dist\index.html')) {
        throw 'Build AgnAI first with -Build.'
    }
    $process = Start-Process -FilePath (Get-Command node.exe).Source `
        -ArgumentList 'srv/start.js' -WorkingDirectory $upstream -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $runtime 'server.log') `
        -RedirectStandardError (Join-Path $runtime 'server-error.log') -PassThru
    Write-Output "AgnAI PID: $($process.Id)"
    Write-Output "$($env:NEW_API_PUBLIC_URL)/character/list"
} finally {
    Pop-Location
}
