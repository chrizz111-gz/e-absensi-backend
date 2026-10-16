# =============================================================================
# gen-secrets.ps1 - Isi JWT_SECRET & JWT_REFRESH_SECRET di .env dengan nilai acak.
# Jika .env belum ada, file dibuat dari .env.example.
#
# Pemakaian (dari root repo):
#   Copy-Item .env.example .env
#   ./scripts/gen-secrets.ps1
# =============================================================================

$ErrorActionPreference = "Stop"

$root        = Split-Path -Parent $PSScriptRoot
$envPath     = Join-Path $root ".env"
$examplePath = Join-Path $root ".env.example"

if (-not (Test-Path -LiteralPath $envPath)) {
    if (-not (Test-Path -LiteralPath $examplePath)) {
        throw "Tidak menemukan .env.example di $root"
    }
    Copy-Item -LiteralPath $examplePath -Destination $envPath
    Write-Host "Membuat .env dari .env.example"
}

function New-Secret {
    $bytes = New-Object byte[] 48
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    return [Convert]::ToBase64String($bytes)
}

$accessSecret  = New-Secret
$refreshSecret = New-Secret

$lines       = Get-Content -LiteralPath $envPath
$seenAccess  = $false
$seenRefresh = $false

for ($i = 0; $i -lt $lines.Count; $i++) {
    if ($lines[$i] -match '^\s*JWT_SECRET\s*=') {
        $lines[$i] = "JWT_SECRET=$accessSecret"
        $seenAccess = $true
    }
    elseif ($lines[$i] -match '^\s*JWT_REFRESH_SECRET\s*=') {
        $lines[$i] = "JWT_REFRESH_SECRET=$refreshSecret"
        $seenRefresh = $true
    }
}

if (-not $seenAccess)  { $lines += "JWT_SECRET=$accessSecret" }
if (-not $seenRefresh) { $lines += "JWT_REFRESH_SECRET=$refreshSecret" }

$utf8 = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllLines($envPath, $lines, $utf8)

Write-Host "Secret JWT ditulis ke .env ($envPath)"
