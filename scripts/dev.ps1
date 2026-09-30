# ============================================================================
# dev.ps1 — 本地一键开发环境（Windows / PowerShell 5.1+）
# 用法：
#   .\scripts\dev.ps1 start   # 按序拉起 mysql(系统服务) → 迁移/seed → recognizer → server → web
#   .\scripts\dev.ps1 status  # 查看各端健康状态
#   .\scripts\dev.ps1 stop    # 停止本脚本启动的前端/服务端/识别服务
# 说明：
#   - MySQL 使用系统服务 MySQL84（3306，root/123456），未安装时给出指引。
#   - recognizer 依赖 venv（首次运行 scripts/setup_recognizer.ps1 安装）。
#   - 日志统一写到仓库根 logs/（不入库）。
# ============================================================================
param([string]$Action = "start")

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$LogDir = Join-Path $Root "logs"
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null

function Test-Port([int]$Port) {
    $tcp = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
    if ($tcp) { return $true }
    # 兜底：Get-NetTCPConnection 可能漏报 IPv6 监听（如 vite 绑 ::1）。
    return [bool](netstat -ano | Select-String (":$Port " + 'LISTENING'))
}

function Ensure-MySQL {
    # 系统服务 MySQL84（或 MySQL 服务）未运行则启动；无法启动时给出指引并退出。
    $svc = Get-Service -Name "MySQL84" -ErrorAction SilentlyContinue
    if (-not $svc) { $svc = Get-Service -Name "MySQL" -ErrorAction SilentlyContinue }
    if (-not $svc) {
        Write-Host "[mysql] 未找到 MySQL84/MySQL 服务。请先安装 MySQL 8（见 README），或手动启动后重试。" -ForegroundColor Red
        exit 1
    }
    if ($svc.Status -ne "Running") {
        Write-Host "[mysql] 启动服务 $($svc.Name) ..."
        Start-Service $svc.Name
        Start-Sleep -Seconds 4
    }
    if (-not (Test-Port 3306)) {
        Write-Host "[mysql] 3306 未监听，请检查 MySQL 服务状态。" -ForegroundColor Red
        exit 1
    }
    Write-Host "[mysql] OK @127.0.0.1:3306" -ForegroundColor Green
}

function Invoke-MigrateAndSeed {
    $env:GOPROXY = "https://goproxy.cn,direct"
    Push-Location (Join-Path $Root "server")
    try {
        # go 的日志输出到 stderr，在 PS 5.1 会渲染为 NativeCommandError；
        # 此处临时放开 ErrorActionPreference 避免被当作终止错误。
        $prevEAP = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        Write-Host "[server] 执行迁移 ..."
        & go run ./cmd/migrate -config ..\conf\server.json -dir migrations 2>&1 | Out-String | Write-Host
        if ($LASTEXITCODE -ne 0) { $ErrorActionPreference = $prevEAP; throw "migrate failed (exit $LASTEXITCODE)" }
        Write-Host "[server] seed ..."
        & go run ./cmd/seed -config ..\conf\server.json 2>&1 | Out-String | Write-Host
        if ($LASTEXITCODE -ne 0) { $ErrorActionPreference = $prevEAP; throw "seed failed (exit $LASTEXITCODE)" }
        $ErrorActionPreference = $prevEAP
    } finally {
        Pop-Location
    }
}

function Start-Recognizer {
    if (Test-Port 4100) { Write-Host "[recognizer] 已在运行 @4100" -ForegroundColor Green; return }
    $venv = Join-Path $Root "recognizer\.venv"
    $py = Join-Path $venv "Scripts\python.exe"
    if (-not (Test-Path $py)) {
        Write-Host "[recognizer] 未找到 venv，跳过（如需启动：cd recognizer; python -m venv .venv; .\.venv\Scripts\pip install -r requirements.txt）" -ForegroundColor Yellow
        return
    }
    Write-Host "[recognizer] 启动 uvicorn @4100 ..."
    Start-Process -FilePath $py -ArgumentList "-m","uvicorn","app.main:app","--host","127.0.0.1","--port","4100" -WorkingDirectory (Join-Path $Root "recognizer") -WindowStyle Hidden -RedirectStandardOutput (Join-Path $LogDir "recognizer.log") -RedirectStandardError (Join-Path $LogDir "recognizer.err.log")
}

