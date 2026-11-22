# Captures real pixels on a disposable GitHub-hosted Windows runner.
# No application-control, execution-policy or certificate trust changes.
param([Parameter(Mandatory)][string]$OutputDirectory)
$ErrorActionPreference = 'Stop'
$null = New-Item -ItemType Directory -Path $OutputDirectory -Force
$marker = Join-Path $OutputDirectory 'ready.json'
$childScript = Join-Path $PSScriptRoot 'show_store_console.ps1'
$env:SOCKLENS_CAPTURE_MARKER = $marker
# This visible console is the screenshot subject, not a background service.
$process = Start-Process powershell.exe -ArgumentList @('-NoProfile','-File',('"'+$childScript+'"')) -WorkingDirectory C:\SockLens -WindowStyle Normal -PassThru
try {
 $deadline = (Get-Date).AddSeconds(45)
 while (-not (Test-Path -LiteralPath $marker)) {
  if ($process.HasExited) { throw "Console exited with $($process.ExitCode)" }
  if ((Get-Date) -gt $deadline) { throw 'Console did not become ready' }
  Start-Sleep -Milliseconds 250
 }
 Start-Sleep -Seconds 2
 $info = Get-Content -LiteralPath $marker -Raw | ConvertFrom-Json
 Add-Type -AssemblyName System.Drawing
 Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class ConsoleCapture {
 [StructLayout(LayoutKind.Sequential)] public struct Rect { public int Left,Top,Right,Bottom; }
 [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hwnd,out Rect r);
 [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr hwnd,IntPtr dc,uint flags);
}
"@
 $rectangle = New-Object ConsoleCapture+Rect
 if (-not [ConsoleCapture]::GetWindowRect([IntPtr]$info.hwnd,[ref]$rectangle)) { throw 'Cannot locate console window' }
 $width=$rectangle.Right-$rectangle.Left; $height=$rectangle.Bottom-$rectangle.Top
 if ($width -le 0 -or $height -le 0) { throw 'Invalid console bounds' }
 $bitmap = New-Object System.Drawing.Bitmap($width,$height)
 $graphics = [Drawing.Graphics]::FromImage($bitmap)
 try {
  $dc=$graphics.GetHdc()
  try { $printed=[ConsoleCapture]::PrintWindow([IntPtr]$info.hwnd,$dc,2) } finally {$graphics.ReleaseHdc($dc)}
  $bitmap.Save((Join-Path $OutputDirectory 'console-window.png'),[Drawing.Imaging.ImageFormat]::Png)
  try {
   $graphics.CopyFromScreen($rectangle.Left,$rectangle.Top,0,0,$bitmap.Size)
   $bitmap.Save((Join-Path $OutputDirectory 'console-screen.png'),[Drawing.Imaging.ImageFormat]::Png)
  } catch { $_ | Out-String | Set-Content (Join-Path $OutputDirectory 'screen-capture-error.txt') }
 } finally { $graphics.Dispose(); $bitmap.Dispose() }
 & C:\SockLens\socklens.exe version | Set-Content (Join-Path $OutputDirectory 'application-version.txt')
 & C:\SockLens\socklens.exe list --help *> (Join-Path $OutputDirectory 'application-help.txt')
 Write-Output "Captured actual console: $width x $height; PrintWindow=$printed"
} finally {
 if (-not $process.HasExited) { Stop-Process -Id $process.Id }
}
