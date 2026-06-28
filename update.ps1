# update.ps1 — Rebuild and restart Note-Aura WITHOUT touching any data.
#
# Stops the running note-aura.exe, rebuilds the binary from the current source,
# then starts it again. Your data (note-aura.db, uploads/, config.yaml) is left
# completely untouched — only the program is updated.
#
# Usage:
#   .\update.ps1            # stop, rebuild, restart
#   .\update.ps1 -NoStart   # stop and rebuild only (don't relaunch)
#
# Run it from the Note-Aura project directory (where main.go lives).

param(
    [switch]$NoStart
)

$ErrorActionPreference = "Stop"
$exe = "note-aura.exe"

# Move to the script's own directory so relative paths resolve correctly.
Set-Location -Path $PSScriptRoot

if (-not (Test-Path "main.go")) {
    Write-Error "main.go not found in $PSScriptRoot — run this from the Note-Aura project directory."
    exit 1
}

Write-Host "==> Stopping any running $exe ..." -ForegroundColor Cyan
$procs = Get-Process -Name "note-aura" -ErrorAction SilentlyContinue
if ($procs) {
    $procs | Stop-Process -Force
    # Wait until the process is gone so the binary file is unlocked for rebuild.
    for ($i = 0; $i -lt 20; $i++) {
        Start-Sleep -Milliseconds 250
        if (-not (Get-Process -Name "note-aura" -ErrorAction SilentlyContinue)) { break }
    }
    Write-Host "    stopped." -ForegroundColor DarkGray
} else {
    Write-Host "    none running." -ForegroundColor DarkGray
}

Write-Host "==> Building $exe ..." -ForegroundColor Cyan
# -buildvcs=false: skip Git VCS stamping. On a network-share / mapped-drive
# checkout, Go's git call can hit a "dubious ownership" check and exit 128
# ("error obtaining VCS status"); the stamp isn't needed for this build.
& go build -buildvcs=false -o $exe .
if ($LASTEXITCODE -ne 0) {
    Write-Error "go build failed — the old binary and your data are unchanged."
    exit 1
}
Write-Host "    build OK." -ForegroundColor Green

if ($NoStart) {
    Write-Host "==> -NoStart given; not relaunching. Start it yourself with: .\$exe" -ForegroundColor Yellow
    exit 0
}

Write-Host "==> Starting $exe ..." -ForegroundColor Cyan
$logFile = Join-Path $PSScriptRoot "note-aura.log"
# Redirect stdout+stderr to the log file via cmd, matching update.sh's nohup >> behaviour.
Start-Process -FilePath "cmd.exe" `
    -ArgumentList "/c `"$(Join-Path $PSScriptRoot $exe)`" >> `"$logFile`" 2>&1" `
    -WorkingDirectory $PSScriptRoot `
    -WindowStyle Hidden
Write-Host "    started. Update complete — data preserved. Logs → $logFile" -ForegroundColor Green
