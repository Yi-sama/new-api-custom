param(
    [Parameter(Mandatory = $true)][string]$ReleaseId,
    [string]$Baseline = 'D:\Open source project\.local-builds\tavern-http-20261003-1708\source\risuai',
    [string[]]$UpdatedFiles = @('src\ts\newApiBridge.ts', 'src\ts\newApiBridge.portal.test.ts')
)
$ErrorActionPreference = 'Stop'
if ($ReleaseId -notmatch '^risuai-entry-[0-9-]+$') { throw 'Invalid release ID.' }
$build = Join-Path 'D:\Open source project\.local-builds' $ReleaseId
$risu = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..\risuai'))
if (Test-Path -LiteralPath $build) { throw 'Use a new version directory.' }
$source = Join-Path $build 'source'
$release = Join-Path $build 'release'
$null = [IO.Directory]::CreateDirectory($source)
$null = [IO.Directory]::CreateDirectory($release)
foreach ($item in (Get-ChildItem -LiteralPath $Baseline -Force)) {
    if ($item.Name -in @('node_modules', 'dist')) { continue }
    Copy-Item -LiteralPath $item.FullName -Destination $source -Recurse
}
foreach ($file in $UpdatedFiles) {
    Copy-Item -LiteralPath (Join-Path $risu $file) -Destination (Join-Path $source $file)
}
$null = New-Item -ItemType Junction -Path (Join-Path $source 'node_modules') -Target (Join-Path $risu 'node_modules')
$oldEnv = @{}
foreach ($name in @('VITE_NEW_API_BRIDGE', 'VITE_NEW_API_PORTAL_URL', 'VITE_RISU_LEGAL_CONFIGURED')) {
    $oldEnv[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
try {
    $env:VITE_NEW_API_BRIDGE = '1'
    $env:VITE_NEW_API_PORTAL_URL = 'http://156.238.252.115:32795'
    $env:VITE_RISU_LEGAL_CONFIGURED = 'TRUE'
    Push-Location $source
    try {
        & 'D:\Node.js\node.exe' 'node_modules\vite\bin\vite.js' build --minify false --outDir (Join-Path $release 'dist')
        if ($LASTEXITCODE -ne 0) { throw 'Vite build failed.' }
    } finally { Pop-Location }
} finally {
    foreach ($name in $oldEnv.Keys) {
        [Environment]::SetEnvironmentVariable($name, $oldEnv[$name], 'Process')
    }
}
$remote = "/opt/us-whitelist/test-new-api-20261002-174631/releases/$ReleaseId"
$compose = @{
    services = @{
        'risuai-test' = @{
            volumes = @(@{ type='bind'; source="$remote/dist"; target='/app/dist'; read_only=$true })
        }
    }
}
[IO.File]::WriteAllText((Join-Path $release 'compose.entry.json'), ($compose | ConvertTo-Json -Depth 8), [Text.UTF8Encoding]::new($false))
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'activate-entry-frontend.sh') -Destination (Join-Path $release 'activate.sh')
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'rollback-entry-frontend.sh') -Destination (Join-Path $release 'rollback.sh')
Copy-Item -LiteralPath (Join-Path $source 'LICENSE') -Destination (Join-Path $release 'LICENSE.risuai')
$manifest = @{ release = $ReleaseId; source_baseline = $Baseline; updated_files = $UpdatedFiles; paid_calls = 0 }
[IO.File]::WriteAllText((Join-Path $release 'release-manifest.json'), ($manifest | ConvertTo-Json -Depth 4), [Text.UTF8Encoding]::new($false))
$sums = foreach ($file in (Get-ChildItem -LiteralPath $release -File -Recurse | Sort-Object FullName)) {
    $relative = [IO.Path]::GetRelativePath($release, $file.FullName).Replace('\', '/')
    "$((Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant())  $relative"
}
[IO.File]::WriteAllText((Join-Path $release 'SHA256SUMS'), ([string]::Join("`n", $sums) + "`n"), [Text.UTF8Encoding]::new($false))
& tar -czf (Join-Path $build "$ReleaseId.tgz") -C $release .
if ($LASTEXITCODE -ne 0) { throw 'Frontend release archive failed.' }
Write-Output "RELEASE=$release"
