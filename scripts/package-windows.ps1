#requires -Version 5.1
<#
.SYNOPSIS
Build a portable, unsigned Windows release without changing the source HTML.
.DESCRIPTION
Run from any directory. Requires Go and Git; no installer or runtime is needed
by the recipient. Each invocation creates a fresh folder and ZIP under bin/releases.
Use -RequireClean for a review package. -ReleaseTag additionally requires the local
v<application-version> tag at HEAD and an explicit -ExpectedGoVersion (goX.Y.Z).
Clean/tag builds ignore user Go configuration, workspaces, and injected build flags,
and use the selected local toolchain without automatic switching. Development builds
retain those settings. The script never creates tags or publishes anything.
#>
[CmdletBinding()]
param(
    [switch]$RequireClean,
    [string]$ReleaseTag,
    [string]$ExpectedGoVersion
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$releaseRoot = [IO.Path]::GetFullPath((Join-Path $repoRoot 'bin\releases'))
$repoPrefix = $repoRoot.TrimEnd('\') + '\'
if (-not $releaseRoot.StartsWith($repoPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Release output must remain inside the repository.'
}
# Do not follow an existing bin/releases junction into another project.
foreach ($directory in @((Join-Path $repoRoot 'bin'), $releaseRoot)) {
    if ((Test-Path -LiteralPath $directory) -and
        ((Get-Item -LiteralPath $directory).Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw "Release output cannot use a junction or symbolic link: $directory"
    }
}

$go = (Get-Command go -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
$git = (Get-Command git -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source

function Invoke-Git {
    param([string[]]$Arguments)
    $result = & $git -C $repoRoot @Arguments
    if ($LASTEXITCODE -ne 0) { throw "git $($Arguments -join ' ') failed ($LASTEXITCODE)." }
    return $result
}

function Get-FileSha256 {
    param([string]$Path)
    $stream = [IO.File]::OpenRead($Path)
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return [BitConverter]::ToString($sha.ComputeHash($stream)).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose(); $stream.Dispose() }
}

function Get-SourceSnapshot {
    # Include uncommitted/untracked source, but never ignored release artifacts.
    $paths = @(Invoke-Git @('ls-files', '-z', '--cached', '--others', '--exclude-standard')) -join "`n"
    $files = foreach ($relative in ($paths.Split([char]0) | Where-Object { $_ } | Sort-Object -Unique)) {
        $path = Join-Path $repoRoot $relative
        if (Test-Path -LiteralPath $path -PathType Leaf) {
            [ordered]@{ path = $relative.Replace('\', '/'); sha256 = Get-FileSha256 $path }
        }
    }
    $canonical = ($files | ForEach-Object { $_.sha256 + '  ' + $_.path }) -join "`n"
    $sha = [Security.Cryptography.SHA256]::Create()
    try { $digest = [BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($canonical))).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
    return @{ sha256 = $digest; files = @($files) }
}

$oldEnvironment = @{}
foreach ($name in @('GOOS', 'GOARCH', 'CGO_ENABLED', 'GOENV', 'GOFLAGS', 'GOWORK',
    'GOTOOLCHAIN', 'GOROOT', 'GO111MODULE', 'GOEXPERIMENT', 'GOAMD64', 'GOFIPS140', 'GODEBUG')) {
    $oldEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
Push-Location $repoRoot
try {
    $commit = (Invoke-Git @('rev-parse', 'HEAD')).Trim()
    $dirty = [bool](Invoke-Git @('status', '--porcelain', '--untracked-files=normal'))
    $cleanRequired = $RequireClean -or -not [string]::IsNullOrWhiteSpace($ReleaseTag)
    if ($cleanRequired -and $dirty) { throw 'This build requires a clean source tree. Commit or set aside source changes first.' }
    if ($ReleaseTag -and $ReleaseTag -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+$') {
        throw 'ReleaseTag must be the local application version tag (vX.Y.Z).'
    }
    if ($ExpectedGoVersion -and $ExpectedGoVersion -notmatch '^go[0-9]+\.[0-9]+\.[0-9]+$') {
        throw 'ExpectedGoVersion must name an exact patched toolchain, for example go1.27.3.'
    }
    if ($ReleaseTag -and -not $ExpectedGoVersion) {
        throw 'Tagged release builds require -ExpectedGoVersion after checking the supported Go patch release.'
    }
    if ($cleanRequired) {
        # A clean Git tree does not constrain GOFLAGS overlays, alternate modfiles,
        # parent workspaces, or settings persisted by 'go env -w'. Ignore all of
        # those before invoking Go, including version/toolchain discovery.
        $env:GOENV = 'off'
        $env:GOWORK = 'off'
        $env:GOTOOLCHAIN = 'local'
        $env:GO111MODULE = 'on'
        $env:GOAMD64 = 'v1'
        $env:GOFIPS140 = 'off'
        foreach ($name in @('GOFLAGS', 'GOROOT', 'GOEXPERIMENT', 'GODEBUG')) {
            [Environment]::SetEnvironmentVariable($name, $null, 'Process')
        }
    }
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'
    $goVersion = (& $go version) -join ' '
    if ($LASTEXITCODE -ne 0) { throw 'go version failed.' }
    $goToolchain = ((& $go env GOVERSION) -join '').Trim()
    if ($LASTEXITCODE -ne 0) { throw 'go env GOVERSION failed.' }
    if ($ExpectedGoVersion -and $goToolchain -ne $ExpectedGoVersion) {
        throw "Expected $ExpectedGoVersion, but the selected Go toolchain is $goToolchain."
    }
    $environmentJSON = (& $go env -json GOENV GOFLAGS GOWORK GOTOOLCHAIN GO111MODULE GOEXPERIMENT GOAMD64 GOFIPS140 GOOS GOARCH CGO_ENABLED) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw 'Could not read the effective Go build environment.' }
    $goEnvironment = $environmentJSON | ConvertFrom-Json
    if (-not $cleanRequired) {
        # Developer flags/configuration can contain paths or secret-bearing linker
        # values. Record which overrides are active, never their arbitrary contents.
        $goEnvironment = [ordered]@{
            GOOS = 'windows'; GOARCH = 'amd64'; CGO_ENABLED = '0'
            GOENV = $(if ($goEnvironment.GOENV) { 'enabled (path omitted)' } else { 'off' })
            GOFLAGS = $(if ($goEnvironment.GOFLAGS) { 'configured (values omitted)' } else { '' })
            GOWORK = $(if ($goEnvironment.GOWORK -in @('', 'off')) { $goEnvironment.GOWORK } else { 'enabled (path omitted)' })
            GOTOOLCHAIN = $(if ($goEnvironment.GOTOOLCHAIN -in @('local', 'auto')) { $goEnvironment.GOTOOLCHAIN } else { 'configured (value omitted)' })
            GO111MODULE = $(if ($goEnvironment.GO111MODULE -in @('', 'on', 'off', 'auto')) { $goEnvironment.GO111MODULE } else { 'configured (value omitted)' })
            GOEXPERIMENT = $(if ($goEnvironment.GOEXPERIMENT) { 'configured (values omitted)' } else { '' })
            GOAMD64 = $(if ($goEnvironment.GOAMD64 -match '^v[1-4]$') { $goEnvironment.GOAMD64 } else { 'configured (value omitted)' })
            GOFIPS140 = $(if ($goEnvironment.GOFIPS140 -in @('', 'off')) { $goEnvironment.GOFIPS140 } else { 'configured (value omitted)' })
        }
    }
    $buildFlags = @('-trimpath', '-buildvcs=false', '-ldflags=-s -w')
    if ($cleanRequired) {
        # A default.pgo outside the tracked source must not silently affect a release.
        # Read-only module resolution also prevents dependency metadata changes.
        $buildFlags += @('-pgo=off', '-mod=readonly')
    }
    $source = Get-SourceSnapshot
    $contract = Get-Content -LiteralPath (Join-Path $repoRoot 'internal\palette\contract.go') -Raw
    $versionMatch = [regex]::Match($contract, 'ToolVersion\s*=\s*"([0-9]+\.[0-9]+\.[0-9]+)"')
    if (-not $versionMatch.Success) { throw 'Could not read ToolVersion from internal/palette/contract.go.' }
    $version = $versionMatch.Groups[1].Value
    if ($ReleaseTag) {
        if ($ReleaseTag -ne ('v' + $version)) { throw "ReleaseTag must match application version v$version." }
        $tagCommit = (Invoke-Git @('rev-parse', '--verify', ('refs/tags/' + $ReleaseTag + '^{commit}'))).Trim()
        if ($tagCommit -ne $commit) { throw "ReleaseTag $ReleaseTag does not point to HEAD." }
    }
    $extractionMatch = [regex]::Match($contract, 'ExtractionVersion\s*=\s*([0-9]+)')
    if (-not $extractionMatch.Success) { throw 'Could not read ExtractionVersion from internal/palette/contract.go.' }
    $extractionVersion = [int]$extractionMatch.Groups[1].Value
    $builtAt = [DateTime]::UtcNow
    $name = 'scene-palette-{0}-windows-amd64-{1}-{2}' -f $version, $builtAt.ToString('yyyyMMdd-HHmmssfff'), $commit.Substring(0, 7)
    if ($dirty) { $name += '-dirty' }
    $packageDir = Join-Path $releaseRoot $name
    $zipPath = $packageDir + '.zip'
    if ((Test-Path -LiteralPath $packageDir) -or (Test-Path -LiteralPath $zipPath)) {
        throw "Release already exists; rerun to create a new build: $packageDir"
    }
    New-Item -ItemType Directory -Path $packageDir | Out-Null

    $buildArgs = @('build') + $buildFlags + @('-o', (Join-Path $packageDir 'scene-palette.exe'), '.')
    & $go @buildArgs
    if ($LASTEXITCODE -ne 0) { throw "go build failed ($LASTEXITCODE). Partial output is retained at $packageDir" }

    $binaryBuildInfo = @(& $go version -m (Join-Path $packageDir 'scene-palette.exe'))
    if ($LASTEXITCODE -ne 0 -or $binaryBuildInfo.Count -eq 0) { throw 'Could not read the executable Go build information.' }
    $binaryVersionMatch = [regex]::Match($binaryBuildInfo[0], ':\s+(go\S+)\s*$')
    if (-not $binaryVersionMatch.Success -or $binaryVersionMatch.Groups[1].Value -ne $goToolchain) {
        throw "The executable Go version does not match the recorded toolchain $goToolchain."
    }

    Copy-Item -LiteralPath (Join-Path $repoRoot 'index.html') -Destination $packageDir
    Copy-Item -LiteralPath (Join-Path $repoRoot 'Run Scene Palette.cmd') -Destination $packageDir
    Copy-Item -LiteralPath (Join-Path $repoRoot 'LICENSE') -Destination $packageDir
    Copy-Item -LiteralPath (Join-Path $repoRoot 'THIRD_PARTY_NOTICES.md') -Destination $packageDir
    $afterBuild = Get-SourceSnapshot
    if ($afterBuild.sha256 -ne $source.sha256 -or (Invoke-Git @('rev-parse', 'HEAD')).Trim() -ne $commit) {
        throw "Source changed during packaging. Rerun after edits settle. Partial output is retained at $packageDir"
    }
    $utf8 = New-Object Text.UTF8Encoding($false)
    $readme = @"
Scene Palette $version - Windows x64 portable edition

Extract the ZIP, then double-click scene-palette.exe or Run Scene Palette.cmd.
Your default browser opens the local app. Keep its console window open while
using it; close the console or press Ctrl+C to stop the local server.
No Go installation, installer, account, or network connection is required.
The server listens on 127.0.0.1:8765, or another local port if that port is busy.

For the zero-install browser version, open index.html directly. Keep using the
same browser/profile to retain its local shelf. The standalone HTML contains
the original JavaScript engine; the executable also provides the Go engine.

Advanced: scene-palette.exe serve starts without opening a browser.
Use scene-palette.exe help for local command-line tools.

This is an unsigned portable build. Windows may show an unrecognized-app warning.
Obtain the package from the project maintainer and verify the accompanying checksum.
build-info.json records the source revision, working-tree state, and hashes.
SHA256SUMS.txt contains SHA-256 hashes of the files in this package.
LICENSE and THIRD_PARTY_NOTICES.md contain the applicable license notices.
"@
    [IO.File]::WriteAllText((Join-Path $packageDir 'README.txt'), $readme + "`r`n", $utf8)
    $metadata = [ordered]@{
        name = 'Scene Palette'
        version = $version
        extractionVersion = $extractionVersion
        builtAtUtc = $builtAt.ToString('o')
        platform = 'windows/amd64'
        goVersion = $goVersion
        goToolchain = $goToolchain
        executableGoVersion = $binaryVersionMatch.Groups[1].Value
        cleanSourceRequired = [bool]$cleanRequired
        goEnvironmentMode = $(if ($cleanRequired) { 'isolated-release' } else { 'inherited-development' })
        goEnvironment = $goEnvironment
        releaseTag = $(if ($ReleaseTag) { $ReleaseTag } else { $null })
        cgoEnabled = $false
        signed = $false
        sourceCommit = $commit
        sourceDirty = $dirty
        sourceTreeSha256 = $source.sha256
        buildFlags = $buildFlags
        sourceFiles = $source.files
    }
    [IO.File]::WriteAllText((Join-Path $packageDir 'build-info.json'), ($metadata | ConvertTo-Json -Depth 6) + "`r`n", $utf8)
    $checksums = Get-ChildItem -LiteralPath $packageDir -File | Sort-Object Name | ForEach-Object {
        (Get-FileSha256 $_.FullName) + '  ' + $_.Name
    }
    [IO.File]::WriteAllText((Join-Path $packageDir 'SHA256SUMS.txt'), ($checksums -join "`r`n") + "`r`n", $utf8)

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [IO.Compression.ZipFile]::CreateFromDirectory($packageDir, $zipPath, [IO.Compression.CompressionLevel]::Optimal, $true)
    $zipHash = Get-FileSha256 $zipPath
    [IO.File]::WriteAllText(($zipPath + '.sha256'), $zipHash + '  ' + [IO.Path]::GetFileName($zipPath) + "`r`n", $utf8)
    Write-Output "Portable folder: $packageDir"
    Write-Output "ZIP: $zipPath"
    Write-Output "ZIP SHA256: $zipHash"
}
finally {
    foreach ($name in $oldEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($name, $oldEnvironment[$name], 'Process')
    }
    Pop-Location
}
