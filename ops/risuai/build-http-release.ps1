param(
    [Parameter(Mandatory = $true)][string]$ReleaseId,
    [string]$OutputRoot = 'D:\Open source project\.local-builds',
    [string]$PublicHost = '156.238.252.115'
)

$ErrorActionPreference = 'Stop'
$newApi = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$risu = [IO.Path]::GetFullPath((Join-Path $newApi '..\risuai'))
$root = Join-Path $OutputRoot $ReleaseId
if ($ReleaseId -notmatch '^tavern-http-[0-9-]+$' -or (Test-Path -LiteralPath $root)) {
    throw 'Use a new timestamped tavern-http release directory.'
}
$null = New-Item -ItemType Directory -Path $root

function Copy-SourceSnapshot([string]$Source, [string]$Destination) {
    $null = New-Item -ItemType Directory -Path $Destination
    $files = & git --no-optional-locks -C $Source -c core.quotepath=false ls-files --cached --others --exclude-standard
    if ($LASTEXITCODE -ne 0) { throw 'Cannot enumerate source snapshot.' }
    foreach ($relative in ($files | Sort-Object -Unique)) {
        if ($relative -match '^(docs|output|bin|dist|web/dist)/|(^|/)node_modules/|\.tsbuildinfo$') { continue }
        $from = Join-Path $Source $relative
        if (-not (Test-Path -LiteralPath $from -PathType Leaf)) { continue }
        $to = Join-Path $Destination $relative
        $null = [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($to))
        Copy-Item -LiteralPath $from -Destination $to
    }
}

function Run-Checked([string]$Executable, [string[]]$Arguments) {
    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Executable failed ($LASTEXITCODE)." }
}

$newSource = Join-Path $root 'source\new-api'
$risuSource = Join-Path $root 'source\risuai'
Copy-SourceSnapshot $newApi $newSource
Copy-SourceSnapshot $risu $risuSource
$null = New-Item -ItemType Junction -Path (Join-Path $newSource 'web\node_modules') -Target (Join-Path $newApi 'web\node_modules')
$null = New-Item -ItemType Junction -Path (Join-Path $risuSource 'node_modules') -Target (Join-Path $risu 'node_modules')

$release = Join-Path $root 'release'
$runtime = Join-Path $release 'risuai-managed'
$null = New-Item -ItemType Directory -Path $runtime
$version = "v1.0.0-rc.28-custom-tavern-http-$ReleaseId"
$oldEnv = @{}
foreach ($name in @('VITE_RISUAI_URL', 'VITE_REACT_APP_VERSION', 'VITE_NEW_API_BRIDGE', 'VITE_NEW_API_PORTAL_URL', 'VITE_RISU_LEGAL_CONFIGURED', 'GOOS', 'GOARCH', 'CGO_ENABLED', 'GOWORK', 'GOMAXPROCS')) {
    $oldEnv[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
try {
    $env:VITE_RISUAI_URL = "http://${PublicHost}:32796/?connect=1"
    $env:VITE_REACT_APP_VERSION = $version
    Push-Location (Join-Path $newSource 'web')
    try {
        Run-Checked 'D:\Node.js\node.exe' @('node_modules\@rsbuild\core\bin\rsbuild.js', 'build')
    } finally { Pop-Location }

    # This release is an authenticated private test instance, not a public RisuAI service.
    $env:VITE_NEW_API_BRIDGE = '1'
    $env:VITE_NEW_API_PORTAL_URL = "http://${PublicHost}:32795"
    $env:VITE_RISU_LEGAL_CONFIGURED = 'TRUE'
    Push-Location $risuSource
    try {
        Run-Checked 'D:\Node.js\node.exe' @('node_modules\vite\bin\vite.js', 'build', '--minify', 'false', '--outDir', (Join-Path $runtime 'dist'))
        Run-Checked 'D:\Node.js\node.exe' @((Join-Path $PSScriptRoot 'bundle-runtime.cjs'), $risuSource, $runtime)
    } finally { Pop-Location }

    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'
    $env:GOWORK = 'off'
    $env:GOMAXPROCS = '2'
    Push-Location $newSource
    try {
        Run-Checked 'D:\Go\bin\go.exe' @('build', '-p', '2', '-trimpath', '-ldflags', "-s -w -X github.com/QuantumNous/new-api/common.Version=$version", '-o', (Join-Path $release 'new-api'), '.')
    } finally { Pop-Location }
} finally {
    foreach ($name in $oldEnv.Keys) { [Environment]::SetEnvironmentVariable($name, $oldEnv[$name], 'Process') }
}
Copy-Item -LiteralPath (Join-Path $newApi 'LICENSE') -Destination (Join-Path $release 'LICENSE.new-api')
Copy-Item -LiteralPath (Join-Path $risu 'LICENSE') -Destination (Join-Path $release 'LICENSE.risuai')
$manifest = [ordered]@{
    release = $ReleaseId
    version = $version
    new_api_head = (& git --no-optional-locks -C $newApi rev-parse HEAD)
    risuai_head = (& git --no-optional-locks -C $risu rev-parse HEAD)
    source = 'Current dirty worktrees; isolated snapshots retained alongside release'
    portal = "http://${PublicHost}:32795"
    tavern = "http://${PublicHost}:32796/?connect=1"
    paid_calls = 'Not authorized'
}
[IO.File]::WriteAllText((Join-Path $release 'release-manifest.json'), ($manifest | ConvertTo-Json), [Text.UTF8Encoding]::new($false))
Write-Output "Release built: $release"
