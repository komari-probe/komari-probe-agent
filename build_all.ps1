# Requires: goreleaser (https://goreleaser.com/install/)

if (-not (Get-Command goreleaser -ErrorAction SilentlyContinue)) {
    Write-Host "goreleaser not found. Install it: https://goreleaser.com/install/" -ForegroundColor Red
    exit 1
}

& goreleaser release --snapshot --clean
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

$buildDir = Join-Path -Path (Get-Location) -ChildPath 'build'

# goreleaser lays each target's binary out under build\<id>_<goos>_<goarch>[_<variant>]\;
# flatten to the komari-agent-<os>-<arch>[.exe] naming the rest of the project expects.
$artifacts = Get-Content (Join-Path $buildDir 'artifacts.json') -Raw | ConvertFrom-Json
foreach ($a in $artifacts) {
    if ($a.name -like 'komari-agent-*') {
        Copy-Item -Path $a.path -Destination (Join-Path $buildDir $a.name) -Force
    }
}

Get-ChildItem -Path $buildDir -Directory -Filter 'komari-agent_*' | Remove-Item -Recurse -Force
Remove-Item -Path (Join-Path $buildDir 'artifacts.json'), (Join-Path $buildDir 'config.yaml'), (Join-Path $buildDir 'metadata.json') -ErrorAction SilentlyContinue

Write-Host "`nBinaries are in the .\build directory." -ForegroundColor White
