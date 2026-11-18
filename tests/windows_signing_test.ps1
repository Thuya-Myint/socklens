$ErrorActionPreference = 'Stop'
$verifier = Join-Path $PSScriptRoot '../.github/scripts/verify_windows.ps1'
$global:socklensSigningTestFixture = $null
function Get-AuthenticodeSignature {
    param([string]$LiteralPath)
    return $global:socklensSigningTestFixture
}
foreach ($case in @('valid', 'unsigned', 'tampered', 'untrusted', 'publisher', 'timestamp', 'catalog')) {
    $global:socklensSigningTestFixture = [pscustomobject]@{
        Status = 'Valid'
        SignatureType = 'Authenticode'
        SignerCertificate = [pscustomobject]@{ Subject = 'CN=Expected Publisher' }
        TimeStamperCertificate = [pscustomobject]@{ Subject = 'CN=Timestamp' }
    }
    switch ($case) {
        unsigned { $global:socklensSigningTestFixture.Status = 'NotSigned' }
        tampered { $global:socklensSigningTestFixture.Status = 'HashMismatch' }
        untrusted { $global:socklensSigningTestFixture.Status = 'NotTrusted' }
        publisher { $global:socklensSigningTestFixture.SignerCertificate.Subject = 'CN=Other Publisher' }
        timestamp { $global:socklensSigningTestFixture.TimeStamperCertificate = $null }
        catalog { $global:socklensSigningTestFixture.SignatureType = 'Catalog' }
    }
    $failed = $false
    try { & $verifier -Directory $PSScriptRoot -ExpectedSubject 'CN=Expected Publisher' | Out-Null }
    catch { $failed = $true }
    if ($failed -ne ($case -ne 'valid')) { throw "Incorrect signature gate result: $case" }
}
Write-Output 'Windows signature gate fixtures passed (7 cases).'
