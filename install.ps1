<#
.SYNOPSIS
    cdu installer for Windows.
.DESCRIPTION
    Downloads the matching release archive from GitHub, verifies its checksum, and
    installs cdu.exe to a per-user directory on PATH.

        irm https://raw.githubusercontent.com/pottom/cdu/main/install.ps1 | iex

.PARAMETER Version
    A release tag to install. Defaults to the latest release.
.PARAMETER InstallDir
    Where to install. Defaults to %LOCALAPPDATA%\Programs\cdu.
#>
[CmdletBinding()]
param(
    [string]$Version = $env:CDU_VERSION,
    [string]$InstallDir = $env:CDU_INSTALL_DIR
)

$ErrorActionPreference = 'Stop'
$repo = 'pottom/cdu'

# The release matrix builds Windows on amd64 only (matching gdu), so that is the one
# architecture there is a build to install.
$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    default { throw "no cdu Windows release for architecture $($env:PROCESSOR_ARCHITECTURE) (amd64 only)" }
}

if (-not $Version) {
    Write-Host 'Finding the latest release ...'
    $latest = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
    $Version = $latest.tag_name
}
if (-not $Version) { throw 'could not determine the latest release' }

# Archive names carry the version without its leading v (GoReleaser's convention).
$ver = $Version -replace '^v', ''
$asset = "cdu_${ver}_windows_${arch}.zip"
$base = "https://github.com/$repo/releases/download/$Version"

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("cdu-" + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    $zip = Join-Path $tmp $asset
    $sums = Join-Path $tmp 'sha256sums.txt'

    Write-Host "Downloading $asset ($Version) ..."
    Invoke-WebRequest "$base/$asset" -OutFile $zip
    Invoke-WebRequest "$base/sha256sums.txt" -OutFile $sums

    Write-Host 'Verifying checksum ...'
    $want = (Get-Content $sums | Where-Object { $_ -match "\s$([regex]::Escape($asset))$" } |
        ForEach-Object { ($_ -split '\s+')[0] } | Select-Object -First 1)
    if (-not $want) { throw "$asset is not listed in sha256sums.txt" }
    $got = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
    if ($want.ToLower() -ne $got) { throw "checksum mismatch for ${asset}: expected $want, got $got" }

    if (-not $InstallDir) {
        $InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\cdu'
    }
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

    Expand-Archive -Path $zip -DestinationPath $tmp -Force
    Copy-Item -Path (Join-Path $tmp 'cdu.exe') -Destination (Join-Path $InstallDir 'cdu.exe') -Force

    Write-Host "Installed cdu $Version to $InstallDir\cdu.exe"

    # A binary on PATH is the point; add the directory to the user PATH if it is not
    # already there, and note that a new shell is needed to pick it up.
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $InstallDir) {
        [Environment]::SetEnvironmentVariable('Path', "$userPath;$InstallDir", 'User')
        Write-Host "Added $InstallDir to your PATH — open a new terminal to use 'cdu'."
    }

    & (Join-Path $InstallDir 'cdu.exe') --version
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
