[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^[0-9A-Za-z][0-9A-Za-z._-]{0,63}$')]
    [string]$Version,

    [string]$DiaRoot = '',
    [string]$IntegrationConfig = '',
    [switch]$SkipTests,
    [switch]$SkipNpmCi,
    [switch]$SkipDatabaseGenerate
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$oldGoToolchain = $env:GOTOOLCHAIN
$env:GOTOOLCHAIN = 'auto'

$repoRoot = Split-Path -Parent $PSScriptRoot
if (-not $DiaRoot) {
    $DiaRoot = Join-Path (Split-Path -Parent $repoRoot) 'dia-platform'
}
$repoRoot = (Resolve-Path -LiteralPath $repoRoot).Path
$DiaRoot = (Resolve-Path -LiteralPath $DiaRoot).Path
if (-not $IntegrationConfig) {
    $IntegrationConfig = Join-Path $repoRoot '.local\camie-integration\portal-static.yaml'
}

$releaseBase = Join-Path $repoRoot '.local\releases'
$workRoot = Join-Path $releaseBase 'work'
$packageName = "camie-release-$Version"
$stage = Join-Path $workRoot $packageName
$archive = Join-Path $releaseBase "$packageName-linux-amd64.tar.gz"
$archiveHash = "$archive.sha256"

function Invoke-Native {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string[]]$Arguments,
        [Parameter(Mandatory = $true)][string]$WorkingDirectory
    )
    Push-Location -LiteralPath $WorkingDirectory
    try {
        & $FilePath @Arguments
        if ($LASTEXITCODE -ne 0) {
            throw "$FilePath $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
        }
    }
    finally {
        Pop-Location
    }
}

function Get-NativeText {
    param([string]$FilePath, [string[]]$Arguments, [string]$WorkingDirectory)
    Push-Location -LiteralPath $WorkingDirectory
    try {
        # Preserve leading porcelain status columns; only trailing line breaks are noise.
        $text = (& $FilePath @Arguments 2>&1 | Out-String).TrimEnd()
        if ($LASTEXITCODE -ne 0) { throw "$FilePath $($Arguments -join ' ') failed: $text" }
        return $text
    }
    finally { Pop-Location }
}

