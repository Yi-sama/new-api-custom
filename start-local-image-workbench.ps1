$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$binary = Join-Path $root 'bin\new-api-image-workbench-local.exe'
$database = Join-Path $root '.local-tests\abc-shared.db'
$secretFile = Join-Path $root '.local-tests\image-studio-session-secret'
$runtime = Join-Path $root '.local-tests'
$dist = Join-Path (Split-Path -Parent $root) 'ai-image-workbench\dist'
$historyURL = 'http://127.0.0.1:8787'

foreach ($required in @($binary, $database, $secretFile, (Join-Path $dist 'studio.html'))) {
    if (-not (Test-Path -LiteralPath $required -PathType Leaf)) {
        throw "Required local file is missing: $required"
    }
}
if ((Get-NetTCPConnection -LocalPort 3000 -State Listen -ErrorAction SilentlyContinue)) {
    throw 'Port 3000 is in use. Stop only the known local New API process before starting this build.'
}
$health = Invoke-RestMethod -Uri "$historyURL/studio-api/health" -TimeoutSec 3
if (-not $health.ok) { throw 'The image workbench history service is not ready.' }

$env:PORT = '3000'
$env:SQLITE_PATH = $database
$env:TRUSTED_PROXIES = 'none'
$env:SESSION_SECRET = (Get-Content -Raw -LiteralPath $secretFile).Trim()
$env:IMAGE_STUDIO_URL = 'http://localhost:3002'
$env:IMAGE_STUDIO_IDENTITY_DOMAIN = 'studio.example.com'
$env:IMAGE_WORKBENCH_DIST_DIR = $dist
$env:IMAGE_WORKBENCH_HISTORY_URL = $historyURL

$process = Start-Process -FilePath $binary `
    -WorkingDirectory $root `
    -WindowStyle Hidden `
    -RedirectStandardOutput (Join-Path $runtime 'image-workbench.stdout.log') `
    -RedirectStandardError (Join-Path $runtime 'image-workbench.stderr.log') `
    -PassThru
Set-Content -LiteralPath (Join-Path $runtime 'image-workbench.pid') -Value $process.Id

for ($attempt = 0; $attempt -lt 60; $attempt++) {
    if ($process.HasExited) { break }
    try {
        $response = Invoke-WebRequest -Uri 'http://localhost:3000/studio/' -TimeoutSec 2
        if ($response.StatusCode -eq 200 -and $response.Content.Contains('studio-root')) {
            Write-Host "New API with Image Workbench is ready on http://localhost:3000/ (PID $($process.Id))."
            exit 0
        }
    } catch {
        Start-Sleep -Milliseconds 500
    }
}
throw "New API failed to serve the workbench. Check $runtime"
