param([Parameter(Mandatory = $true)][string]$ReleaseRoot)
$ErrorActionPreference = 'Stop'
$release = [IO.Path]::GetFullPath($ReleaseRoot)
$manifestPath = Join-Path $release 'release-manifest.json'
$manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
$id = $manifest.release
if ($id -notmatch '^tavern-http-[0-9-]+$') { throw 'Invalid release ID.' }
$save = Join-Path $release 'risuai-managed\save'
if (-not (Test-Path -LiteralPath $save)) { $null = New-Item -ItemType Directory -Path $save }
$remote = "/opt/us-whitelist/test-new-api-20261002-174631/releases/$id"
$runtime = '/opt/us-whitelist/test-new-api-20261002-174631/risuai-runtime'
$compose = @{
    services = @{
        'new-api-test' = @{
            environment = @{ VERSION = $manifest.version }
            volumes = @(@{ type='bind'; source="$remote/new-api"; target='/new-api'; read_only=$true })
        }
        'risuai-test' = @{
            image = 'node:24-alpine'
            container_name = 'risuai-test-20261003'
            restart = 'unless-stopped'
            network_mode = 'host'
            user = '1000:1000'
            working_dir = '/app'
            command = @('node', '/app/server.cjs')
            read_only = $true
            mem_limit = '384m'
            pids_limit = 128
            cap_drop = @('ALL')
            security_opt = @('no-new-privileges:true')
            environment = @{
                PORT='32830'
                NEW_API_URL='http://127.0.0.1:32795'
                NEW_API_PORTAL_URL=$manifest.portal
                NEW_API_PUBLIC_URL='http://156.238.252.115:32796'
                NEW_API_ALLOW_PUBLIC_HTTP='1'
                NEW_API_DATA_DIR='/data'
                TRUST_PROXY='loopback'
            }
            volumes = @(
                @{type='bind'; source="$remote/risuai-managed"; target='/app'; read_only=$true},
                @{type='bind'; source="$runtime/data"; target='/data'},
                @{type='bind'; source="$runtime/save"; target='/app/save'}
            )
            tmpfs = @('/tmp:size=32m,mode=1777')
            healthcheck = @{
                test = @('CMD', 'node', '-e', "fetch('http://127.0.0.1:32830/').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))")
                interval='10s'; timeout='5s'; retries=6; start_period='15s'
            }
            logging = @{driver='json-file'; options=@{'max-size'='5m'; 'max-file'='2'}}
        }
    }
}
$utf8 = [Text.UTF8Encoding]::new($false)
[IO.File]::WriteAllText((Join-Path $release 'compose.tavern.json'), ($compose | ConvertTo-Json -Depth 12), $utf8)
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'activate-http.sh') -Destination (Join-Path $release 'activate.sh')
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'rollback-http.sh') -Destination (Join-Path $release 'rollback.sh')
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'nginx-http.conf') -Destination (Join-Path $release 'nginx.tavern.conf')
$manifest | Add-Member -NotePropertyName runtime -NotePropertyValue 'risuai-managed'
$manifest | Add-Member -NotePropertyName bridge_build -NotePropertyValue 'VITE_NEW_API_BRIDGE=1'
[IO.File]::WriteAllText($manifestPath, ($manifest | ConvertTo-Json -Depth 5), $utf8)
$included = @('new-api','risuai-managed','LICENSE.new-api','LICENSE.risuai','release-manifest.json','compose.tavern.json','activate.sh','rollback.sh','nginx.tavern.conf')
$files = foreach ($name in $included) {
    $item = Get-Item -LiteralPath (Join-Path $release $name)
    if ($item.PSIsContainer) { Get-ChildItem -LiteralPath $item.FullName -File -Recurse } else { $item }
}
$checksums = foreach ($file in $files) {
    $relative = $file.FullName.Substring($release.Length + 1).Replace('\','/')
    "$((Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant())  $relative"
}
[IO.File]::WriteAllText((Join-Path $release 'SHA256SUMS'), ($checksums -join "`n") + "`n", $utf8)
$archive = Join-Path (Split-Path $release -Parent) "$id.tgz"
& tar -czf $archive -C $release @included SHA256SUMS
if ($LASTEXITCODE -ne 0) { throw 'Release archive failed.' }
Get-FileHash -LiteralPath $archive -Algorithm SHA256
Write-Output "FILES=$($files.Count) ARCHIVE=$archive"
