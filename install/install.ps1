param(
    [switch]$Codex,
    [switch]$Claude,
    [switch]$All,
    [string]$Version = "",
    [switch]$LocalSource,
    [switch]$LocalBuild
)

$ErrorActionPreference = "Stop"

$RepoOwner = if ($env:SSH_OPS_REPO_OWNER) { $env:SSH_OPS_REPO_OWNER } else { "HanBBQovo" }
$RepoName = if ($env:SSH_OPS_REPO_NAME) { $env:SSH_OPS_REPO_NAME } else { "ssh-ops" }
$BinDir = if ($env:SSH_OPS_BIN_DIR) { $env:SSH_OPS_BIN_DIR } else { Join-Path $HOME ".local\bin" }
$ConfigDir = if ($env:SSH_OPS_CONFIG_DIR) { $env:SSH_OPS_CONFIG_DIR } else { Join-Path $HOME ".config\ssh-ops" }
$CodexSkillsDir = if ($env:CODEX_HOME) { Join-Path $env:CODEX_HOME "skills" } else { Join-Path $HOME ".codex\skills" }
$ClaudeSkillsDir = if ($env:CLAUDE_HOME) { Join-Path $env:CLAUDE_HOME "skills" } else { Join-Path $HOME ".claude\skills" }

function Log([string]$Message) {
    Write-Host "[ssh-ops] $Message"
}

function Detect-Arch {
    switch ($env:PROCESSOR_ARCHITECTURE.ToLower()) {
        "amd64" { return "amd64" }
        "arm64" { return "arm64" }
        default { throw "当前架构不受支持：$env:PROCESSOR_ARCHITECTURE" }
    }
}

function Ensure-LatestVersion {
    param([string]$InputVersion)
    if (($LocalSource -or $LocalBuild) -and -not $InputVersion) {
        return "dev"
    }
    if ($InputVersion) {
        return $InputVersion
    }
    $apiUrl = "https://api.github.com/repos/$RepoOwner/$RepoName/releases/latest"
    $release = Invoke-RestMethod -Uri $apiUrl
    if (-not $release.tag_name) {
        throw "无法解析最新 release 版本"
    }
    return $release.tag_name
}

function Ensure-PathContainsBinDir {
    if ($env:PATH -split ";" | Where-Object { $_ -eq $BinDir }) {
        return
    }
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (-not $userPath) {
        $userPath = $BinDir
    } elseif (($userPath -split ";") -notcontains $BinDir) {
        $userPath = "$userPath;$BinDir"
    }
    [Environment]::SetEnvironmentVariable("Path", $userPath, "User")
    Log "已将 $BinDir 写入用户 PATH，新开的终端会生效"
}

function Copy-Skill {
    param(
        [string]$SourceDir,
        [string]$TargetRoot
    )
    $targetDir = Join-Path $TargetRoot "ssh-ops"
    New-Item -ItemType Directory -Force -Path $TargetRoot | Out-Null
    if (Test-Path $targetDir) {
        Remove-Item -Recurse -Force $targetDir
    }
    Copy-Item -Recurse -Force $SourceDir $targetDir
}

if (-not $Codex -and -not $Claude -and -not $All) {
    $Codex = $true
}
if ($All) {
    $Codex = $true
    $Claude = $true
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent $scriptDir
$localSkillDir = Join-Path $repoRoot "skills\ssh-ops"
$localConfigExample = Join-Path $repoRoot "examples\config.example.yaml"

if (($LocalSource -or $LocalBuild) -and -not (Test-Path $localSkillDir)) {
    throw "--LocalSource / --LocalBuild 需要在仓库目录里运行"
}

$versionValue = Ensure-LatestVersion -InputVersion $Version
$arch = Detect-Arch
$tmpDir = Join-Path ([IO.Path]::GetTempPath()) ("ssh-ops-install-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null

try {
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    $binTarget = Join-Path $BinDir "sshctl.exe"

    if ($LocalBuild) {
        if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
            throw "--LocalBuild 需要本地已安装 Go"
        }
        Push-Location $repoRoot
        try {
            & go build "-ldflags" "-X main.version=$versionValue" "-o" $binTarget "./cmd/sshctl"
        } finally {
            Pop-Location
        }
        $skillSourceDir = $localSkillDir
        $configExampleFile = $localConfigExample
    } else {
        $binaryAsset = "ssh-ops_windows_$arch.zip"
        $binaryUrl = "https://github.com/$RepoOwner/$RepoName/releases/download/$versionValue/$binaryAsset"
        $binaryZip = Join-Path $tmpDir $binaryAsset
        Invoke-WebRequest -Uri $binaryUrl -OutFile $binaryZip
        Expand-Archive -Path $binaryZip -DestinationPath (Join-Path $tmpDir "bin") -Force
        Copy-Item -Force (Join-Path $tmpDir "bin\sshctl.exe") $binTarget

        if ($LocalSource) {
            $skillSourceDir = $localSkillDir
            $configExampleFile = $localConfigExample
        } else {
            $skillAsset = "ssh-ops-skill.zip"
            $skillUrl = "https://github.com/$RepoOwner/$RepoName/releases/download/$versionValue/$skillAsset"
            $skillZip = Join-Path $tmpDir $skillAsset
            Invoke-WebRequest -Uri $skillUrl -OutFile $skillZip
            Expand-Archive -Path $skillZip -DestinationPath (Join-Path $tmpDir "skill") -Force
            $skillSourceDir = Join-Path $tmpDir "skill\ssh-ops-skill\skills\ssh-ops"
            $configExampleFile = Join-Path $tmpDir "skill\ssh-ops-skill\examples\config.example.yaml"
        }
    }

    if ($Codex) {
        Copy-Skill -SourceDir $skillSourceDir -TargetRoot $CodexSkillsDir
        Log "已安装到 Codex：$(Join-Path $CodexSkillsDir 'ssh-ops')"
    }
    if ($Claude) {
        Copy-Skill -SourceDir $skillSourceDir -TargetRoot $ClaudeSkillsDir
        Log "已安装到 Claude Code：$(Join-Path $ClaudeSkillsDir 'ssh-ops')"
    }

    New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null
    $configTarget = Join-Path $ConfigDir "config.yaml"
    if (-not (Test-Path $configTarget)) {
        Copy-Item -Force $configExampleFile $configTarget
        Log "已创建默认配置：$configTarget"
    } else {
        Log "已保留现有配置：$configTarget"
    }

    Ensure-PathContainsBinDir

    Write-Host ""
    Write-Host "安装完成"
    Write-Host "- 版本：$versionValue"
    Write-Host "- 二进制：$binTarget"
    Write-Host "- 配置目录：$ConfigDir"
    Write-Host ""
    Write-Host "下一步："
    Write-Host "1. 编辑 $configTarget"
    Write-Host "2. 运行：sshctl validate-config --pretty"
    Write-Host "3. 重启 Codex / Claude Code，或开启一个新会话"
} finally {
    if (Test-Path $tmpDir) {
        Remove-Item -Recurse -Force $tmpDir
    }
}
