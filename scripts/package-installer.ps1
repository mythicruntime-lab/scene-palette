#requires -Version 5.1
<#
.SYNOPSIS
Package an existing verified clean Windows x64 portable build as an unsigned per-user installer.
.DESCRIPTION
Never recompiles Scene Palette. Requires Git, Go (to inspect embedded build info),
and a local NSIS 3 compiler. The portable build must come from this exact clean
source revision and an explicitly selected patched Go toolchain. Build it first
with scripts/package-windows.ps1 -RequireClean -ExpectedGoVersion goX.Y.Z.
Outputs are retained under ignored bin/releases. No tags or publishing are performed.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$PortableDirectory,
    [Parameter(Mandatory = $true)][string]$ExpectedGoVersion,
    [string]$NSISCompiler = 'makensis.exe'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$releaseRoot = Join-Path $repoRoot 'bin\releases'
$portableRoot = [IO.Path]::GetFullPath($PortableDirectory)
$installerScript = Join-Path $PSScriptRoot 'windows-installer.nsi'
$payloadNames = @('scene-palette.exe', 'index.html', 'Run Scene Palette.cmd', 'README.txt',
    'LICENSE', 'THIRD_PARTY_NOTICES.md', 'build-info.json', 'SHA256SUMS.txt')
$hashedNames = @($payloadNames | Where-Object { $_ -ne 'SHA256SUMS.txt' })

function Get-FileSha256 {
    param([string]$Path)
    $stream = [IO.File]::OpenRead($Path)
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return [BitConverter]::ToString($sha.ComputeHash($stream)).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose(); $stream.Dispose() }
}

