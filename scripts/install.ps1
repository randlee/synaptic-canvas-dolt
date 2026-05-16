$ErrorActionPreference = "Stop"

function Remove-EmptyParents {
    param(
        [string]$Start,
        [string]$Stop
    )

    $current = $Start
    while ($current -and $current -ne $Stop) {
        if (-not (Test-Path -LiteralPath $current)) {
            break
        }
        $children = Get-ChildItem -LiteralPath $current -Force
        if ($children.Count -gt 0) {
            break
        }
        Remove-Item -LiteralPath $current -Force
        $parent = Split-Path -Parent $current
        if ($parent -eq $current) {
            break
        }
        $current = $parent
    }
}

function Write-Note {
    param([string]$Message)
    Write-Host $Message
}

function Add-PathEntry {
    param([string]$BinDir)

    $separator = [System.IO.Path]::PathSeparator
    $pathEntries = @()
    if ($env:PATH) {
        $pathEntries = $env:PATH -split [System.IO.Path]::PathSeparator | Where-Object { $_ }
    }
    if (-not ($pathEntries -contains $BinDir)) {
        $env:PATH = if ($env:PATH) { $BinDir + $separator + $env:PATH } else { $BinDir }
    }

    $userPath = [Environment]::GetEnvironmentVariable("PATH", [EnvironmentVariableTarget]::User)
    $userEntries = @()
    if ($userPath) {
        $userEntries = $userPath -split [System.IO.Path]::PathSeparator | Where-Object { $_ }
    }
    if (-not ($userEntries -contains $BinDir)) {
        $updatedUserPath = if ($userPath) { $userPath + $separator + $BinDir } else { $BinDir }
        [Environment]::SetEnvironmentVariable("PATH", $updatedUserPath, [EnvironmentVariableTarget]::User)
    }
}

function Get-UniqueSiblingPath {
    param(
        [string]$Parent,
        [string]$Prefix
    )

    do {
        $candidate = Join-Path $Parent ($Prefix + [System.Guid]::NewGuid().ToString("N"))
    } while (Test-Path -LiteralPath $candidate)
    return $candidate
}

function Test-SimulatedSkillCopyFailure {
    $threshold = 0
    if ($env:SC_INSTALL_TEST_FAIL_AFTER_SKILL_COPY) {
        [void][int]::TryParse($env:SC_INSTALL_TEST_FAIL_AFTER_SKILL_COPY, [ref]$threshold)
    }
    if ($threshold -le 0) {
        return
    }
    $count = 0
    if ($env:SC_INSTALL_SKILL_COPY_COUNT) {
        [void][int]::TryParse($env:SC_INSTALL_SKILL_COPY_COUNT, [ref]$count)
    }
    $count++
    $env:SC_INSTALL_SKILL_COPY_COUNT = [string]$count
    if ($count -ge $threshold) {
        throw "simulated skill copy failure after $count managed file copies"
    }
}

$repoRoot = if ($env:SC_INSTALL_REPO_ROOT) { $env:SC_INSTALL_REPO_ROOT } else { Split-Path -Parent $PSScriptRoot }
$srcRoot = Join-Path $repoRoot "src"
$skillSource = Join-Path $repoRoot ".claude/skills/sc-plugin"
$homeDir = if ($env:SC_INSTALL_HOME) {
    $env:SC_INSTALL_HOME
} elseif ($env:HOME) {
    $env:HOME
} elseif ($env:USERPROFILE) {
    $env:USERPROFILE
} else {
    throw "HOME or USERPROFILE must be set"
}

if (-not (Test-Path -LiteralPath $srcRoot)) {
    throw "expected Go source at $srcRoot"
}
if (-not (Test-Path -LiteralPath $skillSource)) {
    throw "expected sc:plugin source at $skillSource"
}

$binDir = if ($env:SC_INSTALL_BIN_DIR) {
    $env:SC_INSTALL_BIN_DIR
} else {
    Join-Path $homeDir "AppData/Local/Programs/SynapticCanvas/bin"
}
$binPath = Join-Path $binDir "sc.exe"
$configDir = Join-Path $homeDir ".sc"
$configPath = Join-Path $configDir "config.toml"
$stateRoot = Join-Path $homeDir ".synaptic/installers/sc-plugin"
$skillTarget = Join-Path $homeDir ".claude/skills/sc-plugin"
$skillParent = Split-Path -Parent $skillTarget
$skillManifest = Join-Path $stateRoot "managed-files.txt"
$binaryManifest = Join-Path $stateRoot "binary-path.txt"
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("sc-install-" + [System.Guid]::NewGuid().ToString("N"))
$stagedSkill = $null
$backupSkill = $null

