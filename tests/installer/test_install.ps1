$ErrorActionPreference = "Stop"

function Assert-True {
    param(
        [bool]$Condition,
        [string]$Message
    )

    if (-not $Condition) {
        throw $Message
    }
}

$repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("sc-installer-test-" + [System.Guid]::NewGuid().ToString("N"))
$homeDir = Join-Path $tempRoot "home"
$binDir = Join-Path $tempRoot "bin"

New-Item -ItemType Directory -Force -Path $homeDir, $binDir | Out-Null

try {
    $env:HOME = $homeDir
    $env:USERPROFILE = $homeDir
    $env:SC_INSTALL_BIN_DIR = $binDir
    $env:PATH = $binDir + [System.IO.Path]::PathSeparator + $env:PATH
    $env:GOCACHE = Join-Path $tempRoot "go-cache"
    $env:GOMODCACHE = Join-Path $tempRoot "go-mod-cache"
    $env:GOFLAGS = "-modcacherw"
    New-Item -ItemType Directory -Force -Path $env:GOCACHE, $env:GOMODCACHE | Out-Null

    & (Join-Path $repoRoot "scripts/install.ps1")

    $binaryPath = Join-Path $binDir "sc.exe"
    $skillPath = Join-Path $homeDir ".claude/skills/sc-plugin/SKILL.md"
    $configPath = Join-Path $homeDir ".sc/config.toml"
    Assert-True (Test-Path -LiteralPath $binaryPath) "missing installed sc.exe"
    Assert-True (Test-Path -LiteralPath $skillPath) "missing installed skill"
    Assert-True (Test-Path -LiteralPath $configPath) "missing config.toml"

    $versionOutput = & $binaryPath --version
    Assert-True ($versionOutput -match "sc version ") "unexpected version output: $versionOutput"

    @(
        "[dolt]"
        'branch = "beta"'
    ) | Set-Content -LiteralPath $configPath
    Set-Content -LiteralPath $skillPath -Value "locally edited"
    Set-Content -LiteralPath (Join-Path $homeDir ".claude/skills/sc-plugin/USER-NOTES.md") -Value "user note"
    $agentDir = Join-Path $homeDir ".claude/agents"
    New-Item -ItemType Directory -Force -Path $agentDir | Out-Null
    Set-Content -LiteralPath (Join-Path $agentDir "my-agent.md") -Value "assistant instructions"
    Set-Content -LiteralPath $binaryPath -Value "stale"

    & (Join-Path $repoRoot "scripts/install.ps1")

    $configContents = Get-Content -LiteralPath $configPath -Raw
    Assert-True ($configContents -match 'branch = "beta"') "config was not preserved"
    $skillContents = Get-Content -LiteralPath $skillPath -Raw
    Assert-True ($skillContents -match "Thin Claude skill wrapper") "managed skill file was not refreshed"
    $notesContents = Get-Content -LiteralPath (Join-Path $homeDir ".claude/skills/sc-plugin/USER-NOTES.md") -Raw
    Assert-True ($notesContents -match "user note") "unmanaged file was removed"
    $agentContents = Get-Content -LiteralPath (Join-Path $agentDir "my-agent.md") -Raw
    Assert-True ($agentContents -match "assistant instructions") "unrelated file outside managed skill tree was modified"
    $versionOutput = & $binaryPath --version
    Assert-True ($versionOutput -match "sc version ") "unexpected version output after rerun: $versionOutput"
} finally {
    if (Test-Path -LiteralPath $tempRoot) {
        try {
            Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction Stop
        } catch {
            Write-Warning ("installer test cleanup skipped: " + $_.Exception.Message)
        }
    }
}
