@echo off
setlocal
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0start-local-b-c.ps1"
if errorlevel 1 pause
