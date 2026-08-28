$ErrorActionPreference = 'Stop'

$root = $PSScriptRoot
$release = Join-Path $root 'release'
$binaryName = 'cursor-tab-server-linux-amd64'
$binaryPath = Join-Path $release $binaryName
$archivePath = Join-Path $release "$binaryName.tar.gz"

New-Item -ItemType Directory -Force -Path $release | Out-Null
Remove-Item -Force -ErrorAction SilentlyContinue $binaryPath, $archivePath

$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'

Push-Location $root
try {
    go build -trimpath -tags 'netgo osusergo' -ldflags '-s -w -extldflags=-static' -o $binaryPath .
    tar -czf $archivePath -C $release $binaryName
} finally {
    Pop-Location
}

Write-Host "Created: $binaryPath"
Write-Host "Created: $archivePath"
