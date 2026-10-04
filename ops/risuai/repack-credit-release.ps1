param(
    [Parameter(Mandatory = $true)][string]$ReleaseId,
    [Parameter(Mandatory = $true)][string]$SourceReleaseId
)
$ErrorActionPreference = 'Stop'
if ($ReleaseId -notmatch '^tavern-credit-[0-9-]+$' -or $SourceReleaseId -notmatch '^tavern-credit-[0-9-]+$') {
    throw 'Invalid release ID.'
}
$buildRoot = 'D:\Open source project\.local-builds'
$source = Join-Path $buildRoot "$SourceReleaseId\release"
$build = Join-Path $buildRoot $ReleaseId
$release = Join-Path $build 'release'
if (Test-Path -LiteralPath $build) { throw 'Use a new version directory.' }
if (!(Test-Path -LiteralPath (Join-Path $source 'SHA256SUMS'))) { throw 'Source manifest is missing.' }
foreach ($line in (Get-Content -LiteralPath (Join-Path $source 'SHA256SUMS'))) {
    if ($line -notmatch '^([a-f0-9]{64})  (.+)$') { throw 'Invalid source checksum entry.' }
    $expected = $Matches[1]
    $file = Join-Path $source $Matches[2]
    if ((Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expected) {
        throw "Source checksum mismatch: $file"
    }
}
$null = [IO.Directory]::CreateDirectory($release)
foreach ($file in (Get-ChildItem -LiteralPath $source)) {
    Copy-Item -LiteralPath $file.FullName -Destination $release -Recurse
}
$utf8 = [Text.UTF8Encoding]::new($false)
$composeFile = Join-Path $release 'compose.credit.json'
$compose = Get-Content -LiteralPath $composeFile -Raw -Encoding UTF8 | ConvertFrom-Json
$oldRemote = "/opt/us-whitelist/test-new-api-20261002-174631/releases/$SourceReleaseId"
$remote = "/opt/us-whitelist/test-new-api-20261002-174631/releases/$ReleaseId"
foreach ($name in @('new-api-test', 'risuai-test')) {
    foreach ($volume in $compose.services.$name.volumes) {
        if ($volume.source.StartsWith("$oldRemote/")) {
            $volume.source = $remote + $volume.source.Substring($oldRemote.Length)
        }
    }
}
$version = "v1.0.0-rc.28-custom-$ReleaseId"
$compose.services.'new-api-test'.environment.VERSION = $version
[IO.File]::WriteAllText($composeFile, ($compose | ConvertTo-Json -Depth 8), $utf8)
foreach ($pair in @(
    @('activate-credit.sh', 'activate.sh'), @('rollback-credit.sh', 'rollback.sh'),
    @('credit-schema.pg.sql', 'credit-schema.pg.sql')
)) {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot $pair[0]) -Destination (Join-Path $release $pair[1])
}
$manifestFile = Join-Path $release 'release-manifest.json'
$manifest = Get-Content -LiteralPath $manifestFile -Raw -Encoding UTF8 | ConvertFrom-Json
$manifest.release = $ReleaseId
$manifest.version = $version
$manifest | Add-Member -NotePropertyName reused_artifacts_from -NotePropertyValue $SourceReleaseId
$manifest | Add-Member -NotePropertyName deployment_fix -NotePropertyValue 'Gift schema ownership verified as the application database role; retained service key reused.'
[IO.File]::WriteAllText($manifestFile, ($manifest | ConvertTo-Json -Depth 5), $utf8)
$sums = foreach ($file in (Get-ChildItem -LiteralPath $release -File -Recurse | Sort-Object FullName)) {
    $relative = [IO.Path]::GetRelativePath($release, $file.FullName).Replace('\', '/')
    if ($relative -eq 'SHA256SUMS') { continue }
    "$((Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant())  $relative"
}
[IO.File]::WriteAllText((Join-Path $release 'SHA256SUMS'), ([string]::Join("`n", $sums) + "`n"), $utf8)
& tar -czf (Join-Path $build "$ReleaseId.tgz") -C $release .
if ($LASTEXITCODE -ne 0) { throw 'Release archive failed.' }
Write-Output "RELEASE=$release"
