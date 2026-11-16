param(
    [Parameter(Mandatory)][string]$Version,
    [Parameter(Mandatory)][string]$Commit,
    [Parameter(Mandatory)][string]$Directory
)
$ErrorActionPreference = 'Stop'
$stamp = git show -s --format=%cI $Commit
if ($LASTEXITCODE -ne 0) { throw 'Cannot resolve build date.' }
New-Item -ItemType Directory -Path $Directory -Force | Out-Null
$env:GOOS = 'windows'
$env:CGO_ENABLED = '0'
foreach ($arch in @('amd64', 'arm64')) {
    $env:GOARCH = $arch
    go build -trimpath -ldflags "-s -w -X main.version=$Version -X main.commit=$Commit -X main.buildDate=$stamp" -o (Join-Path $Directory "windows_$arch.exe") ./cmd/socklens
    if ($LASTEXITCODE -ne 0) { throw "Build failed: $arch" }
}
