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
$skillManifest = Join-Path $stateRoot "managed-files.txt"
$binaryManifest = Join-Path $stateRoot "binary-path.txt"
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("sc-install-" + [System.Guid]::NewGuid().ToString("N"))

New-Item -ItemType Directory -Force -Path $binDir, $configDir, $stateRoot, $skillTarget, $tempRoot | Out-Null
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

    if (Test-Path -LiteralPath $skillManifest) {
        foreach ($rel in Get-Content -LiteralPath $skillManifest) {
            if (-not $rel) {
                continue
            }
            if (-not $sourceSet.ContainsKey($rel)) {
                $targetFile = Join-Path $skillTarget ($rel -replace "/", [System.IO.Path]::DirectorySeparatorChar)
                if (Test-Path -LiteralPath $targetFile) {
                    Remove-Item -LiteralPath $targetFile -Force
                    Remove-EmptyParents -Start (Split-Path -Parent $targetFile) -Stop $skillTarget
                }
            }
        }
    }

    foreach ($rel in $sourceFiles) {
        $sourceFile = Join-Path $skillSource ($rel -replace "/", [System.IO.Path]::DirectorySeparatorChar)
        $targetFile = Join-Path $skillTarget ($rel -replace "/", [System.IO.Path]::DirectorySeparatorChar)
        $targetDir = Split-Path -Parent $targetFile
        New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
        Copy-Item -LiteralPath $sourceFile -Destination $targetFile -Force
    }
    Set-Content -LiteralPath $skillManifest -Value $sourceFiles

    if (-not (($env:PATH -split [System.IO.Path]::PathSeparator) -contains $binDir)) {
        Write-Note "warning: $binDir is not currently on PATH"
    }

    Write-Note "Installed sc to $binPath"
    Write-Note "Installed sc:plugin to $skillTarget"
    Write-Note "Preserved config at $configPath"
} finally {
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force
    }
}