function Start-Server {
    if (Test-Port 8080) {
        Write-Host "[server] 已在运行 @8080" -ForegroundColor Green
        return
    }
    Write-Host "[server] 启动 go server @8080 ..."
    $env:GOPROXY = "https://goproxy.cn,direct"
    Push-Location (Join-Path $Root "server")
    try {
        Start-Process -FilePath "go" -ArgumentList "run","cmd/server/main.go","-config","..\conf\server.json" -WorkingDirectory (Join-Path $Root "server") -WindowStyle Hidden -RedirectStandardOutput (Join-Path $LogDir "server.log") -RedirectStandardError (Join-Path $LogDir "server.err.log")
    } finally {
        Pop-Location
    }
    # 等待就绪
    for ($i = 0; $i -lt 30; $i++) {
        Start-Sleep -Seconds 1
        try {
            $ready = Invoke-RestMethod -Uri "http://127.0.0.1:8080/readyz" -TimeoutSec 2
            if ($ready.status -eq "ready") { Write-Host "[server] ready" -ForegroundColor Green; return }
        } catch { }
    }
    Write-Host "[server] 启动超时，请查看 logs/server.err.log" -ForegroundColor Yellow
}

function Start-Web {
    if (Test-Port 5173) { Write-Host "[web] 已在运行 @5173" -ForegroundColor Green; return }
    if (-not (Test-Path (Join-Path $Root "web\node_modules"))) {
        Write-Host "[web] node_modules 缺失，执行 npm ci ..."
        Push-Location (Join-Path $Root "web")
        try { & npm ci 2>&1 | Out-String | Write-Host } finally { Pop-Location }
    }
    Write-Host "[web] 启动 vite @5173 ..."
    Push-Location (Join-Path $Root "web")
    try {
        Start-Process -FilePath "npm.cmd" -ArgumentList "run","dev" -WorkingDirectory (Join-Path $Root "web") -WindowStyle Hidden -RedirectStandardOutput (Join-Path $LogDir "web.log") -RedirectStandardError (Join-Path $LogDir "web.err.log")
    } finally {
        Pop-Location
    }
}

function Show-Status {
    $ports = @{ "MySQL(3306)" = 3306; "Recognizer(4100)" = 4100; "Server(8080)" = 8080; "Web(5173)" = 5173 }
    foreach ($k in $ports.Keys) {
        $open = Test-Port $ports[$k]
        Write-Host ("{0,-20} {1}" -f $k, ($(if ($open) { "UP" } else { "DOWN" })))
    }
    Write-Host "日志目录: $LogDir"
}

function Stop-All {
    # 停止 web(vite) 与 server(go run) 进程：按端口反查 PID。
    foreach ($p in @(5173, 8080, 4100)) {
        $conn = Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($conn) {
            Write-Host "[stop] 端口 $p -> PID $($conn.OwningProcess)"
            Stop-Process -Id $conn.OwningProcess -Force
        }
    }
    Write-Host "[stop] 完成（MySQL 系统服务保持运行）"
}

switch ($Action.ToLower()) {
    "start" {
        Ensure-MySQL
        Invoke-MigrateAndSeed
        Start-Recognizer
        Start-Server
        Start-Web
        # 等待 vite 完成端口监听，避免状态误报 DOWN。
        Start-Sleep -Seconds 3
        Show-Status
        Write-Host "`n演示账号: admin/Admin123, reviewer/Review123, inspector/Insp123"
        Write-Host "PC 管理端: http://localhost:5173"
    }
    "status" { Show-Status }
    "stop" { Stop-All }
    default {
        Write-Host "用法: .\scripts\dev.ps1 start|status|stop"
        exit 1
    }
}
