param(
    [Parameter(Mandatory = $true)]
    [string]$Port = "COM6"
)

$ErrorActionPreference = "Stop"
$hex = "C:\gocode\klipper\out\klipper.elf.hex"
if (-not (Test-Path $hex)) {
    Write-Error "Missing $hex — run build-ramps.sh in C:\gocode\klipper first."
}

Write-Host "Flashing Klipper to $Port ..."
avrdude -c wiring -p m2560 -P $Port -b 115200 -D -U "flash:w:${hex}:i"

Write-Host "Done. Wait a few seconds, then probe with:"
Write-Host "  go -C backend run ./cmd/mcu-probe --port $Port --json"
