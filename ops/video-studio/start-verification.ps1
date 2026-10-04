param(
    [int]$Port = 3117,
    [string]$RuntimeRoot = 'G:\new-api-video-runtime',
    [string]$CanvasDist = 'G:\infinite-canvas-video-runtime\dist'
)

$ErrorActionPreference = 'Stop'
if (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) {
    throw "Verification port $Port is in use."
}
$fixtureRoot = Join-Path $RuntimeRoot 'verification'
New-Item -ItemType Directory -Path $fixtureRoot -Force | Out-Null
foreach ($name in @('SQL_DSN', 'LOG_SQL_DSN', 'REDIS_CONN_STRING', 'CHANNEL_UPDATE_FREQUENCY', 'FRONTEND_BASE_URL', 'SUB2API_BASE_URL', 'SUB2API_ADMIN_API_KEY')) {
    Remove-Item -LiteralPath "Env:$name" -ErrorAction SilentlyContinue
}
$env:SQLITE_PATH = Join-Path $fixtureRoot 'verification.db'
$env:PORT = [string]$Port
$env:BIND_ADDRESS = '127.0.0.1'
$env:NODE_TYPE = 'master'
$env:TRUSTED_PROXIES = 'none'
$env:UPDATE_TASK = 'false'
$env:ENABLE_PPROF = 'false'
$env:CHANNEL_UPSTREAM_MODEL_UPDATE_TASK_ENABLED = 'false'
$env:SESSION_SECRET = 'video-studio-isolated-fixture-not-for-production'
$env:VIDEO_STUDIO_DIST_DIR = (Resolve-Path -LiteralPath $CanvasDist).Path
$env:VIDEO_STUDIO_DATA_DIR = Join-Path $fixtureRoot 'studio-data'
$process = Start-Process -FilePath (Join-Path $RuntimeRoot 'new-api-video.exe') `
    -WorkingDirectory $fixtureRoot -WindowStyle Hidden -PassThru `
    -RedirectStandardOutput (Join-Path $fixtureRoot 'server.stdout.log') `
    -RedirectStandardError (Join-Path $fixtureRoot 'server.stderr.log')
[IO.File]::WriteAllText((Join-Path $fixtureRoot 'server.pid'), [string]$process.Id)
$base = "http://127.0.0.1:$Port"
$ready = $false
for ($attempt = 0; $attempt -lt 40; $attempt++) {
    if ($process.HasExited) { throw 'Verification server exited; inspect fixture logs.' }
    try {
        $ready = (Invoke-RestMethod "$base/api/status" -TimeoutSec 2).success
        if ($ready) { break }
    } catch [System.Net.Http.HttpRequestException] {}
    catch [System.Net.WebException] {}
    Start-Sleep 1
}
if (-not $ready) { throw 'Verification server did not become ready.' }

# Synthetic credentials only. This database has no copied accounts or real provider keys.
$credentials = @{ username = 'videotest'; password = 'VideoStudio-Local-Only-2026!' }
if (-not (Invoke-RestMethod "$base/api/setup").data.status) {
    $setup = $credentials.Clone()
    $setup.confirmPassword = $credentials.password
    $setup.SelfUseModeEnabled = $true
    $result = Invoke-RestMethod "$base/api/setup" -Method Post -ContentType 'application/json' -Body ($setup | ConvertTo-Json)
    if (-not $result.success) { throw "Fixture setup failed: $($result.message)" }
}
$login = Invoke-RestMethod "$base/api/user/login" -Method Post -ContentType 'application/json' -Body ($credentials | ConvertTo-Json)
if (-not $login.success -or -not $login.data.access_token) { throw 'Fixture login failed.' }
$headers = @{ Authorization = "Bearer $($login.data.access_token)" }
$channels = Invoke-RestMethod "$base/api/channel/?p=1&page_size=100" -Headers $headers
if (-not ($channels.data.items | Where-Object name -eq 'Local video verification')) {
    $body = @{
        mode = 'single'
        channel = @{
            name = 'Local video verification'
            type = 1
            status = 1
            key = 'local-verification-not-a-provider-key'
            base_url = 'http://127.0.0.1:3108'
            models = 'sora-2'
            group = 'default'
        }
    } | ConvertTo-Json -Depth 5
    $result = Invoke-RestMethod "$base/api/channel/" -Method Post -Headers $headers -ContentType 'application/json' -Body $body
    if (-not $result.success) { throw "Fixture channel setup failed: $($result.message)" }
}
$metadata = Invoke-RestMethod "$base/api/models/?p=1&page_size=100" -Headers $headers
if (-not ($metadata.data.items | Where-Object model_name -eq 'sora-2')) {
    $body = @{
        model_name = 'sora-2'
        status = 1
        sync_official = 0
        name_rule = 0
        endpoints = '{"openai-video":{"path":"/v1/videos","method":"POST"}}'
    } | ConvertTo-Json
    $result = Invoke-RestMethod "$base/api/models/" -Method Post -Headers $headers -ContentType 'application/json' -Body $body
    if (-not $result.success) { throw "Fixture model metadata setup failed: $($result.message)" }
}
$session = Invoke-RestMethod "$base/api/video-studio/session" -Method Post -Headers $headers
if (-not $session.success -or -not $session.data.token) { throw 'Fixture Video Studio session failed.' }
$models = Invoke-RestMethod "$base/api/video-studio/v1/models" -Headers @{ Authorization = "Bearer $($session.data.token)" }
if (-not ($models.data | Where-Object id -eq 'sora-2')) { throw 'Fixture video model catalog is missing.' }
Write-Output "Verification ready at http://localhost:$Port/video-studio (PID $($process.Id))"
Write-Output 'Synthetic account: videotest. No generation request was submitted.'
