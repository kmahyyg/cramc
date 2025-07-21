$PSNativeCommandUseErrorActionPreference = $true
$ErrorActionPreference = "SilentlyContinue"
$iUsername = ""

# Define the paths to clean
$paths = @(
    "C:\Users\$iUsername\AppData\Microsoft\Excel",
    "C:\TMP\",
    "C:\Users\$iUsername\AppData\Local\Microsoft\Windows\INetCache"
)

foreach ($ipath in $paths)
{
    Get-ChildItem -Path $ipath -Recurse -File -Force | Remove-Item -Force
    Get-ChildItem -Path $ipath -Recurse -Directory -Force  | Remove-Item -Force -Recurse
}

