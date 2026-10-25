param([ValidateSet('start', 'status', 'stop', 'logs')][string]$Action = 'start')
$ErrorActionPreference = 'Stop'
$ProjectRoot = Split-Path -Parent $PSScriptRoot
Push-Location $ProjectRoot
try {
    switch ($Action) {
        'start' { docker compose up -d --build --wait }
        'status' { docker compose ps }
        'stop' { docker compose down }
        'logs' { docker compose logs --tail 100 }
    }
    if ($LASTEXITCODE -ne 0) { throw "Docker Compose failed: $LASTEXITCODE" }
} finally { Pop-Location }