function Assert-CleanSources {
    $portalStatus = Get-NativeText git @('-C', $repoRoot, 'status', '--porcelain', '--untracked-files=normal') $repoRoot
    if ($portalStatus) {
        throw "portal-static working tree is not clean. Commit the release first:`n$portalStatus"
    }

    $diaStatus = Get-NativeText git @('-C', $DiaRoot, 'status', '--porcelain', '--untracked-files=normal', '--', 'business/portal', 'business/member') $DiaRoot
    if ($diaStatus) {
        $unexpected = @($diaStatus -split "`r?`n" | Where-Object {
            $path = if ($_.Length -gt 3) { $_.Substring(3).Replace('\', '/') } else { $_ }
            $path -notin @('business/portal/backend/config.yaml', 'business/member/backend/config.yaml')
        })
        if ($unexpected.Count -gt 0) {
            throw "dia-platform portal/member source has uncommitted release changes:`n$($unexpected -join "`n")"
        }
        Write-Warning "Ignoring local backend config changes; release uses deploy/camie config templates.`n$diaStatus"
    }
}

function Assert-ElfAmd64 {
    param([string]$Path)
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    if ($bytes.Length -lt 20 -or $bytes[0] -ne 0x7f -or $bytes[1] -ne 0x45 -or $bytes[2] -ne 0x4c -or $bytes[3] -ne 0x46) {
        throw "$Path is not an ELF binary"
    }
    $machine = [BitConverter]::ToUInt16($bytes, 18)
    if ($machine -ne 0x3e) { throw "$Path is not an x86-64 ELF binary (e_machine=$machine)" }
}

function Normalize-Lf {
    param([string]$Text)
    return ($Text -replace "`r`n", "`n") -replace "`r", "`n"
}

function To-Base64Utf8 {
    param([string]$Text)
    return [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((Normalize-Lf $Text)))
}

function Copy-DirectoryContent {
    param([string]$Source, [string]$Destination)
    New-Item -ItemType Directory -Path $Destination -Force | Out-Null
    Get-ChildItem -LiteralPath $Source -Force | ForEach-Object {
        Copy-Item -LiteralPath $_.FullName -Destination $Destination -Recurse -Force
    }
}

function Write-Utf8NoBom {
    param([string]$Path, [string]$Value)
    [System.IO.File]::WriteAllText($Path, $Value, [System.Text.UTF8Encoding]::new($false))
}

foreach ($tool in @('git', 'go', 'node', 'npm.cmd', 'tar')) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
        throw "Required build tool is missing: $tool"
    }
}

Assert-CleanSources

$portalCommit = Get-NativeText git @('-C', $repoRoot, 'rev-parse', 'HEAD') $repoRoot
$diaCommit = Get-NativeText git @('-C', $DiaRoot, 'rev-parse', 'HEAD') $DiaRoot
$releaseTag = "camie-release-$Version"
$portalTagText = Get-NativeText git @('-C', $repoRoot, 'tag', '--points-at', 'HEAD') $repoRoot
$diaTagText = Get-NativeText git @('-C', $DiaRoot, 'tag', '--points-at', 'HEAD') $DiaRoot
$portalTags = @($portalTagText -split "`r?`n")
$diaTags = @($diaTagText -split "`r?`n")
if ($releaseTag -notin $portalTags) { throw "portal-static HEAD must be tagged $releaseTag before packaging" }
if ($releaseTag -notin $diaTags) { throw "dia-platform HEAD must be tagged $releaseTag before packaging" }
$nodeVersion = Get-NativeText node @('--version') $repoRoot
$npmVersion = Get-NativeText npm.cmd @('--version') $repoRoot
$staticGoVersion = Get-NativeText go @('env', 'GOVERSION') $repoRoot
$diaGoVersion = Get-NativeText go @('env', 'GOVERSION') (Join-Path $DiaRoot 'business\portal\backend')

$diaVersionText = $diaGoVersion -replace '^go', ''
if ([version]$diaVersionText -lt [version]'1.27.1') {
    throw "dia-platform requires Go 1.27.1+, but the selected toolchain is $diaGoVersion. Install it on the build machine or enable GOTOOLCHAIN=auto."
}

if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force }
New-Item -ItemType Directory -Path $stage, (Join-Path $stage 'bin'), (Join-Path $stage 'web'), (Join-Path $stage 'share'), (Join-Path $stage 'db') -Force | Out-Null

if (-not $SkipTests) {
    Invoke-Native go @('test', './...') $repoRoot
    Invoke-Native go @('vet', './...') $repoRoot
    foreach ($backend in @('business\portal\backend', 'business\member\backend')) {
        $dir = Join-Path $DiaRoot $backend
        Invoke-Native go @('test', './...') $dir
        Invoke-Native go @('vet', './...') $dir
    }
}

foreach ($frontend in @(
    @{ Name = 'business_portal'; Dir = 'business\portal\frontend' },
    @{ Name = 'business_member'; Dir = 'business\member\frontend' }
)) {
    $dir = Join-Path $DiaRoot $frontend.Dir
    if (-not $SkipNpmCi) { Invoke-Native npm.cmd @('ci') $dir }
    Invoke-Native npm.cmd @('run', 'build') $dir
    Copy-DirectoryContent (Join-Path $dir 'dist') (Join-Path $stage "web\$($frontend.Name)")
}

if (-not $SkipDatabaseGenerate) {
    if (-not (Test-Path -LiteralPath $IntegrationConfig)) {
        throw "Integration config not found: $IntegrationConfig. Supply -IntegrationConfig or explicitly use -SkipDatabaseGenerate."
    }
    if (-not $env:CAMIE_DB_DSN -or -not $env:CAMIE_STATIC_TOKEN) {
        throw 'CAMIE_DB_DSN and CAMIE_STATIC_TOKEN must be set for the real-database generation gate.'
    }
    Invoke-Native go @('run', './cmd/portal-static', 'generate', '--config', $IntegrationConfig) $repoRoot
}