New-Item -ItemType Directory -Force -Path $binDir, $configDir, $stateRoot, $skillParent, $tempRoot | Out-Null
try {
    $buildPath = if ($env:SC_INSTALL_BINARY) {
        $env:SC_INSTALL_BINARY
    } else {
        $built = Join-Path $tempRoot "sc.exe"
        Push-Location $srcRoot
        try {
            & go build -o $built .
        } finally {
            Pop-Location
        }
        $built
    }

    if (-not (Test-Path -LiteralPath $buildPath)) {
        throw "expected built binary at $buildPath"
    }

    if (Test-Path -LiteralPath $binaryManifest) {
        $previousBin = (Get-Content -LiteralPath $binaryManifest -ErrorAction SilentlyContinue | Select-Object -First 1)
        if ($previousBin -and $previousBin -ne $binPath -and (Test-Path -LiteralPath $previousBin)) {
            Remove-Item -LiteralPath $previousBin -Force
        }
    }
    Copy-Item -LiteralPath $buildPath -Destination $binPath -Force
    Set-Content -LiteralPath $binaryManifest -Value $binPath -NoNewline

    if (-not (Test-Path -LiteralPath $configPath)) {
        @(
            "# Synaptic Canvas CLI configuration"
            "# User-owned file: installer creates it once and preserves later edits."
        ) | Set-Content -LiteralPath $configPath
    }

    $sourceFiles = Get-ChildItem -LiteralPath $skillSource -File -Recurse | ForEach-Object {
        $_.FullName.Substring($skillSource.Length + 1).Replace("\", "/")
    } | Sort-Object
    $sourceSet = @{}
    foreach ($rel in $sourceFiles) {
        $sourceSet[$rel] = $true
    }

    $stagedSkill = Get-UniqueSiblingPath -Parent $skillParent -Prefix ".sc-plugin.stage."
    New-Item -ItemType Directory -Force -Path $stagedSkill | Out-Null
    if (Test-Path -LiteralPath $skillTarget) {
        Get-ChildItem -LiteralPath $skillTarget -Force | ForEach-Object {
            Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $stagedSkill $_.Name) -Recurse -Force
        }
    }

    if (Test-Path -LiteralPath $skillManifest) {
        foreach ($rel in Get-Content -LiteralPath $skillManifest) {
            if (-not $rel) {
                continue
            }
            if (-not $sourceSet.ContainsKey($rel)) {
                $targetFile = Join-Path $stagedSkill ($rel -replace "/", [System.IO.Path]::DirectorySeparatorChar)
                if (Test-Path -LiteralPath $targetFile) {
                    Remove-Item -LiteralPath $targetFile -Force
                    Remove-EmptyParents -Start (Split-Path -Parent $targetFile) -Stop $stagedSkill
                }
            }
        }
    }

    foreach ($rel in $sourceFiles) {
        $sourceFile = Join-Path $skillSource ($rel -replace "/", [System.IO.Path]::DirectorySeparatorChar)
        $targetFile = Join-Path $stagedSkill ($rel -replace "/", [System.IO.Path]::DirectorySeparatorChar)
        $targetDir = Split-Path -Parent $targetFile
        New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
        Copy-Item -LiteralPath $sourceFile -Destination $targetFile -Force
        Test-SimulatedSkillCopyFailure
    }

    if (Test-Path -LiteralPath $skillTarget) {
        $backupSkill = Get-UniqueSiblingPath -Parent $skillParent -Prefix ".sc-plugin.backup."
        Move-Item -LiteralPath $skillTarget -Destination $backupSkill
    }
    try {
        Move-Item -LiteralPath $stagedSkill -Destination $skillTarget -Force
        $stagedSkill = $null
        if ($backupSkill -and (Test-Path -LiteralPath $backupSkill)) {
            Remove-Item -LiteralPath $backupSkill -Recurse -Force
            $backupSkill = $null
        }
    } catch {
        if ($backupSkill -and (Test-Path -LiteralPath $backupSkill) -and -not (Test-Path -LiteralPath $skillTarget)) {
            Move-Item -LiteralPath $backupSkill -Destination $skillTarget
            $backupSkill = $null
        }
        throw
    }
    Set-Content -LiteralPath $skillManifest -Value $sourceFiles

    Add-PathEntry -BinDir $binDir

    Write-Note "Installed sc to $binPath"
    Write-Note "Installed sc:plugin to $skillTarget"
    Write-Note "Preserved config at $configPath"
} finally {
    if ($backupSkill -and (Test-Path -LiteralPath $backupSkill) -and -not (Test-Path -LiteralPath $skillTarget)) {
        try {
            Move-Item -LiteralPath $backupSkill -Destination $skillTarget
        } catch {
            Write-Warning ("installer backup restore skipped: " + $_.Exception.Message)
        }
    }
    if ($stagedSkill -and (Test-Path -LiteralPath $stagedSkill)) {
        try {
            Remove-Item -LiteralPath $stagedSkill -Recurse -Force -ErrorAction Stop
        } catch {
            Write-Warning ("installer stage cleanup skipped: " + $_.Exception.Message)
        }
    }
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force
    }
}
