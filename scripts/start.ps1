param([switch]$SkipBuild)
$ErrorActionPreference = 'Stop'
$ProjectRoot = Split-Path -Parent $PSScriptRoot
Push-Location $ProjectRoot
try {
    if (-not (Test-Path -LiteralPath '.env')) {
        Copy-Item -LiteralPath '.env.example' -Destination '.env'
    }
    $ModelConfig = Get-Content -Raw -LiteralPath 'conf/recognizer.json' | ConvertFrom-Json
    $WeightManifest = Get-Content -Raw -LiteralPath 'data/yolo/weights/manifest.json' | ConvertFrom-Json
    if ($ModelConfig.recognition.model_version -ne $WeightManifest.model_version) {
        throw 'Recognizer model version does not match the weight manifest'
    }
    $WeightPaths = @{
        primary = 'data/yolo/weights/best.pt'
        specialist = $ModelConfig.recognition.specialist_weights_path
        spalling = $ModelConfig.recognition.spalling_weights_path
    }
    foreach ($Member in @('primary', 'specialist', 'spalling')) {
        if (-not $WeightPaths[$Member]) { continue }
        $Entry = $WeightManifest.$Member
        if (-not $Entry -or -not $Entry.file -or -not $Entry.sha256) { throw "Missing $Member in weight manifest" }
        $WeightPath = Join-Path 'data/yolo/weights' $Entry.file
        $ConfiguredPath = $WeightPaths[$Member]
        if (-not [IO.Path]::IsPathRooted($ConfiguredPath)) { $ConfiguredPath = Join-Path $ProjectRoot $ConfiguredPath }
        if ([IO.Path]::GetFullPath((Join-Path $ProjectRoot $WeightPath)) -ne [IO.Path]::GetFullPath($ConfiguredPath)) {
            throw "$Member weight path does not match the manifest"
        }
        if (-not (Test-Path -LiteralPath $WeightPath -PathType Leaf)) { throw "Missing model weight: $WeightPath" }
        if ((Get-FileHash -LiteralPath $WeightPath -Algorithm SHA256).Hash -ne $Entry.sha256) {
            throw "Model weight hash mismatch: $WeightPath"
        }
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
