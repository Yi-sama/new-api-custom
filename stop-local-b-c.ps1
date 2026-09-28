$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$executable = Join-Path $projectRoot 'bin\new-api-local-b-c.exe'
$pidFile = Join-Path $projectRoot '.local-tests\b-c-shared.pid'

if (-not (Test-Path -LiteralPath $pidFile -PathType Leaf)) {
    Write-Host 'B+C local server is not running.'
    exit 0
}

$serverPid = [int](Get-Content -Raw -LiteralPath $pidFile)
$process = Get-Process -Id $serverPid -ErrorAction SilentlyContinue
if ($process) {
    if ($process.Path -ne $executable) {
        Write-Error "PID $serverPid is not the B+C local server. Nothing was stopped."
    }
    Stop-Process -Id $serverPid
    $process.WaitForExit(10000) | Out-Null
}

Remove-Item -LiteralPath $pidFile -Force

Write-Host 'B+C local server stopped.'
