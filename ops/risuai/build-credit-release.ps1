param([Parameter(Mandatory = $true)][string]$ReleaseId)
$ErrorActionPreference = 'Stop'
if ($ReleaseId -notmatch '^tavern-credit-[0-9-]+$') { throw 'Invalid release ID.' }
$build = Join-Path 'D:\Open source project\.local-builds' $ReleaseId
if (Test-Path -LiteralPath $build) { throw 'Use a new version directory.' }
$newApi = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$risu = [IO.Path]::GetFullPath((Join-Path $newApi '..\risuai'))
$newSource = Join-Path $build 'source\new-api'
$risuSource = Join-Path $build 'source\risuai'
$release = Join-Path $build 'release'
$null = [IO.Directory]::CreateDirectory($release)
& robocopy 'D:\Open source project\.local-builds\tavern-credit-20261003-2028\source\new-api' $newSource /E /XJ /XD node_modules .git /NFL /NDL /NJH /NJS /NP
if ($LASTEXITCODE -ge 8) { throw 'New API snapshot copy failed.' }
& robocopy 'D:\Open source project\.local-builds\tavern-credit-20261003-2028\source\risuai' $risuSource /E /XJ /XD node_modules dist .git /NFL /NDL /NJH /NJS /NP
if ($LASTEXITCODE -ge 8) { throw 'RisuAI snapshot copy failed.' }
$backendFiles = @(
    'model/tavern_credit.go', 'model/main.go', 'controller/tavern.go', 'controller/tavern_test.go',
    'middleware/tavern.go', 'middleware/tavern_test.go', 'router/relay-router.go',
    'service/tavern_funding.go', 'service/tavern_funding_test.go', 'service/billing_session.go',
    'service/billing.go', 'service/log_info_generate.go', 'relay/common/relay_info.go'
)
$frontendFiles = @(
    'src/lang/cn.ts', 'src/lang/en.ts', 'src/lib/Others/WelcomeRisu.svelte',
    'src/lib/Setting/Pages/UserSettings.svelte', 'src/ts/newApiBridge.ts',
    'src/ts/newApiBridge.test.ts', 'server/node/new-api.cjs', 'server/node/new-api.test.cjs'
)
# The validated candidate is frozen so concurrent worktree edits cannot enter this release.
$null = New-Item -ItemType Junction -Path (Join-Path $risuSource 'node_modules') -Target (Join-Path $risu 'node_modules')
$version = "v1.0.0-rc.28-custom-$ReleaseId"
$oldEnv = @{}
foreach ($name in @('VITE_NEW_API_BRIDGE', 'VITE_NEW_API_PORTAL_URL', 'VITE_RISU_LEGAL_CONFIGURED', 'GOOS', 'GOARCH', 'CGO_ENABLED', 'GOWORK', 'GOMAXPROCS')) {
    $oldEnv[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
try {
    Push-Location $newSource
    try {
        $env:GOWORK = 'off'
        & go test ./controller ./service ./middleware -run Tavern -count=1 -timeout=60s
        if ($LASTEXITCODE -ne 0) { throw 'Snapshot Tavern regression failed.' }
        $env:GOOS = 'linux'
        $env:GOARCH = 'amd64'
        $env:CGO_ENABLED = '0'
        $env:GOMAXPROCS = '2'
        & go build -p 2 -trimpath -ldflags "-s -w -X github.com/QuantumNous/new-api/common.Version=$version" -o (Join-Path $release 'new-api') .
        if ($LASTEXITCODE -ne 0) { throw 'Linux build failed.' }
    } finally { Pop-Location }
    $env:VITE_NEW_API_BRIDGE = '1'
    $env:VITE_NEW_API_PORTAL_URL = 'http://156.238.252.115:32795'
    $env:VITE_RISU_LEGAL_CONFIGURED = 'TRUE'
    Push-Location $risuSource
    try {
        & node node_modules/vite/bin/vite.js build --minify false --outDir (Join-Path $release 'dist')
        if ($LASTEXITCODE -ne 0) { throw 'Vite build failed.' }
        & node (Join-Path $PSScriptRoot 'bundle-runtime.cjs') $risuSource (Join-Path $release 'runtime')
        if ($LASTEXITCODE -ne 0) { throw 'Runtime bundle failed.' }
    } finally { Pop-Location }
} finally {
    foreach ($name in $oldEnv.Keys) { [Environment]::SetEnvironmentVariable($name, $oldEnv[$name], 'Process') }
}
$remote = "/opt/us-whitelist/test-new-api-20261002-174631/releases/$ReleaseId"
$key = '/opt/us-whitelist/test-new-api-20261002-174631/tavern-credit-runtime/service.key'
$compose = @{
    services = @{
        'new-api-test' = @{
            environment = @{ VERSION = $version; TAVERN_SERVICE_KEY_FILE = '/run/tavern-service-key' }
            volumes = @(
                @{ type='bind'; source="$remote/new-api"; target='/new-api'; read_only=$true },
                @{ type='bind'; source=$key; target='/run/tavern-service-key'; read_only=$true }
            )
        }
        'risuai-test' = @{
            environment = @{ TAVERN_SERVICE_KEY_FILE = '/run/tavern-service-key' }
            volumes = @(
                @{ type='bind'; source="$remote/dist"; target='/app/dist'; read_only=$true },
                @{ type='bind'; source="$remote/runtime/server.cjs"; target='/app/server.cjs'; read_only=$true },
                @{ type='bind'; source="$remote/runtime/server.cjs.LEGAL.txt"; target='/app/server.cjs.LEGAL.txt'; read_only=$true },
                @{ type='bind'; source=$key; target='/run/tavern-service-key'; read_only=$true }
            )
        }
    }
}
[IO.File]::WriteAllText((Join-Path $release 'compose.credit.json'), ($compose | ConvertTo-Json -Depth 8), [Text.UTF8Encoding]::new($false))
foreach ($pair in @(
    @('activate-credit.sh','activate.sh'), @('rollback-credit.sh','rollback.sh'),
    @('credit-schema.pg.sql','credit-schema.pg.sql')
)) {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot $pair[0]) -Destination (Join-Path $release $pair[1])
}
Copy-Item -LiteralPath (Join-Path $newSource 'LICENSE') -Destination (Join-Path $release 'LICENSE.new-api')
Copy-Item -LiteralPath (Join-Path $risuSource 'LICENSE') -Destination (Join-Path $release 'LICENSE.risuai')
$manifest = @{
    release = $ReleaseId; version = $version
    backend_baseline = 'tavern-http-20261003-1708'
    frontend_baseline = 'risuai-entry-20261003-1850'
    validated_source_snapshot = 'tavern-credit-20261003-2028'
    updated_backend_files = $backendFiles; updated_frontend_files = $frontendFiles
    paid_calls = 0
}
[IO.File]::WriteAllText((Join-Path $release 'release-manifest.json'), ($manifest | ConvertTo-Json -Depth 4), [Text.UTF8Encoding]::new($false))
$sums = foreach ($file in (Get-ChildItem -LiteralPath $release -File -Recurse | Sort-Object FullName)) {
    $relative = [IO.Path]::GetRelativePath($release, $file.FullName).Replace('\', '/')
    "$((Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant())  $relative"
}
[IO.File]::WriteAllText((Join-Path $release 'SHA256SUMS'), ([string]::Join("`n", $sums) + "`n"), [Text.UTF8Encoding]::new($false))
& tar -czf (Join-Path $build "$ReleaseId.tgz") -C $release .
if ($LASTEXITCODE -ne 0) { throw 'Release archive failed.' }
Write-Output "RELEASE=$release"
