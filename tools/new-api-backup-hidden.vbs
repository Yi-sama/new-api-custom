Option Explicit

Dim fso, shell, scriptPath, command
Set fso = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("WScript.Shell")

scriptPath = fso.BuildPath(fso.GetParentFolderName(WScript.ScriptFullName), "new-api-backup-pull.ps1")
command = "powershell.exe -NoProfile -NoLogo -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File """ & scriptPath & """"
WScript.Quit shell.Run(command, 0, True)
