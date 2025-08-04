# Get the current user and check SYSTEM
$currentUser = [System.Security.Principal.WindowsIdentity]::GetCurrent()
if ($currentUser.IsSystem) {
    Write-Host "It's forbidden to run under SYSTEM."
    exit
}
# Create Empty Folder
Remove-Item -Path "C:\TEMP\EMPTYDIR" -Recurse -Force -ErrorAction SilentlyContinue
New-Item -Path "C:\TEMP\EMPTYDIR" -ItemType Directory -Force -ErrorAction Stop
# Kill Excel Process
Get-Process "EXCEL" -ErrorAction SilentlyContinue | Stop-Process -Force
# Robocopy overwrite folder
robocopy.exe "C:\TEMP\EMPTYDIR" "$HOME\AppData\Roaming\Microsoft\Excel" /MIR
robocopy.exe "C:\TEMP\EMPTYDIR" "C:\TMP" /MIR
robocopy.exe "C:\TEMP\EMPTYDIR" "$HOME\AppData\Local\Microsoft\Windows\INetCache" /MIR
# Remove Empty Folder (defer)
Remove-Item -Path "C:\TEMP\EMPTYDIR" -Recurse -Force -ErrorAction SilentlyContinue
Write-Host "Done."
