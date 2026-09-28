@echo off
setlocal
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0stop-local-b-c.ps1"
if errorlevel 1 pause
