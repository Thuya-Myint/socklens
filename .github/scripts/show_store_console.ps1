# Runs only on the disposable Windows screenshot runner.
$ErrorActionPreference = 'Stop'
Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class OwnConsole {
 [DllImport("kernel32.dll")] public static extern IntPtr GetConsoleWindow();
 [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr h,IntPtr after,int x,int y,int w,int height,uint flags);
 [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
}
"@
$Host.UI.RawUI.WindowTitle = 'SockLens v0.12.1 - Windows command options'
$Host.UI.RawUI.BackgroundColor = 'Black'
$Host.UI.RawUI.ForegroundColor = 'Gray'
$hwnd = [OwnConsole]::GetConsoleWindow()
[OwnConsole]::SetWindowPos($hwnd,[IntPtr]::Zero,10,10,1400,950,4) | Out-Null
[OwnConsole]::SetForegroundWindow($hwnd) | Out-Null
Clear-Host
Write-Host 'PS C:\SockLens> .\socklens.exe version'
& C:\SockLens\socklens.exe version
Write-Host ''
Write-Host 'PS C:\SockLens> .\socklens.exe list --help'
& C:\SockLens\socklens.exe list --help
Write-Host ''
Write-Host 'PS C:\SockLens> '
@{ hwnd=$hwnd.ToInt64(); version='v0.12.1'; source='bcd6bf310c32254be83441848eba1c9f4cfdac2b' } | ConvertTo-Json | Set-Content -LiteralPath $env:SOCKLENS_CAPTURE_MARKER
Start-Sleep -Seconds 120
