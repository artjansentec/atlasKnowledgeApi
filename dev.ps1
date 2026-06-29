# Atlas Knowledge — scripts para Windows (PowerShell)
# Uso: .\dev.ps1          → migrate + API
#      .\dev.ps1 -ApiOnly → só a API (sem migrate)

param(
    [switch]$ApiOnly
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

# Libera porta 8080 se outra instância antiga estiver rodando
$port = 8080
if (Test-Path ".env") {
    $envLine = Get-Content ".env" | Where-Object { $_ -match '^PORT=' } | Select-Object -First 1
    if ($envLine -match 'PORT=(\d+)') { $port = [int]$Matches[1] }
}
$existing = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
if ($existing) {
    $pid = $existing.OwningProcess | Select-Object -First 1
    Write-Host ">> Encerrando processo antigo na porta $port (PID $pid)..."
    Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
}

if (-not (Test-Path ".env")) {
    Copy-Item ".env.example" ".env"
    Write-Host "Criado .env a partir de .env.example"
}

if (-not $ApiOnly) {
    Write-Host ">> Aplicando migrations..."
    go run ./cmd/migrate up
    if ($LASTEXITCODE -ne 0) {
        Write-Host ">> Tentando reparar migrations..."
        go run ./cmd/migrate repair
    }

}

Write-Host ">> Iniciando API em http://localhost:8080"
Write-Host ">> Swagger UI: http://localhost:8080/swagger"
go run ./cmd/api
