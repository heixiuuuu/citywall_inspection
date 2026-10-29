param([switch]$SkipBuild)
$ErrorActionPreference = 'Stop'
$ProjectRoot = Split-Path -Parent $PSScriptRoot
Push-Location $ProjectRoot
try {
    if (-not (Test-Path -LiteralPath '.env')) {
        Copy-Item -LiteralPath '.env.example' -Destination '.env'
    }
    $Compose = @('compose', '-f', 'docker-compose.yml', '-f', 'docker-compose.vision.yml')
    & docker @Compose up -d --wait vision
    if ($LASTEXITCODE -ne 0) { throw 'Unable to start the vision service' }
    $Models = & docker @Compose exec -T vision ollama list
    if ($LASTEXITCODE -ne 0) { throw 'Unable to list installed vision models' }
    if (-not ($Models | Where-Object { $_ -match '^qwen3-vl:2b-instruct\s' })) {
        & docker @Compose exec -T vision ollama pull qwen3-vl:2b-instruct
        if ($LASTEXITCODE -ne 0) { throw 'Unable to download the vision model' }
    }
    $Arguments = @('up', '-d', '--wait')
    if ($SkipBuild) { $Arguments += '--no-build' } else { $Arguments += '--build' }
    & docker @Compose @Arguments
    if ($LASTEXITCODE -ne 0) { throw 'Unable to start the system' }
} finally {
    Pop-Location
}
