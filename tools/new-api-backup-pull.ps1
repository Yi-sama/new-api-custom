[CmdletBinding()]
param(
    [string]$RemoteHost = '156.238.252.115',
    [string]$RemoteUser = 'root',
    [string]$IdentityFile = "$HOME\.ssh\us-server-ed25519",
    [string]$BackupRoot = 'D:\Server Backups\us-whitelist',
    [int]$RetentionDays = 30
)

$ErrorActionPreference = 'Stop'
New-Item -ItemType Directory -Force -Path $BackupRoot | Out-Null
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$name = "us-whitelist-$stamp.tar.gz"
$remoteDir = "/opt/us-whitelist/backups/client-$stamp"
$remoteArchive = "/opt/us-whitelist/backups/$name"
$localArchive = Join-Path $BackupRoot $name
$localSha = "$localArchive.sha256"
$partialArchive = "$localArchive.partial"
$partialSha = "$localSha.partial"

function Write-BackupLog([string]$Message) {
    $line = "{0:u} {1}" -f (Get-Date), $Message
    Add-Content -LiteralPath (Join-Path $BackupRoot 'pull.log') -Value $line -Encoding UTF8
    Write-Output $line
}

function Invoke-Ssh([string]$Command) {
    $args = @('-p', '22', '-l', $RemoteUser, '-o', 'IdentitiesOnly=yes', '-o', 'BatchMode=yes',
        '-o', 'ConnectTimeout=15', '-o', 'PasswordAuthentication=no', '-o', 'KbdInteractiveAuthentication=no',
        '-i', $IdentityFile, $RemoteHost, $Command)
    $output = & ssh.exe @args 2>&1
    if ($LASTEXITCODE -ne 0) { throw "ssh failed ($LASTEXITCODE): $($output -join ' ')" }
    return @($output)
}

function Invoke-Scp([string]$RemotePath, [string]$LocalPath) {
    $target = "$RemoteUser@$RemoteHost`:$RemotePath"
    & scp.exe -q -P 22 -o IdentitiesOnly=yes -o BatchMode=yes -o ConnectTimeout=15 -o PasswordAuthentication=no -o KbdInteractiveAuthentication=no -i $IdentityFile $target $LocalPath
    if ($LASTEXITCODE -ne 0) { throw "scp failed ($LASTEXITCODE): $RemotePath" }
}

Write-BackupLog "starting US server backup $stamp"
$prepare = @"
set -e
mkdir -p '$remoteDir'
cleanup_work() {
  rm -rf '$remoteDir'
  rm -f /opt/us-whitelist/data/redis-newapi/client-$stamp-newapi.rdb
  rm -f /opt/us-whitelist/data/redis-sub2api/client-$stamp-sub2api.rdb
  rm -f '$remoteArchive.partial'
}
trap cleanup_work EXIT
docker exec sub2api-postgres sh -lc 'pg_dump -U "`$POSTGRES_USER" -d newapi -Fc' > '$remoteDir/newapi.pgdump'
docker exec sub2api-postgres sh -lc 'pg_dump -U "`$POSTGRES_USER" -d sub2api -Fc' > '$remoteDir/sub2api.pgdump'
docker exec -i sub2api-postgres pg_restore --list < '$remoteDir/newapi.pgdump' >/dev/null
docker exec -i sub2api-postgres pg_restore --list < '$remoteDir/sub2api.pgdump' >/dev/null
docker exec redis redis-cli --rdb /data/client-$stamp-newapi.rdb >/dev/null 2>&1
docker exec sub2api-redis redis-cli --rdb /data/client-$stamp-sub2api.rdb >/dev/null 2>&1
docker exec redis redis-check-rdb /data/client-$stamp-newapi.rdb >/dev/null
docker exec sub2api-redis redis-check-rdb /data/client-$stamp-sub2api.rdb >/dev/null
cp /opt/us-whitelist/data/redis-newapi/client-$stamp-newapi.rdb '$remoteDir/newapi-redis.rdb'
cp /opt/us-whitelist/data/redis-sub2api/client-$stamp-sub2api.rdb '$remoteDir/sub2api-redis.rdb'
cp /opt/us-whitelist/docker-compose.yml '$remoteDir/docker-compose.yml'
cp -a /opt/us-whitelist/env '$remoteDir/env'
mkdir -p '$remoteDir/sub2api-data'
cp -a /opt/sub2api/data/.installed /opt/sub2api/data/config.yaml /opt/sub2api/data/model_pricing.json /opt/sub2api/data/model_pricing.sha256 '$remoteDir/sub2api-data/'
tar -czf '$remoteArchive.partial' -C /opt/us-whitelist/backups 'client-$stamp'
mv '$remoteArchive.partial' '$remoteArchive'
sha256sum '$remoteArchive' > '$remoteArchive.sha256'
printf '%s\n' '$remoteArchive'
"@
try {
    Invoke-Ssh $prepare | Out-Null
    Invoke-Scp $remoteArchive $partialArchive
    Invoke-Scp "$remoteArchive.sha256" $partialSha
    $expected = (Get-Content -LiteralPath $partialSha -Raw -Encoding UTF8).Trim().Split(' ')[0].ToLowerInvariant()
    $actual = (Get-FileHash -LiteralPath $partialArchive -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($expected -ne $actual) { throw "SHA-256 mismatch for $partialArchive" }
    Move-Item -LiteralPath $partialArchive -Destination $localArchive -Force
    Move-Item -LiteralPath $partialSha -Destination $localSha -Force
} finally {
    Invoke-Ssh "rm -f '$remoteArchive' '$remoteArchive.sha256'" | Out-Null
    Remove-Item -LiteralPath $partialArchive, $partialSha -Force -ErrorAction SilentlyContinue
}
Write-BackupLog "accepted $name"

$cutoff = (Get-Date).AddDays(-$RetentionDays)
Get-ChildItem -LiteralPath $BackupRoot -File -Filter 'us-whitelist-*.tar.gz' |
    Where-Object LastWriteTime -lt $cutoff | ForEach-Object {
        Remove-Item -LiteralPath $_.FullName -Force
        Remove-Item -LiteralPath "$($_.FullName).sha256" -Force -ErrorAction SilentlyContinue
    }
Write-BackupLog 'US server backup completed'
