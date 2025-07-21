$PSNativeCommandUseErrorActionPreference = "Continue"
$ErrorActionPreference = "Continue"

Write-Host "Error message will keep poping WITHOUT stop further execution."

function CleanUp-StaleFiles {
    param(
        [Parameter(Mandatory=$true)]
        [string]$username
    )

    # Define the paths to clean
    $paths = @(
        "C:\Users\$username\AppData\Microsoft\Excel",
        "C:\TMP\",
        "C:\Users\$username\AppData\Local\Microsoft\Windows\INetCache"
    )

    foreach ($path in $paths) {
        if (Test-Path $path) {
            Write-Host "Cleaning path: $path"

            # Get all files recursively, including hidden and system files
            $files = Get-ChildItem -Path $path -Recurse -File -Force -ErrorAction Continue

            foreach ($file in $files) {
                # Remove the file
                Remove-Item -Path $file.FullName -Force -ErrorAction Continue
                Write-Host "  Removed: $($file.FullName)"
            }

            # Remove empty directories
            $directories = Get-ChildItem -Path $path -Recurse -Directory -Force -ErrorAction Continue | Sort-Object FullName -Descending
            foreach ($directory in $directories) {
                if ((Get-ChildItem -Path $directory.FullName -Force -ErrorAction Continue).Count -eq 0) {
                    Remove-Item -Path $directory.FullName -Force -ErrorAction Continue
                    Write-Host "  Removed empty directory: $($directory.FullName)"
                }
            }

            Write-Host "Completed cleaning: $path"
        }
        else {
            Write-Host "Path does not exist: $path"
        }
    }

    Write-Host "File removal operation completed for user: $username"
}
