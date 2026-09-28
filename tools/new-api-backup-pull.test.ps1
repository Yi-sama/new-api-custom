$ErrorActionPreference = 'Stop'
$script = Join-Path $PSScriptRoot 'new-api-backup-pull.ps1'
$wrapper = Join-Path $PSScriptRoot 'new-api-backup-hidden.vbs'
$installer = Join-Path $PSScriptRoot 'install-new-api-backup-task.ps1'
$errors = $null
$tokens = $null
[void][Management.Automation.Language.Parser]::ParseFile($script, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
if (-not (Test-Path -LiteralPath $wrapper)) { throw "Missing hidden launcher: $wrapper" }

$source = Get-Content -LiteralPath $script -Raw
$wrapperSource = Get-Content -LiteralPath $wrapper -Raw
$installerSource = Get-Content -LiteralPath $installer -Raw
foreach ($required in @(
    '156.238.252.115',
    'us-server-ed25519',
    'D:\Server Backups\us-whitelist',
    'newapi.pgdump',
    'sub2api.pgdump',
    'newapi-redis.rdb',
    'sub2api-redis.rdb',
    'pg_restore --list',
    'redis-check-rdb',
    'docker-compose.yml',
    '.partial',
    'trap cleanup_work EXIT',
    'Get-FileHash',
    'rm -f'
)) {
    if (-not $source.Contains($required)) { throw "Missing backup contract: $required" }
}
foreach ($required in @('WindowStyle Hidden', 'shell.Run(command, 0, True)')) {
    if (-not $wrapperSource.Contains($required)) { throw "Missing hidden launcher contract: $required" }
}
if (-not $installerSource.Contains("New-ScheduledTaskAction -Execute 'wscript.exe'")) {
    throw 'Scheduled task must use the hidden wscript launcher'
}

'PASS: syntax and US backup dataset contract'
