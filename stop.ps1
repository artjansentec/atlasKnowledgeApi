# Libera a porta da API (padrão 8080)
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$port = 8080
if (Test-Path ".env") {
    $envLine = Get-Content ".env" | Where-Object { $_ -match '^PORT=' } | Select-Object -First 1
    if ($envLine -match 'PORT=(\d+)') { $port = [int]$Matches[1] }
}

$connections = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
if (-not $connections) {
    Write-Host "Porta $port já está livre."
    exit 0
}

$connections | ForEach-Object {
    $proc = Get-Process -Id $_.OwningProcess -ErrorAction SilentlyContinue
    $name = if ($proc) { $proc.ProcessName } else { "?" }
    Write-Host "Encerrando $name (PID $($_.OwningProcess)) na porta $port..."
    Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue
}

Start-Sleep -Seconds 1
Write-Host "Porta $port liberada. Rode: go run ./cmd/api"
