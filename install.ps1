Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = 'dreuse/prdash'
$bindir = if ($env:PRDASH_INSTALL_DIR) { $env:PRDASH_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'prdash\bin' }

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { throw "install: unsupported architecture $env:PROCESSOR_ARCHITECTURE" }
}

$version = $env:PRDASH_VERSION
if (-not $version) {
    $version = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest").tag_name
}
if (-not $version -or -not $version.StartsWith('v')) {
    throw "install: could not resolve a release tag, got '$version'"
}

$name = "prdash_${version}_windows_$arch"
$base = "https://github.com/$repo/releases/download/$version"
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) "prdash-install-$([System.IO.Path]::GetRandomFileName())"
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
    Write-Host "downloading prdash $version for windows/$arch"
    $archive = Join-Path $tmp "$name.zip"
    $sums = Join-Path $tmp 'checksums.txt'
    Invoke-WebRequest "$base/$name.zip" -OutFile $archive
    Invoke-WebRequest "$base/checksums.txt" -OutFile $sums

    $sum = (Get-FileHash $archive -Algorithm SHA256).Hash.ToLower()
    $published = $null
    foreach ($line in Get-Content $sums) {
        $fields = $line -split '\s+'
        if ($fields.Count -ge 2 -and $fields[1].TrimStart('*') -eq "$name.zip") {
            $published = $fields[0].ToLower()
            break
        }
    }
    if (-not $published) {
        throw "install: $name.zip is not listed in checksums.txt, refusing to install it"
    }
    if ($published -ne $sum) {
        throw "install: $name.zip does not match its published checksum, refusing to install it"
    }

    Expand-Archive -Path $archive -DestinationPath $tmp -Force
    New-Item -ItemType Directory -Path $bindir -Force | Out-Null
    Move-Item (Join-Path $tmp "$name\prdash.exe") (Join-Path $bindir 'prdash.exe') -Force
}
finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "installed prdash $version to $bindir"

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $bindir) {
    $updated = if ($userPath) { "$userPath;$bindir" } else { $bindir }
    [Environment]::SetEnvironmentVariable('Path', $updated, 'User')
    Write-Host "added $bindir to your PATH, open a new terminal to run it"
}

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    Write-Host 'prdash needs the gh cli, see https://cli.github.com'
}