function Assert-NoReparsePoint {
    param([string]$Path)
    $candidate = [IO.Path]::GetFullPath($Path)
    while ($candidate) {
        if ((Test-Path -LiteralPath $candidate) -and
            ((Get-Item -LiteralPath $candidate).Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            throw "Packaging paths cannot contain junctions or symbolic links: $candidate"
        }
        $candidate = [IO.Path]::GetDirectoryName($candidate)
    }
}

$git = (Get-Command git -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
$go = (Get-Command go -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
$compiler = (Get-Command $NSISCompiler -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
# Background Windows PowerShell sessions may not auto-load this built-in module.
# Import the module shipped with the selected PowerShell, never a user profile.
Import-Module -Name (Join-Path $PSHOME 'Modules\Microsoft.PowerShell.Security\Microsoft.PowerShell.Security.psd1') -ErrorAction Stop
$signatureCommand = Get-Command 'Microsoft.PowerShell.Security\Get-AuthenticodeSignature' -CommandType Cmdlet -ErrorAction Stop
function Invoke-Git {
    param([string[]]$Arguments)
    $result = & $git -C $repoRoot @Arguments
    if ($LASTEXITCODE -ne 0) { throw "git $($Arguments -join ' ') failed ($LASTEXITCODE)." }
    return $result
}

if ($ExpectedGoVersion -notmatch '^go[0-9]+\.[0-9]+\.[0-9]+$') {
    throw 'ExpectedGoVersion must identify an exact patched toolchain (goX.Y.Z).'
}
Assert-NoReparsePoint $releaseRoot
Assert-NoReparsePoint $portableRoot
if (-not (Test-Path -LiteralPath $portableRoot -PathType Container)) { throw 'PortableDirectory must be an existing portable folder.' }
$entries = @(Get-ChildItem -LiteralPath $portableRoot -Force)
if ($entries.Count -ne $payloadNames.Count) { throw 'Portable folder must contain exactly the eight allowlisted payload files.' }
foreach ($entry in $entries) {
    if ($entry.PSIsContainer -or $entry.Name -cnotin $payloadNames -or
        ($entry.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw "Unexpected portable payload entry: $($entry.Name)"
    }
}

$checksums = @{}
foreach ($line in [IO.File]::ReadAllLines((Join-Path $portableRoot 'SHA256SUMS.txt'))) {
    $match = [regex]::Match($line, '^([a-f0-9]{64})  (.+)$')
    if (-not $match.Success) { throw 'Malformed SHA256SUMS.txt line.' }
    $name = $match.Groups[2].Value
    if ($name -cnotin $hashedNames -or $checksums.ContainsKey($name)) { throw "Unexpected or duplicate checksum entry: $name" }
    $checksums[$name] = $match.Groups[1].Value
}
if ($checksums.Count -ne $hashedNames.Count) { throw 'SHA256SUMS.txt does not cover every payload file.' }
foreach ($name in $hashedNames) {
    if ((Get-FileSha256 (Join-Path $portableRoot $name)) -ne $checksums[$name]) { throw "Portable checksum mismatch: $name" }
}
$payloadHashes = [ordered]@{}
foreach ($name in $payloadNames) { $payloadHashes[$name] = Get-FileSha256 (Join-Path $portableRoot $name) }

$build = Get-Content -LiteralPath (Join-Path $portableRoot 'build-info.json') -Raw | ConvertFrom-Json
$commit = (Invoke-Git @('rev-parse', 'HEAD')).Trim()
if (Invoke-Git @('status', '--porcelain', '--untracked-files=normal')) { throw 'Installer packaging requires a clean source tree.' }
if ($build.name -cne 'Scene Palette' -or $build.platform -cne 'windows/amd64' -or
    $build.sourceCommit -cne $commit -or $build.sourceDirty -cne $false -or
    $build.cleanSourceRequired -cne $true -or $build.signed -cne $false -or $build.cgoEnabled -cne $false -or
    $build.goEnvironmentMode -cne 'isolated-release') {
    throw 'Portable build must be unsigned, Windows x64, and built in isolated-release mode from this exact clean source commit.'
}
if ($build.goToolchain -cne $ExpectedGoVersion -or $build.executableGoVersion -cne $ExpectedGoVersion -or
    $build.goVersion -cnotmatch ('^go version ' + [regex]::Escape($ExpectedGoVersion) + ' ')) {
    throw 'Portable provenance does not match ExpectedGoVersion.'
}
# Go reports GOENV as an empty string when configuration-file loading is off.
$requiredEnvironment = [ordered]@{ GOENV = ''; GOFLAGS = ''; GOWORK = 'off'; GOTOOLCHAIN = 'local';
    GO111MODULE = 'on'; GOEXPERIMENT = ''; GOAMD64 = 'v1'; GOFIPS140 = 'off'; GOOS = 'windows'; GOARCH = 'amd64'; CGO_ENABLED = '0' }
foreach ($name in $requiredEnvironment.Keys) {
    if ($build.goEnvironment.$name -cne $requiredEnvironment[$name]) { throw "Unexpected portable build environment: $name" }
}
$requiredFlags = @('-trimpath', '-buildvcs=false', '-ldflags=-s -w', '-pgo=off', '-mod=readonly')
if (@($build.buildFlags).Count -ne $requiredFlags.Count -or
    (@($build.buildFlags) -join "`n") -cne ($requiredFlags -join "`n")) { throw 'Portable build flags do not match the clean release build policy.' }

# Verify the complete source snapshot, not just the claimed commit or app version.
$paths = @(Invoke-Git @('ls-files', '-z', '--cached', '--others', '--exclude-standard')) -join "`n"
$sourceFiles = @($paths.Split([char]0) | Where-Object { $_ } | Sort-Object -Unique | ForEach-Object {
    [ordered]@{ path = $_.Replace('\', '/'); sha256 = Get-FileSha256 (Join-Path $repoRoot $_) }
})
$canonical = ($sourceFiles | ForEach-Object { $_.sha256 + '  ' + $_.path }) -join "`n"
$sha = [Security.Cryptography.SHA256]::Create()
try { $sourceHash = [BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($canonical))).Replace('-', '').ToLowerInvariant() }
finally { $sha.Dispose() }
if ($build.sourceTreeSha256 -cne $sourceHash -or @($build.sourceFiles).Count -ne $sourceFiles.Count) { throw 'Portable source snapshot does not match this checkout.' }
for ($index = 0; $index -lt $sourceFiles.Count; $index++) {
    if ($build.sourceFiles[$index].path -cne $sourceFiles[$index].path -or
        $build.sourceFiles[$index].sha256 -cne $sourceFiles[$index].sha256) { throw 'Portable source file provenance does not match this checkout.' }
}
$contract = Get-Content -LiteralPath (Join-Path $repoRoot 'internal\palette\contract.go') -Raw
$versionMatch = [regex]::Match($contract, 'ToolVersion\s*=\s*"([0-9]+\.[0-9]+\.[0-9]+)"')
$extractionMatch = [regex]::Match($contract, 'ExtractionVersion\s*=\s*([0-9]+)')
if (-not $versionMatch.Success -or $build.version -cne $versionMatch.Groups[1].Value -or
    -not $extractionMatch.Success -or $build.extractionVersion -ne [int]$extractionMatch.Groups[1].Value) {
    throw 'Portable application/extraction versions do not match the source contract.'
}
$version = $versionMatch.Groups[1].Value
if (@($version.Split('.') | Where-Object { [int]$_ -gt 65535 }).Count) { throw 'Application version is outside Windows version resource limits.' }
if ($build.releaseTag) {
    if ($build.releaseTag -cne ('v' + $version) -or
        (Invoke-Git @('rev-parse', '--verify', ('refs/tags/' + $build.releaseTag + '^{commit}'))).Trim() -cne $commit) {
        throw 'Portable release tag does not match this source commit.'
    }
}
$goInspectionEnvironment = @{}
foreach ($name in @('GOENV', 'GOTOOLCHAIN', 'GOWORK', 'GOFLAGS')) {
    $goInspectionEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
try {
    $env:GOENV = 'off'
    $env:GOTOOLCHAIN = 'local'
    $env:GOWORK = 'off'
    [Environment]::SetEnvironmentVariable('GOFLAGS', $null, 'Process')
    $binaryInfo = @(& $go version -m (Join-Path $portableRoot 'scene-palette.exe'))
    $goInspectionExit = $LASTEXITCODE
}
finally {
    foreach ($name in $goInspectionEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($name, $goInspectionEnvironment[$name], 'Process')
    }
}
if ($goInspectionExit -ne 0 -or $binaryInfo.Count -eq 0 -or
    $binaryInfo[0] -cnotmatch (':\s+' + [regex]::Escape($ExpectedGoVersion) + '\s*$')) { throw 'Executable embedded Go version does not match portable provenance.' }
foreach ($setting in @('GOOS=windows', 'GOARCH=amd64', 'CGO_ENABLED=0')) {
    if (-not @($binaryInfo | Where-Object { $_ -match ('\sbuild\s+' + [regex]::Escape($setting) + '$') }).Count) {
        throw "Executable build information is missing $setting."
    }
}
$compilerVersion = ((& $compiler /VERSION) -join '').Trim()
if ($LASTEXITCODE -ne 0 -or $compilerVersion -notmatch '^v3\.[0-9]+(?:\.[0-9]+)?$') { throw 'A release NSIS 3 compiler is required.' }

$builtAt = [DateTime]::UtcNow
$name = 'scene-palette-{0}-windows-amd64-installer-{1}-{2}' -f $version, $builtAt.ToString('yyyyMMdd-HHmmssfff'), $commit.Substring(0, 7)
$outputDirectory = Join-Path $releaseRoot $name
if (Test-Path -LiteralPath $outputDirectory) { throw 'Installer output already exists; rerun to obtain a fresh output path.' }
New-Item -ItemType Directory -Path $outputDirectory | Out-Null
$outputFile = Join-Path $outputDirectory ('scene-palette-' + $version + '-windows-amd64-setup.exe')
$sizeKB = [int][Math]::Ceiling((($entries | Measure-Object Length -Sum).Sum + 128KB) / 1KB)
& $compiler /NOCONFIG /NOCD /V2 /WX "/DPORTABLE_DIR=$portableRoot" "/DOUTPUT_FILE=$outputFile" "/DAPP_VERSION=$version" "/DAPP_VERSION_QUAD=$version.0" "/DSOURCE_COMMIT=$commit" "/DPORTABLE_SIZE_KB=$sizeKB" $installerScript
if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $outputFile -PathType Leaf)) { throw "NSIS compilation failed. Partial output is retained at $outputDirectory" }
# Fail closed if inputs changed while NSIS read them.
foreach ($name in $payloadNames) {
    if ((Get-FileSha256 (Join-Path $portableRoot $name)) -cne $payloadHashes[$name]) { throw "Portable input changed during compilation: $name" }
}
if ((Invoke-Git @('rev-parse', 'HEAD')).Trim() -cne $commit -or
    (Invoke-Git @('status', '--porcelain', '--untracked-files=normal'))) { throw 'Source changed during installer packaging.' }
$signature = & $signatureCommand -LiteralPath $outputFile
if ($signature.Status -ne 'NotSigned') { throw 'Expected an unsigned installer; inspect its unexpected Authenticode status.' }
$outputHash = Get-FileSha256 $outputFile
$metadata = [ordered]@{
    name = 'Scene Palette'; kind = 'windows-per-user-installer'; version = $version
    extractionVersion = $build.extractionVersion; platform = 'windows/amd64'; builtAtUtc = $builtAt.ToString('o')
    signed = $false; sourceCommit = $commit; sourceDirty = $false; sourceTreeSha256 = $sourceHash
    goToolchain = $ExpectedGoVersion; applicationRecompiled = $false; nsisVersion = $compilerVersion
    installScope = 'current-user'; defaultInstallDirectory = '%LOCALAPPDATA%\Programs\Scene Palette'
    applicationRegistryKey = 'HKCU\Software\Scene Palette'
    uninstallRegistryKey = 'HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\ScenePalette (64-bit view)'
    startMenuShortcut = 'current-user Programs\Scene Palette.lnk'; optionalDesktopShortcut = 'current-user Desktop\Scene Palette.lnk'
    nsisCompilerSha256 = Get-FileSha256 $compiler; installerScriptSha256 = Get-FileSha256 $installerScript
    portableFiles = $payloadHashes; installerFile = [IO.Path]::GetFileName($outputFile); installerSha256 = $outputHash
}
$utf8 = New-Object Text.UTF8Encoding($false)
$metadataFile = Join-Path $outputDirectory 'installer-build-info.json'
[IO.File]::WriteAllText($metadataFile, ($metadata | ConvertTo-Json -Depth 6) + "`r`n", $utf8)
[IO.File]::WriteAllText((Join-Path $outputDirectory 'SHA256SUMS.txt'),
    $outputHash + '  ' + [IO.Path]::GetFileName($outputFile) + "`r`n" + (Get-FileSha256 $metadataFile) + "  installer-build-info.json`r`n", $utf8)
Write-Output "Installer: $outputFile"
Write-Output "Installer SHA256: $outputHash"
Write-Output "Installer provenance: $metadataFile"
