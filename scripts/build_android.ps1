param([switch]$SkipWebBuild)
$ErrorActionPreference = 'Stop'
$ProjectRoot = Split-Path -Parent $PSScriptRoot
if (-not $env:JAVA_HOME -or -not $env:ANDROID_HOME) {
    throw 'Set JAVA_HOME to JDK 17 and ANDROID_HOME to Android SDK first.'
}
Push-Location $ProjectRoot
try {
    if (-not $SkipWebBuild) {
        Push-Location miniapp
        try {
            & npm run build:h5
            if ($LASTEXITCODE -ne 0) { throw 'H5 build failed' }
        } finally { Pop-Location }
    }
    $Source = Join-Path $ProjectRoot 'miniapp\dist\build\h5'
    if (-not (Test-Path -LiteralPath (Join-Path $Source 'index.html'))) { throw 'H5 output is missing' }
    $Destination = Join-Path $ProjectRoot 'android\app\src\main\assets\www'
    if ([IO.Path]::GetFullPath($Destination) -ne "$ProjectRoot\android\app\src\main\assets\www") {
        throw 'Unexpected assets destination'
    }
    New-Item -ItemType Directory -Path $Destination -Force | Out-Null
    & robocopy $Source $Destination /MIR /NJH /NJS /NFL /NDL /NP
    if ($LASTEXITCODE -gt 7) { throw 'Copying H5 assets failed' }
    Push-Location android
    try {
        & .\gradlew.bat assembleDebug
        if ($LASTEXITCODE -ne 0) { throw 'Android build failed' }
    } finally { Pop-Location }
    Write-Host (Join-Path $ProjectRoot 'android\app\build\outputs\apk\debug\app-debug.apk')
} finally { Pop-Location }
