#requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string] $MakeAppx,

    [Parameter(Mandatory)]
    [string] $InputDirectory,

    [Parameter(Mandatory)]
    [string] $OutputDirectory,

    [Parameter(Mandatory)]
    [ValidatePattern('^v?(0|[1-9][0-9]{0,4})\.(0|[1-9][0-9]{0,4})\.(0|[1-9][0-9]{0,4})$')]
    [string] $Version
)

$ErrorActionPreference = 'Stop'
$versionParts = $Version.TrimStart('v').Split('.')
if ([int]$versionParts[0] -eq 0 -and [int]$versionParts[1] -eq 0 -and [int]$versionParts[2] -eq 0) {
    throw 'A Store package must have a nonzero version.'
}
if ($versionParts | Where-Object { [int]$_ -gt 65535 }) {
    throw 'MSIX version components must be at most 65535.'
}
$packageVersion = ($versionParts -join '.') + '.0'
$sdkTool = (Resolve-Path -LiteralPath $MakeAppx).Path
$source = (Resolve-Path -LiteralPath $InputDirectory).Path
$repo = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..')).Path
$output = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputDirectory)
$null = New-Item -ItemType Directory -Path $output -Force
$bundle = Join-Path $output ('SockLens_' + $packageVersion + '.msixbundle')
if (Test-Path -LiteralPath $bundle) { throw "Package already exists at $bundle." }

$stage = Join-Path $output ('.msix-stage-' + [Guid]::NewGuid().ToString('N'))
$null = New-Item -ItemType Directory -Path $stage
try {
    $packages = Join-Path $stage 'packages'
    $null = New-Item -ItemType Directory -Path $packages
    foreach ($architecture in @('x64', 'arm64')) {
        $layout = Join-Path $stage $architecture
        $null = New-Item -ItemType Directory -Path $layout
        $executable = Join-Path $source ('socklens-' + $architecture + '.exe')
        Copy-Item -LiteralPath $executable -Destination (Join-Path $layout 'socklens.exe')
        Copy-Item -LiteralPath (Join-Path $repo 'LICENSE') -Destination $layout
        Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'Assets') -Destination $layout -Recurse

        [xml]$manifest = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'AppxManifest.xml') -Raw
        $manifest.Package.Identity.SetAttribute('Version', $packageVersion)
        $manifest.Package.Identity.SetAttribute('ProcessorArchitecture', $architecture)
        $manifest.Save((Join-Path $layout 'AppxManifest.xml'))

        $package = Join-Path $packages ('SockLens_' + $packageVersion + '_' + $architecture + '.msix')
        & $sdkTool pack /d $layout /p $package
        if ($LASTEXITCODE -ne 0) { throw "MakeAppx pack failed for $architecture (exit $LASTEXITCODE)." }
    }

    $stagedBundle = Join-Path $stage ([IO.Path]::GetFileName($bundle))
    & $sdkTool bundle /d $packages /p $stagedBundle /bv $packageVersion
    if ($LASTEXITCODE -ne 0) { throw "MakeAppx bundle failed (exit $LASTEXITCODE)." }
    Move-Item -LiteralPath $stagedBundle -Destination $bundle
    Get-Item -LiteralPath $bundle
} finally {
    $resolvedOutput = (Resolve-Path -LiteralPath $output).Path.TrimEnd('\') + '\'
    $resolvedStage = (Resolve-Path -LiteralPath $stage).Path
    if ($resolvedStage.StartsWith($resolvedOutput, [StringComparison]::OrdinalIgnoreCase) -and
        (Split-Path -Leaf $resolvedStage) -match '^\.msix-stage-[a-f0-9]{32}$') {
        Remove-Item -LiteralPath $resolvedStage -Recurse -Force
    }
}
