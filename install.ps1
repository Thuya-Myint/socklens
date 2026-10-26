param(
    [string]$Version = "",
    [string]$InstallDir = "",
    [ValidateSet("auto", "amd64", "arm64")][string]$Architecture = "auto",
    [switch]$NoPathUpdate
)
$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") { throw "Use install.sh on Linux or macOS." }
if (-not $Version) {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/Thuya-Myint/socklens/releases/latest"
    $Version = $release.tag_name
}
if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$') { throw "Invalid release version." }
if ($Architecture -eq "auto") {
    $nativeArchitecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    switch ($nativeArchitecture) {
        "X64" { $Architecture = "amd64" }
        "Arm64" { $Architecture = "arm64" }
        default { throw "Unsupported Windows architecture: $nativeArchitecture" }
    }
}
if (-not $InstallDir) {
    if (-not $env:LOCALAPPDATA) { throw "LOCALAPPDATA is unavailable; provide -InstallDir." }
    $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\SockLens\bin"
}
$InstallDir = [System.IO.Path]::GetFullPath($InstallDir)
$destination = Join-Path $InstallDir "socklens.exe"
if (Test-Path -LiteralPath $destination) {
    $existing = Get-Item -LiteralPath $destination -Force
    if ($existing.PSIsContainer -or ($existing.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
        throw "Refusing to replace a directory or link: $destination"
    }
}
$temporaryRoot = [System.IO.Path]::GetTempPath()
$work = Join-Path $temporaryRoot ("socklens-install-" + [Guid]::NewGuid().ToString("N"))
$staged = $null
New-Item -ItemType Directory -Path $work | Out-Null
try {
    $folder = "socklens_${Version}_windows_${Architecture}"
    $archiveName = "$folder.zip"
    $archivePath = Join-Path $work $archiveName
    $manifestPath = Join-Path $work "SHA256SUMS"
    $base = "https://github.com/Thuya-Myint/socklens/releases/download/$Version"
    Invoke-WebRequest -UseBasicParsing -Uri "$base/$archiveName" -OutFile $archivePath
    Invoke-WebRequest -UseBasicParsing -Uri "$base/SHA256SUMS" -OutFile $manifestPath
    $pattern = '^([0-9a-fA-F]{64})\s+' + [regex]::Escape($archiveName) + '$'
    $checksums = @()
    foreach ($line in Get-Content -LiteralPath $manifestPath) {
        if ($line -match $pattern) { $checksums += $Matches[1].ToLowerInvariant() }
    }
    if ($checksums.Count -ne 1) { throw "Missing or duplicate archive checksum." }
    $actual = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $checksums[0]) { throw "Checksum mismatch; nothing installed." }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [System.IO.Compression.ZipFile]::OpenRead($archivePath)
    try {
        $entries = @($zip.Entries | Where-Object { $_.FullName -eq "$folder/socklens.exe" })
        if ($entries.Count -ne 1 -or $entries[0].Length -eq 0) { throw "Executable missing or duplicated in archive." }
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        $staged = Join-Path $InstallDir (".socklens-" + [Guid]::NewGuid().ToString("N") + ".tmp")
        $source = $entries[0].Open()
        try {
            $output = [System.IO.File]::Create($staged)
            try { $source.CopyTo($output) } finally { $output.Dispose() }
        } finally { $source.Dispose() }
    } finally { $zip.Dispose() }
    if ([System.IO.File]::Exists($destination)) {
        [System.IO.File]::Replace($staged, $destination, $null)
    } else {
        [System.IO.File]::Move($staged, $destination)
    }
    $staged = $null
    if (-not $NoPathUpdate) {
        $userPath = [string][Environment]::GetEnvironmentVariable("Path", "User")
        if (($userPath -split ';') -notcontains $InstallDir) {
            [Environment]::SetEnvironmentVariable("Path", ($userPath.TrimEnd(';') + ";" + $InstallDir).TrimStart(';'), "User")
        }
        if (($env:Path -split ';') -notcontains $InstallDir) { $env:Path += ";" + $InstallDir }
    }
    Write-Output "Installed SockLens $Version to $destination"
    Write-Output "Run: socklens version"
    if ($NoPathUpdate) { Write-Output "PATH unchanged; run the executable using its full path." }
} finally {
    if ($staged -and [System.IO.File]::Exists($staged)) { [System.IO.File]::Delete($staged) }
    $prefix = [System.IO.Path]::GetFullPath($temporaryRoot).TrimEnd([char[]]'\/') + [System.IO.Path]::DirectorySeparatorChar
    $resolvedWork = [System.IO.Path]::GetFullPath($work)
    if ($resolvedWork.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolvedWork)) {
        Remove-Item -LiteralPath $resolvedWork -Recurse -Force
    }
}
