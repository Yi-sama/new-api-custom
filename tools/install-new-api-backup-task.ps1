$ErrorActionPreference = 'Stop'
$scriptPath = Join-Path $PSScriptRoot 'new-api-backup-pull.ps1'
$wrapperPath = Join-Path $PSScriptRoot 'new-api-backup-hidden.vbs'
$taskName = 'US server application backup pull'
$installRoot = Join-Path $env:LOCALAPPDATA 'NewApiBackup'
$installedScript = Join-Path $installRoot 'new-api-backup-pull.ps1'
$installedWrapper = Join-Path $installRoot 'new-api-backup-hidden.vbs'
New-Item -ItemType Directory -Force -Path $installRoot | Out-Null
$sid = [Security.Principal.WindowsIdentity]::GetCurrent().User.Value
& icacls.exe $installRoot /inheritance:r /grant:r "*$sid`:(OI)(CI)F" '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' /Q | Out-Null
if ($LASTEXITCODE -ne 0) { throw "Failed to secure $installRoot" }
Remove-Item -LiteralPath $installedScript -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $installedWrapper -Force -ErrorAction SilentlyContinue
Copy-Item -LiteralPath $scriptPath, $wrapperPath -Destination $installRoot -Force
$action = New-ScheduledTaskAction -Execute 'wscript.exe' -Argument "//B //NoLogo `"$installedWrapper`""
$trigger = New-ScheduledTaskTrigger -Once -At ((Get-Date).AddMinutes(2)) -RepetitionInterval (New-TimeSpan -Hours 6) -RepetitionDuration (New-TimeSpan -Days 3650)
$principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType Interactive -RunLevel Limited
$settings = New-ScheduledTaskSettingsSet -Hidden -StartWhenAvailable -ExecutionTimeLimit (New-TimeSpan -Hours 1)
Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
Write-Output "registered: $taskName"
