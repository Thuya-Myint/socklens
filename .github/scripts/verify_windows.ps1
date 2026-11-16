param(
    [Parameter(Mandatory)][string]$Directory,
    [Parameter(Mandatory)][string]$ExpectedSubject
)
$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($ExpectedSubject)) { throw 'Expected publisher subject is required.' }
foreach ($arch in @('amd64', 'arm64')) {
    $path = Join-Path $Directory "windows_$arch.exe"
    $signature = Get-AuthenticodeSignature -LiteralPath $path
    if ($signature.Status -ne 'Valid' -or $signature.SignatureType -ne 'Authenticode' -or
        $null -eq $signature.SignerCertificate -or $null -eq $signature.TimeStamperCertificate) {
        throw "Missing, invalid, untrusted, or untimestamped Authenticode signature: $path"
    }
    if ($signature.SignerCertificate.Subject -cne $ExpectedSubject) {
        throw "Unexpected signing publisher: $($signature.SignerCertificate.Subject)"
    }
    Write-Output "$arch verified: $($signature.SignerCertificate.Subject)"
}
