<#
.SYNOPSIS
The Windows installer (.github/workflows/release.yml).

  scripts/package_windows.ps1 -Tag <tag> -Stage <dir>    -> dist\rimworld-go-<tag>-windows-setup.exe
  scripts/package_windows.ps1 -Tag <tag> -Smoke <setup>  install silently for this user, run -version
                                                          and -selfcheck, uninstall, check nothing is left

-Stage is the folder `scripts/package_release.sh <exe> <name> dir` staged
(rimworld-go.exe, built with -H=windowsgui and its go-winres icon and
version resource, plus the assets). Inno Setup is installed with choco when
ISCC is missing. The icon is packaging\icons\rimworld-go.ico; ALLOW_NO_ICON=1
leaves it out, for a local test only. When WINDOWS_CERT_PFX_BASE64 (and
WINDOWS_CERT_PASSWORD) are set, the game and the setup are signed with
signtool; otherwise both stay unsigned.

A -H=windowsgui program has no console, so the smoke test reads -version
and -selfcheck through Start-Process -RedirectStandardOutput, which hands
the game a file as its standard output.
#>
param(
    [Parameter(Mandatory = $true)] [string] $Tag,
    [string] $Stage,
    [string] $Smoke
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$tmp = if ($env:RUNNER_TEMP) { $env:RUNNER_TEMP } else { $env:TEMP }

# NumVersion turns v0.3.0-rc1 into 0.3.0.0 (a version resource takes four numbers).
function NumVersion([string] $tag) {
    $v = $tag.TrimStart('v') -replace '[-+].*$', ''
    if ($v -notmatch '^\d+(\.\d+){0,3}$') { $v = '0.0.0' }
    $parts = [System.Collections.Generic.List[string]]::new()
    $parts.AddRange([string[]]$v.Split('.'))
    while ($parts.Count -lt 4) { $parts.Add('0') }
    return ($parts -join '.')
}

function Find-Iscc {
    $cmd = Get-Command iscc -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $candidates = @(
        (Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6\ISCC.exe'),
        (Join-Path $env:ProgramFiles 'Inno Setup 6\ISCC.exe'),
        (Join-Path $env:LOCALAPPDATA 'Programs\Inno Setup 6\ISCC.exe')
    )
    foreach ($p in $candidates) { if (Test-Path $p) { return $p } }
    return $null
}

function Get-Iscc {
    $iscc = Find-Iscc
    if ($iscc) { return $iscc }
    Write-Host 'package_windows: Inno Setup not found; installing it with choco'
    choco install innosetup -y --no-progress | Out-Host
    if ($LASTEXITCODE -ne 0) { throw 'package_windows: choco install innosetup failed' }
    $iscc = Find-Iscc
    if (-not $iscc) { throw 'package_windows: ISCC.exe is still missing after choco install innosetup' }
    return $iscc
}

# Sign-File signs path when the certificate secrets are set; it is a no-op otherwise.
function Sign-File([string] $path) {
    if (-not $env:WINDOWS_CERT_PFX_BASE64) { return }
    $signtool = Get-ChildItem -Path (Join-Path ${env:ProgramFiles(x86)} 'Windows Kits\10\bin\*\x64\signtool.exe') |
        Sort-Object FullName | Select-Object -Last 1
    if (-not $signtool) { throw 'package_windows: signtool.exe not found' }
    $pfx = Join-Path $tmp 'rimworld-go-sign.pfx'
    [IO.File]::WriteAllBytes($pfx, [Convert]::FromBase64String($env:WINDOWS_CERT_PFX_BASE64))
    try {
        & $signtool.FullName sign /f $pfx /p $env:WINDOWS_CERT_PASSWORD /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 $path
        if ($LASTEXITCODE -ne 0) { throw "package_windows: signing $path failed" }
    } finally {
        Remove-Item -Force $pfx
    }
}

function Build-Setup {
    $stageDir = (Resolve-Path $Stage).Path
    $exe = Join-Path $stageDir 'rimworld-go.exe'
    if (-not (Test-Path $exe)) { throw "package_windows: $exe is missing" }
    $icon = Join-Path $repo 'packaging\icons\rimworld-go.ico'
    $defines = @("/DAppTag=$Tag", "/DAppNumVersion=$(NumVersion $Tag)", "/DStageDir=$stageDir", "/DOutDir=$(Join-Path $repo 'dist')")
    if (Test-Path $icon) {
        $defines += "/DIconFile=$icon"
    } elseif ($env:ALLOW_NO_ICON -ne '1') {
        throw "package_windows: $icon is missing (make app-icon); ALLOW_NO_ICON=1 skips it for a local test"
    }
    Sign-File $exe
    $iscc = Get-Iscc
    & $iscc /Qp @defines (Join-Path $repo 'packaging\windows\rimworld-go.iss')
    if ($LASTEXITCODE -ne 0) { throw "package_windows: ISCC failed ($LASTEXITCODE)" }
    $setup = Join-Path $repo "dist\rimworld-go-$Tag-windows-setup.exe"
    Sign-File $setup
    $size = [math]::Round((Get-Item $setup).Length / 1MB)
    Write-Host "package_windows: $setup ($size MB)"
}

# Run-Game runs the installed game with one flag from C:\ (so the working
# directory lends it no assets) and returns its exit code and output.
function Run-Game([string] $exe, [string] $flag) {
    $out = Join-Path $tmp "rimworld-go$flag.out"
    $err = Join-Path $tmp "rimworld-go$flag.err"
    $p = Start-Process -FilePath $exe -ArgumentList $flag -WorkingDirectory "$env:SystemDrive\" -Wait -PassThru `
        -RedirectStandardOutput $out -RedirectStandardError $err
    $text = (Get-Content -Raw -ErrorAction SilentlyContinue $out)
    if ($null -eq $text) { $text = '' }
    $errText = (Get-Content -Raw -ErrorAction SilentlyContinue $err)
    if ($errText) { Write-Host "stderr: $errText" }
    return @{ Code = $p.ExitCode; Out = $text.Trim() }
}

function Test-Setup {
    $setup = (Resolve-Path $Smoke).Path
    $log = Join-Path $tmp 'rimworld-go-setup.log'
    $p = Start-Process -FilePath $setup -ArgumentList '/VERYSILENT', '/CURRENTUSER', '/SUPPRESSMSGBOXES', '/NORESTART', "/LOG=`"$log`"" -Wait -PassThru
    if ($p.ExitCode -ne 0) { Get-Content $log | Out-Host; throw "smoke: setup exited $($p.ExitCode)" }
    $app = Join-Path $env:LOCALAPPDATA 'Programs\rimworld-go'
    $exe = Join-Path $app 'rimworld-go.exe'
    if (-not (Test-Path $exe)) { Get-Content $log | Out-Host; throw "smoke: $exe was not installed (per-user install expected)" }
    $menu = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\rimworld-go.lnk'
    if (-not (Test-Path $menu)) { throw "smoke: no Start menu entry at $menu" }

    $v = Run-Game $exe '-version'
    if ($v.Code -ne 0 -or $v.Out -ne $Tag) { throw "smoke: -version exited $($v.Code) and printed '$($v.Out)', want '$Tag'" }
    $c = Run-Game $exe '-selfcheck'
    Write-Host $c.Out
    if ($c.Code -ne 0) { throw "smoke: -selfcheck exited $($c.Code)" }
    $roots = 0
    foreach ($line in ($c.Out -split "`r?`n")) {
        if ($line -match '^ok\s+(translations|art|menu splash|sounds): (.*)$') {
            if ($Matches[2] -notlike "*\Programs\rimworld-go\assets*") { throw "smoke: '$line' is outside the installed assets" }
            $roots++
        }
    }
    if ($roots -ne 4) { throw "smoke: the -selfcheck report names $roots of the 4 asset roots" }

    # The uninstaller copies itself to a temporary folder and returns at
    # once, so wait for the install folder to go.
    $u = Start-Process -FilePath (Join-Path $app 'unins000.exe') -ArgumentList '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART' -Wait -PassThru
    if ($u.ExitCode -ne 0) { throw "smoke: uninstall exited $($u.ExitCode)" }
    $deadline = (Get-Date).AddSeconds(90)
    while ((Test-Path $app) -and (Get-Date) -lt $deadline) { Start-Sleep -Seconds 2 }
    if (Test-Path $app) {
        Get-ChildItem -Recurse $app | Select-Object -First 20 | Out-Host
        throw "smoke: $app is still there after the uninstall"
    }
    if (Test-Path $menu) { throw 'smoke: the Start menu entry outlived the uninstall' }
    Write-Host "smoke: $setup ok"
}

if ($Smoke) {
    Test-Setup
} elseif ($Stage) {
    Build-Setup
} else {
    throw 'package_windows: pass -Stage <dir> or -Smoke <setup.exe>'
}