$oldGoos = $env:GOOS
$oldGoarch = $env:GOARCH
$oldCgo = $env:CGO_ENABLED
try {
    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'
    Invoke-Native go @('build', '-trimpath', '-ldflags=-s -w', '-o', (Join-Path $stage 'bin\portal-static'), './cmd/portal-static') $repoRoot
    Invoke-Native go @('build', '-trimpath', '-ldflags=-s -w', '-o', (Join-Path $stage 'bin\portal'), '.') (Join-Path $DiaRoot 'business\portal\backend')
    Invoke-Native go @('build', '-trimpath', '-ldflags=-s -w', '-o', (Join-Path $stage 'bin\member'), '.') (Join-Path $DiaRoot 'business\member\backend')
}
finally {
    $env:GOOS = $oldGoos
    $env:GOARCH = $oldGoarch
    $env:CGO_ENABLED = $oldCgo
}

foreach ($binary in @('portal', 'member', 'portal-static')) {
    Assert-ElfAmd64 (Join-Path $stage "bin\$binary")
}

Copy-Item -LiteralPath (Join-Path $repoRoot 'deploy\camie') -Destination (Join-Path $stage 'ops') -Recurse -Force
Copy-DirectoryContent (Join-Path $repoRoot 'internal\adapters\camie\testdata\site') (Join-Path $stage 'share\camie-site')
Copy-DirectoryContent (Join-Path $repoRoot 'internal\adapters\camie\testdata\templates') (Join-Path $stage 'share\camie-preview-templates')

$templateRoot = Join-Path $repoRoot 'internal\adapters\camie\testdata\templates'
$layout = Get-Content -LiteralPath (Join-Path $templateRoot 'layout.html.tmpl') -Raw -Encoding utf8
$homeTemplate = Get-Content -LiteralPath (Join-Path $templateRoot 'home.html.tmpl') -Raw -Encoding utf8
$list = Get-Content -LiteralPath (Join-Path $templateRoot 'list.html.tmpl') -Raw -Encoding utf8
$article = Get-Content -LiteralPath (Join-Path $templateRoot 'article.html.tmpl') -Raw -Encoding utf8
$aboutBase = Get-Content -LiteralPath (Join-Path $templateRoot 'about.html.tmpl') -Raw -Encoding utf8
$paginationMarker = '{{define "pagination"}}'
$markerIndex = $list.IndexOf($paginationMarker, [StringComparison]::Ordinal)
if ($markerIndex -lt 0) { throw "list template is missing $paginationMarker" }
$sectionList = $list.Substring(0, $markerIndex).TrimEnd()

$sources = [ordered]@{
    LAYOUT = $layout
    HOME = $homeTemplate
    PARTY = $sectionList.Replace('{{define "list"}}', '{{define "party-list"}}')
    MINISTRY = $sectionList.Replace('{{define "list"}}', '{{define "ministry-list"}}')
    NEWS = $sectionList.Replace('{{define "list"}}', '{{define "news-list"}}')
    TRAINING = $sectionList.Replace('{{define "list"}}', '{{define "training-list"}}')
    STANDARDS = $sectionList.Replace('{{define "list"}}', '{{define "standards-list"}}')
    LIST = $list
    ARTICLE = $article
    ABOUT = $aboutBase.TrimEnd() + "`n" + $sectionList.Replace('{{define "list"}}', '{{define "about-list"}}') + "`n"
}

$sqlOut = Join-Path $stage 'db'
$migrate = Get-Content -LiteralPath (Join-Path $repoRoot 'deploy\camie\sql\migrate.sql.in') -Raw -Encoding utf8
$migrate = $migrate.Replace('__RELEASE_VERSION__', $Version)
foreach ($entry in $sources.GetEnumerator()) {
    $migrate = $migrate.Replace("__$($entry.Key)_BASE64__", (To-Base64Utf8 $entry.Value))
}
if ($migrate -match '__[A-Z_]+__') { throw 'Unresolved placeholder remains in generated migrate.sql' }
Write-Utf8NoBom (Join-Path $sqlOut 'migrate.sql') $migrate

