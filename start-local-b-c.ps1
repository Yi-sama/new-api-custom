$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$executable = Join-Path $projectRoot 'bin\new-api-local-b-c.exe'
$database = Join-Path $projectRoot '.local-tests\abc-shared.db'
$pidFile = Join-Path $projectRoot '.local-tests\b-c-shared.pid'
$stdoutLog = Join-Path $projectRoot '.local-tests\b-c-shared.stdout.log'
$stderrLog = Join-Path $projectRoot '.local-tests\b-c-shared.stderr.log'
$statusUrl = 'http://localhost:3000/api/status'
$studioSecretFile = Join-Path $projectRoot '.local-tests\image-studio-session-secret'

function Test-Server {
    try {
        return (Invoke-WebRequest -Uri $statusUrl -UseBasicParsing -TimeoutSec 2).StatusCode -eq 200
    }
    catch {
        return $false
    }
}

if (Test-Server) {
    Start-Process 'http://localhost:3000/'
    exit 0
}

$listener = Get-NetTCPConnection -State Listen -LocalPort 3000 -ErrorAction SilentlyContinue
if ($listener) {
    Write-Error 'Port 3000 is already in use by another process.'
}

if (-not (Test-Path -LiteralPath $executable -PathType Leaf)) {
    Write-Error "Executable is missing: $executable"
}

New-Item -ItemType Directory -Path (Split-Path -Parent $database) -Force | Out-Null
if (-not (Test-Path -LiteralPath $database -PathType Leaf)) {
    New-Item -ItemType File -Path $database | Out-Null
}

$env:TRUSTED_PROXIES = if ($env:TRUSTED_PROXIES) { $env:TRUSTED_PROXIES } else { 'none' }

$studioSecret = if (Test-Path -LiteralPath $studioSecretFile -PathType Leaf) {
    (Get-Content -Raw -LiteralPath $studioSecretFile).Trim()
}
if ($studioSecret) {
    if (-not $env:SESSION_SECRET) { $env:SESSION_SECRET = $studioSecret }
    if (-not $env:IMAGE_STUDIO_URL) { $env:IMAGE_STUDIO_URL = 'http://localhost:3002' }
    if (-not $env:IMAGE_STUDIO_IDENTITY_DOMAIN) { $env:IMAGE_STUDIO_IDENTITY_DOMAIN = 'studio.example.com' }
}

$env:PORT = '3000'
$env:SQLITE_PATH = $database
$process = Start-Process -FilePath $executable `
    -WorkingDirectory $projectRoot `
    -WindowStyle Hidden `
    -RedirectStandardOutput $stdoutLog `
    -RedirectStandardError $stderrLog `
    -PassThru
[System.IO.File]::WriteAllText($pidFile, [string]$process.Id, [System.Text.UTF8Encoding]::new($false))

for ($attempt = 0; $attempt -lt 60; $attempt++) {
    if (Test-Server) {
        Start-Process 'http://localhost:3000/'
        exit 0
    }
    if ($process.HasExited) {
        break
    }
    Start-Sleep -Seconds 1
}

Write-Host "B+C local server failed to start. Logs: $stdoutLog and $stderrLog"
exit 1
