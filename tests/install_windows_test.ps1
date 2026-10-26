$ErrorActionPreference = "Stop"
$project = Split-Path -Parent $PSScriptRoot
$temporaryRoot = [System.IO.Path]::GetTempPath()
$testRoot = Join-Path $temporaryRoot ("socklens-installer-test-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $testRoot | Out-Null
$global:SockLensFixtureDownloads = Join-Path $testRoot "downloads"
New-Item -ItemType Directory -Path $global:SockLensFixtureDownloads | Out-Null
function global:Invoke-WebRequest {
    param([string]$Uri, [string]$OutFile, [switch]$UseBasicParsing)
    $name = [System.IO.Path]::GetFileName(([Uri]$Uri).AbsolutePath)
    Copy-Item -LiteralPath (Join-Path $global:SockLensFixtureDownloads $name) -Destination $OutFile
}
try {
    $folderName = "socklens_v0.2.0_windows_amd64"
    $folder = Join-Path $testRoot $folderName
    New-Item -ItemType Directory -Path $folder | Out-Null
    $fixture = [byte[]](77,90,1,2,3,4)
    [System.IO.File]::WriteAllBytes((Join-Path $folder "socklens.exe"), $fixture)
    $archive = Join-Path $global:SockLensFixtureDownloads "$folderName.zip"
    Compress-Archive -LiteralPath $folder -DestinationPath $archive
    $hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    Set-Content -LiteralPath (Join-Path $global:SockLensFixtureDownloads "SHA256SUMS") -Value "$hash  $folderName.zip" -Encoding ascii
    $installDir = Join-Path $testRoot "installed"
    $taskOriginalPath = $env:Path
    try {
        $env:Path = ""
        & (Join-Path $project "install.ps1") -Version v0.2.0 -Architecture amd64 -InstallDir $installDir -NoPathUpdate
    } finally { $env:Path = $taskOriginalPath }
    $installed = Join-Path $installDir "socklens.exe"
    if ([Convert]::ToBase64String([System.IO.File]::ReadAllBytes($installed)) -ne [Convert]::ToBase64String($fixture)) { throw "Installed bytes differ." }
    Write-Output "PASS: Windows installation without Go, compiler, external programs, or PATH changes"
    [System.IO.File]::WriteAllBytes($archive, [byte[]](0,1,2))
    $failed = $false
    try {
        & (Join-Path $project "install.ps1") -Version v0.2.0 -Architecture amd64 -InstallDir $installDir -NoPathUpdate
    } catch {
        if ($_.ToString() -notmatch "Checksum mismatch") { throw }
        $failed = $true
    }
    if (-not $failed) { throw "Bad checksum accepted." }
    if ([Convert]::ToBase64String([System.IO.File]::ReadAllBytes($installed)) -ne [Convert]::ToBase64String($fixture)) { throw "Failed install changed existing executable." }
    Write-Output "PASS: bad checksum preserves the existing executable"
} finally {
    $prefix = [System.IO.Path]::GetFullPath($temporaryRoot).TrimEnd([char[]]'\/') + [System.IO.Path]::DirectorySeparatorChar
    $resolved = [System.IO.Path]::GetFullPath($testRoot)
    if ($resolved.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) { Remove-Item -LiteralPath $resolved -Recurse -Force }
    Remove-Item Function:\Invoke-WebRequest -ErrorAction SilentlyContinue
    Remove-Variable SockLensFixtureDownloads -Scope Global -ErrorAction SilentlyContinue
}
