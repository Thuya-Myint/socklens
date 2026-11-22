# Microsoft Store package

SockLens is submitted as an MSIX bundle with x64 and ARM64 packages. The manifest
uses the package identity assigned to SockLens in Partner Center, with a console
execution alias of `socklens.exe`. The Store signs the package after certification
and manages its updates.

From the repository root, cross-build the Store executables with Go and Make:

```sh
make store-build VERSION=v0.12.1
```

From PowerShell on Windows, use MakeAppx from the Windows SDK to package those
executables. Pass the Windows path to the output of the previous command when
building in WSL.

```powershell
.\tools\msix\package.ps1 -MakeAppx 'C:\path\to\MakeAppx.exe' `
    -InputDirectory .\dist\store-v0.12.1 -OutputDirectory .\dist\store-v0.12.1 `
    -Version v0.12.1
```

Upload the resulting `SockLens_0.12.1.0.msixbundle` in the Packages section of the
SockLens MSIX submission. The Store's fourth version component is reserved. A
subsequent build for the Store needs an increased first, second, or third
component. Check that Partner Center shows both architectures and the correct
product identity.

For the restricted-capability explanation, SockLens needs to run as a desktop
console application for its execution alias and to query local socket and
process information using Windows APIs. It runs with the launching user's
permissions. Verify the Store-installed build from PowerShell with
`socklens version`, `socklens probe --help`, and `socklens update --help`.

Microsoft documents [packaging a CLI as MSIX](https://learn.microsoft.com/en-us/windows/apps/dev-tools/winapp-cli/guides/packaging-cli)
and [submitting an MSIX app](https://learn.microsoft.com/en-us/windows/apps/publish/publish-your-app/create-app-submission?pivots=store-installer-msix).
