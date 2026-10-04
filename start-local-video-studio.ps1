param(
    [int]$Port = 3000,
    [string]$RuntimeRoot = 'G:\new-api-video-runtime',
    [string]$CanvasDist = 'G:\infinite-canvas-video-runtime\dist'
)

$ErrorActionPreference = 'Stop'
$projectRoot = $PSScriptRoot
$executable = Join-Path $RuntimeRoot 'new-api-video.exe'
$database = Join-Path $RuntimeRoot 'preview.db'
$sourceDatabase = Join-Path $projectRoot '.local-tests\abc-shared.db'

if (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) {
    throw "Port $Port is in use. Pass -Port with another local port."
}
if (-not (Test-Path -LiteralPath $executable -PathType Leaf)) {
    throw "Build the local executable first: $executable"
}
if (-not (Test-Path -LiteralPath (Join-Path $CanvasDist 'index.html') -PathType Leaf)) {
    throw "Build the managed Infinite Canvas first: $CanvasDist"
}
New-Item -ItemType Directory -Path $RuntimeRoot -Force | Out-Null
if (-not (Test-Path -LiteralPath $database)) {
    if (-not (Test-Path -LiteralPath $sourceDatabase -PathType Leaf)) {
        throw 'The existing local test database is missing; refusing to create an empty replacement.'
    }
    @'
import pathlib, sqlite3, sys
source = sqlite3.connect(pathlib.Path(sys.argv[1]).as_uri() + "?mode=ro", uri=True)
destination = sqlite3.connect(sys.argv[2])
source.backup(destination)
assert destination.execute("PRAGMA integrity_check").fetchone()[0] == "ok"
destination.close()
source.close()
'@ | python - $sourceDatabase $database
    if ($LASTEXITCODE -ne 0) { throw 'Local test database snapshot failed.' }
}

foreach ($name in @('SQL_DSN', 'LOG_SQL_DSN', 'REDIS_CONN_STRING', 'CHANNEL_UPDATE_FREQUENCY', 'FRONTEND_BASE_URL', 'SUB2API_BASE_URL', 'SUB2API_ADMIN_API_KEY')) {
    Remove-Item -LiteralPath "Env:$name" -ErrorAction SilentlyContinue
}
$env:BIND_ADDRESS = '127.0.0.1'
$env:PORT = [string]$Port
$env:SQLITE_PATH = $database
# Preview the copied accounts without refreshing real provider credentials or scheduling upstream calls.
$env:NODE_TYPE = 'slave'
$env:UPDATE_TASK = 'false'
$env:ENABLE_PPROF = 'false'
$env:TRUSTED_PROXIES = 'none'
$env:CHANNEL_UPSTREAM_MODEL_UPDATE_TASK_ENABLED = 'false'
$env:VIDEO_STUDIO_DIST_DIR = (Resolve-Path -LiteralPath $CanvasDist).Path
$env:VIDEO_STUDIO_DATA_DIR = Join-Path $RuntimeRoot 'studio-data'
$studioSecret = Join-Path $projectRoot '.local-tests\image-studio-session-secret'
if (Test-Path -LiteralPath $studioSecret -PathType Leaf) {
    $env:SESSION_SECRET = (Get-Content -Raw -LiteralPath $studioSecret).Trim()
    $env:IMAGE_STUDIO_URL = 'http://localhost:3002'
    $env:IMAGE_STUDIO_IDENTITY_DOMAIN = 'studio.example.com'
}
$process = Start-Process -FilePath $executable -WorkingDirectory $RuntimeRoot -WindowStyle Hidden `
    -RedirectStandardOutput (Join-Path $RuntimeRoot "server-$Port.stdout.log") `
    -RedirectStandardError (Join-Path $RuntimeRoot "server-$Port.stderr.log") -PassThru
[IO.File]::WriteAllText((Join-Path $RuntimeRoot "server-$Port.pid"), [string]$process.Id)

for ($attempt = 0; $attempt -lt 40; $attempt++) {
    if ($process.HasExited) { throw "Local server exited. Inspect logs in $RuntimeRoot" }
    try {
        $status = Invoke-RestMethod -Uri "http://127.0.0.1:$Port/api/status" -TimeoutSec 2
        if ($status.success) {
            Write-Output "Video Studio: http://localhost:$Port/video-studio"
            Write-Output "PID: $($process.Id)"
            Write-Output "Local snapshot: $database"
            exit 0
        }
    } catch [System.Net.Http.HttpRequestException] {}
    catch [System.Net.WebException] {}
    Start-Sleep -Seconds 1
}
throw "Local server did not become ready. Inspect logs in $RuntimeRoot"
