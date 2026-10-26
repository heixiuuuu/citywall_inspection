param(
    [string]$Python = 'C:\environment\citywall-python312\Scripts\python.exe',
    [int]$MySQLPort = 13307,
    [int]$APIPort = 18080,
    [int]$WebPort = 15173,
    [switch]$SkipBuildChecks
)
$ErrorActionPreference = 'Stop'
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$TestProject = 'citywall-test-' + [Guid]::NewGuid().ToString('N').Substring(0, 10)
$TestOutput = Join-Path 'C:\code\tmp' $TestProject
$ComposeArguments = @('compose', '-p', $TestProject, '-f', (Join-Path $ProjectRoot 'docker-compose.yml'), '-f', (Join-Path $ProjectRoot 'docker-compose.acceptance.yml'))
$EnvironmentNames = @('MYSQL_PORT', 'SERVER_PORT', 'WEB_PORT', 'MYSQL_PASSWORD', 'MYSQL_ROOT_PASSWORD', 'JWT_SECRET', 'RECOGNIZER_TOKEN', 'VLM_BASE_URL', 'VLM_MODEL', 'VLM_API_KEY', 'WECHAT_APP_ID', 'WECHAT_APP_SECRET', 'CITY_WALL_TEST_DSN', 'CITY_WALL_API_BASE', 'CITY_WALL_TEST_PDF', 'PYTHONIOENCODING')
$PreviousEnvironment = @{}
foreach ($EnvironmentName in $EnvironmentNames) { $PreviousEnvironment[$EnvironmentName] = [Environment]::GetEnvironmentVariable($EnvironmentName, 'Process') }

function Invoke-Checked {
    param([string]$Executable, [string[]]$CommandArguments, [string]$Directory = $ProjectRoot)
    Push-Location $Directory
    try {
        & $Executable @CommandArguments
        if ($LASTEXITCODE -ne 0) { throw "$Executable failed with exit code $LASTEXITCODE" }
    } finally { Pop-Location }
}

try {
    if (-not (Test-Path -LiteralPath $Python)) { throw "Python environment not found: $Python" }
    New-Item -ItemType Directory -Path $TestOutput -Force | Out-Null
    $env:MYSQL_PORT = "$MySQLPort"
    $env:SERVER_PORT = "$APIPort"
    $env:WEB_PORT = "$WebPort"
    $env:MYSQL_PASSWORD = 'citywall_test'
    $env:MYSQL_ROOT_PASSWORD = 'citywall_test_root'
    $env:JWT_SECRET = 'citywall_acceptance_jwt_secret'
    $env:RECOGNIZER_TOKEN = ''
    $env:VLM_BASE_URL = ''
    $env:VLM_MODEL = ''
    $env:VLM_API_KEY = ''
    $env:WECHAT_APP_ID = ''
    $env:WECHAT_APP_SECRET = ''
    $env:PYTHONIOENCODING = 'utf-8'
    $env:CITY_WALL_API_BASE = "http://127.0.0.1:$APIPort/api/v1"
    $env:CITY_WALL_TEST_DSN = "root:citywall_test_root@tcp(127.0.0.1:$MySQLPort)/city_wall_test?charset=utf8mb4&parseTime=True&loc=UTC&multiStatements=true"
    $env:CITY_WALL_TEST_PDF = Join-Path $TestOutput 'report.pdf'

    if (-not $SkipBuildChecks) {
        Invoke-Checked 'go' @('test', './...') (Join-Path $ProjectRoot 'server')
        Invoke-Checked 'go' @('vet', './...') (Join-Path $ProjectRoot 'server')
        Invoke-Checked $Python @('-m', 'pytest', 'tests', '-q') (Join-Path $ProjectRoot 'recognizer')
        foreach ($Frontend in @('web', 'miniapp')) {
            Invoke-Checked 'npm' @('ci') (Join-Path $ProjectRoot $Frontend)
        }
        Invoke-Checked 'npm' @('run', 'build') (Join-Path $ProjectRoot 'web')
        foreach ($Target in @('build:h5', 'build:mp-weixin', 'build:app')) {
            Invoke-Checked 'npm' @('run', $Target) (Join-Path $ProjectRoot 'miniapp')
        }
    }
    $env:RECOGNIZER_TOKEN = 'citywall_acceptance_internal'
    Invoke-Checked 'docker' ($ComposeArguments + @('up', '-d', '--build', '--wait'))
    Invoke-Checked 'docker' ($ComposeArguments + @('exec', '-T', 'mysql', 'mysql', '-uroot', '-pcitywall_test_root', '-e', 'CREATE DATABASE city_wall_test CHARACTER SET utf8mb4;'))
    $FixtureSQL = "INSERT INTO users (username, password_hash, display_name, status) SELECT 'inspector_other', password_hash, '其他巡检员', 'ACTIVE' FROM users WHERE username='inspector'; INSERT INTO user_roles (user_id, role_code) SELECT id, 'INSPECTOR' FROM users WHERE username='inspector_other';"
    Invoke-Checked 'docker' ($ComposeArguments + @('exec', '-T', 'mysql', 'mysql', '-uroot', '-pcitywall_test_root', '-D', 'city_wall', '-e', $FixtureSQL))
    Invoke-Checked 'go' @('test', '-tags', 'integration', './internal/module/task', './internal/module/observation', '-count=1') (Join-Path $ProjectRoot 'server')
    foreach ($Scenario in @('e2e_scenarios.py', 'e2e_issue.py', 'e2e_idempotency.py', 'e2e_evidence.py', 'e2e_report.py')) {
        Invoke-Checked $Python @((Join-Path $PSScriptRoot $Scenario))
    }
    Write-Host "Acceptance checks passed. Test outputs: $TestOutput"
} catch {
    & docker @ComposeArguments logs --no-color --tail 100
    throw
} finally {
    & docker @ComposeArguments down --volumes
    foreach ($EnvironmentName in $EnvironmentNames) { [Environment]::SetEnvironmentVariable($EnvironmentName, $PreviousEnvironment[$EnvironmentName], 'Process') }
}