$rollback = Get-Content -LiteralPath (Join-Path $repoRoot 'deploy\camie\sql\rollback.sql.in') -Raw -Encoding utf8
$rollback = $rollback.Replace('__RELEASE_VERSION__', $Version)
Write-Utf8NoBom (Join-Path $sqlOut 'rollback.sql') $rollback
Copy-Item -LiteralPath (Join-Path $repoRoot 'deploy\camie\sql\preflight.sql') -Destination (Join-Path $sqlOut 'preflight.sql') -Force
Remove-Item -LiteralPath (Join-Path $stage 'ops\sql') -Recurse -Force

$metadata = [ordered]@{
    version = $Version
    built_at_utc = [DateTime]::UtcNow.ToString('o')
    target = 'linux/amd64'
    portal_static_commit = $portalCommit
    dia_platform_commit = $diaCommit
    git_tag = $releaseTag
    portal_static_go = $staticGoVersion
    dia_platform_go = $diaGoVersion
    node = $nodeVersion
    npm = $npmVersion
    database_generate_gate = -not $SkipDatabaseGenerate
    member_status_risk_accepted = $true
}
Write-Utf8NoBom (Join-Path $stage 'VERSION') ($Version + "`n")
Write-Utf8NoBom (Join-Path $stage 'MANIFEST.json') (($metadata | ConvertTo-Json -Depth 3) + "`n")

$forbiddenNames = Get-ChildItem -LiteralPath $stage -Recurse -Force | Where-Object {
    $_.Name -match '(?i)(db-password|jwt-secret|static-token|test-member-passwords|\.exe$|\.pid$|\.log$)' -or
    ($_.Name -match '^\.env$')
}
if ($forbiddenNames) { throw "Forbidden files entered release package:`n$($forbiddenNames.FullName -join "`n")" }

$textFiles = Get-ChildItem -LiteralPath $stage -Recurse -File | Where-Object {
    $_.Extension -in @('.yaml', '.yml', '.json', '.md', '.sql', '.sh', '.service', '.conf', '.example', '.tmpl', '.html', '.css', '.js') -or $_.Name -eq 'VERSION'
}
$forbiddenPatterns = @(
    'D:\\WebstormProjects',
    'C:\\Users\\administration',
    '127\.0\.0\.1:1808[0-9]',
    'camie_test_active',
    'camie_test_pending'
)
foreach ($pattern in $forbiddenPatterns) {
    $matches = $textFiles | Select-String -Pattern $pattern -ErrorAction SilentlyContinue
    if ($matches) { throw "Forbidden local/test value matched '$pattern':`n$($matches -join "`n")" }
}

$checksumLines = Get-ChildItem -LiteralPath $stage -Recurse -File |
    Where-Object { $_.Name -ne 'SHA256SUMS' } |
    Sort-Object FullName |
    ForEach-Object {
        $relative = [IO.Path]::GetRelativePath($stage, $_.FullName).Replace('\', '/')
        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash.ToLowerInvariant()
        "$hash  $relative"
    }
Write-Utf8NoBom (Join-Path $stage 'SHA256SUMS') (($checksumLines -join "`n") + "`n")

if (Test-Path -LiteralPath $archive) { Remove-Item -LiteralPath $archive -Force }
if (Test-Path -LiteralPath $archiveHash) { Remove-Item -LiteralPath $archiveHash -Force }
New-Item -ItemType Directory -Path $releaseBase -Force | Out-Null
Invoke-Native tar @('-czf', $archive, '-C', $workRoot, $packageName) $repoRoot
$archiveDigest = (Get-FileHash -Algorithm SHA256 -LiteralPath $archive).Hash.ToLowerInvariant()
Write-Utf8NoBom $archiveHash "$archiveDigest  $([IO.Path]::GetFileName($archive))`n"

Write-Host "CAMIE release package created:" -ForegroundColor Green
Write-Host "  $archive"
Write-Host "  $archiveHash"
Write-Host "  staging: $stage"

$env:GOTOOLCHAIN = $oldGoToolchain
