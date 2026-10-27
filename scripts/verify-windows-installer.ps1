#requires -Version 5.1
<#
Exercises the real per-user installer in an isolated application directory.
Temporarily uses product HKCU registration and shortcuts; refuses to run if any
existing installation, registration, or product shortcut could be affected.
Run the longer browser option through jobgate on the shared workstation.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$SetupFile,
    [Parameter(Mandatory=$true)][string]$PortableDirectory,
    [Parameter(Mandatory=$true)][string]$PreviousSetupFile,
    [switch]$RunBrowserChecks
)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$repoRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$setup=[IO.Path]::GetFullPath($SetupFile)
$previous=[IO.Path]::GetFullPath($PreviousSetupFile)
$portable=[IO.Path]::GetFullPath($PortableDirectory)
foreach($file in @($setup,$previous,(Join-Path $portable 'build-info.json'))) {
    if(-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing test input: $file" }
}
$build=Get-Content -LiteralPath (Join-Path $portable 'build-info.json') -Raw | ConvertFrom-Json
$node=$(if($RunBrowserChecks){(Get-Command node -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source}else{$null})
$programs=[Environment]::GetFolderPath('Programs')
$desktop=[Environment]::GetFolderPath('DesktopDirectory')
$startLink=Join-Path $programs 'Scene Palette.lnk'
$desktopLink=Join-Path $desktop 'Scene Palette.lnk'
$appKey='Software\Scene Palette'
$uninstallKey='Software\Microsoft\Windows\CurrentVersion\Uninstall\ScenePalette'
$cu64=[Microsoft.Win32.RegistryKey]::OpenBaseKey([Microsoft.Win32.RegistryHive]::CurrentUser,[Microsoft.Win32.RegistryView]::Registry64)
$cu32=[Microsoft.Win32.RegistryKey]::OpenBaseKey([Microsoft.Win32.RegistryHive]::CurrentUser,[Microsoft.Win32.RegistryView]::Registry32)
foreach($view in @($cu64,$cu32)) {
    foreach($keyName in @($appKey,$uninstallKey)) {
        $existing=$view.OpenSubKey($keyName)
        if($existing){$existing.Dispose();throw 'Existing Scene Palette registration found. Installer tests will not change it.'}
    }
}
foreach($existing in @($startLink,$desktopLink,(Join-Path $env:LOCALAPPDATA 'Programs\Scene Palette'))) {
    if(Test-Path -LiteralPath $existing){throw "Existing user installation/shortcut found; refusing to change it: $existing"}
}
$stamp=[DateTime]::UtcNow.ToString('yyyyMMdd-HHmmssfff')
$output=Join-Path $repoRoot ('bin\checks\installer-lifecycle-'+$stamp)
$installDir=Join-Path $output 'Installed App'
$otherDir=Join-Path $output 'Different Destination'
$report=[ordered]@{status='running';startedAtUtc=[DateTime]::UtcNow.ToString('o');setup=$setup;previousSetup=$previous;portable=$portable;installDirectory=$installDir;checks=@();cleanup='pending'}
New-Item -ItemType Directory -Path $output | Out-Null
$server=$null
$payload=@('scene-palette.exe','index.html','Run Scene Palette.cmd','README.txt','LICENSE','THIRD_PARTY_NOTICES.md','build-info.json','SHA256SUMS.txt')
function FileHash([string]$path){
    $stream=[IO.File]::OpenRead($path);$sha=[Security.Cryptography.SHA256]::Create()
    try{return [BitConverter]::ToString($sha.ComputeHash($stream)).Replace('-','').ToLowerInvariant()}
    finally{$stream.Dispose();$sha.Dispose()}
}
function Check([bool]$condition,[string]$name){if(-not $condition){throw $name}}
function Passed([string]$name){$report.checks+= $name;Write-Output "PASS $name"}
function AppValue([string]$key,[string]$name){
    $entry=$cu64.OpenSubKey($key);if(-not $entry){return $null}
    try{return $entry.GetValue($name)}finally{$entry.Dispose()}
}
function Invoke-OwnedProgram([string]$file,[string]$arguments,[int]$timeout=30000,[switch]$NoRuntime){
    $info=New-Object Diagnostics.ProcessStartInfo
    $info.FileName=$file;$info.Arguments=$arguments;$info.UseShellExecute=$false;$info.CreateNoWindow=$true
    $info.RedirectStandardOutput=$true;$info.RedirectStandardError=$true
    if($NoRuntime){$info.EnvironmentVariables['PATH']=$env:SystemRoot+'\System32;'+$env:SystemRoot}
    $process=New-Object Diagnostics.Process;$process.StartInfo=$info
    try{
        [void]$process.Start();$stdout=$process.StandardOutput.ReadToEndAsync();$stderr=$process.StandardError.ReadToEndAsync()
        if(-not $process.WaitForExit($timeout)){$process.Kill();$process.WaitForExit();throw "Owned process timed out: $file"}
        return @{code=$process.ExitCode;stdout=$stdout.GetAwaiter().GetResult();stderr=$stderr.GetAwaiter().GetResult()}
    }finally{$process.Dispose()}
}
function Invoke-Uninstall([int]$expected=0){
    # Use the exact uninstaller bytes from an external test location, matching
    # NSIS's normal temporary-copy execution while retaining a waitable PID.
    $copy=Join-Path $output 'uninstall-test-runner.exe'
    Copy-Item -LiteralPath (Join-Path $installDir 'Uninstall.exe') -Destination $copy -Force
    $result=Invoke-OwnedProgram $copy ('/S _?='+$installDir)
    Check ($result.code -eq $expected) "Uninstaller exit $($result.code), expected $expected. $($result.stderr)"
}
function Check-Payload {
    foreach($file in $payload){Check ((FileHash (Join-Path $installDir $file)) -eq (FileHash (Join-Path $portable $file))) "Installed payload differs: $file"}
    Check (Test-Path -LiteralPath (Join-Path $installDir 'Uninstall.exe')) 'Installed uninstaller missing.'
    Check ((AppValue $appKey 'InstallLocation') -eq $installDir) 'Wrong application install registration.'
    Check ((AppValue $uninstallKey 'InstallLocation') -eq $installDir) 'Wrong uninstall location.'
    Check ((AppValue $uninstallKey 'DisplayVersion') -eq $build.version) 'Wrong installed version.'
    Check ((AppValue $uninstallKey 'UninstallString') -eq ('"'+$installDir+'\Uninstall.exe"')) 'Uninstall command is not properly quoted.'
    $shell=New-Object -ComObject WScript.Shell
    $link=$shell.CreateShortcut($startLink)
    Check ($link.TargetPath -eq (Join-Path $installDir 'scene-palette.exe')) 'Start Menu shortcut targets wrong app.'
}
function Start-OwnedServer {
    $info=New-Object Diagnostics.ProcessStartInfo
    $info.FileName=Join-Path $installDir 'scene-palette.exe';$info.Arguments='serve -addr 127.0.0.1:0'
    $info.UseShellExecute=$false;$info.CreateNoWindow=$true;$info.RedirectStandardOutput=$true;$info.RedirectStandardError=$true
    $info.EnvironmentVariables['PATH']=$env:SystemRoot+'\System32;'+$env:SystemRoot
    $script:server=New-Object Diagnostics.Process;$script:server.StartInfo=$info;[void]$script:server.Start()
    $line=$script:server.StandardOutput.ReadLineAsync()
    Check ($line.Wait(15000)) 'Installed test server did not become ready.'
    Check ($line.Result -match '^Scene Palette frontend: (http://127\.0\.0\.1:\d+/)$') 'Installed server returned an unexpected local URL.'
    return $Matches[1]
}
function Stop-OwnedServer {
    if($script:server){if(-not $script:server.HasExited){$script:server.Kill();$script:server.WaitForExit()};$script:server.Dispose();$script:server=$null}
}
try{
    $first=Invoke-OwnedProgram $previous ('/S /D='+$installDir)
    Check ($first.code -eq 0) "Previous-version fixture install failed ($($first.code))."
    $oldVersion=AppValue $uninstallKey 'DisplayVersion'
    Check ([version]$oldVersion -lt [version]$build.version) 'Upgrade fixture is not an older application version.'
    $oldBuild=Get-Content -LiteralPath (Join-Path $installDir 'build-info.json') -Raw | ConvertFrom-Json
    Check ($oldBuild.version -eq $oldVersion) 'Previous payload metadata differs from the registered fixture version.'
    Check ((FileHash (Join-Path $installDir 'scene-palette.exe')) -ne (FileHash (Join-Path $portable 'scene-palette.exe'))) 'Upgrade fixture must contain a different executable.'
    Check (-not (Test-Path -LiteralPath $desktopLink)) 'Desktop shortcut should be optional and off by default.'
    Passed "Previous-version fixture installs per user without a desktop shortcut ($oldVersion)."
    $exports=Join-Path $installDir 'exports';New-Item -ItemType Directory -Path $exports | Out-Null
    $saved=Join-Path $exports 'keep-palette.json';[IO.File]::WriteAllText($saved,'{"betaTest":"preserve this exported palette"}')
    $unknown=Join-Path $installDir 'keep-user-file.txt';[IO.File]::WriteAllText($unknown,'This unrelated file must survive upgrade and uninstall.')
    $savedHash=FileHash $saved;$unknownHash=FileHash $unknown
    $url=Start-OwnedServer
    $before=FileHash (Join-Path $installDir 'scene-palette.exe')
    $blocked=Invoke-OwnedProgram $setup '/S'
    Check ($blocked.code -ne 0) 'Running application upgrade was not rejected.'
    Check ((FileHash (Join-Path $installDir 'scene-palette.exe')) -eq $before) 'Rejected upgrade changed the running executable.'
    Check (-not $server.HasExited) 'Installer terminated the running application.'
    Stop-OwnedServer
    Passed 'Upgrade refuses a running application without changing its binary or killing it.'
    $upgrade=Invoke-OwnedProgram $setup '/S'
    Check ($upgrade.code -eq 0) "Upgrade failed ($($upgrade.code))."
    Check-Payload
    Check ((FileHash $saved) -eq $savedHash -and (FileHash $unknown) -eq $unknownHash) 'Upgrade changed user files.'
    Passed 'Previous version upgrades in the registered custom location with exact portable payload and user files preserved.'
    # Fixture: an existing user opted into Desktop before this reinstall.
    $desktopSettings=$cu64.CreateSubKey($appKey)
    try{$desktopSettings.SetValue('DesktopShortcut',1,[Microsoft.Win32.RegistryValueKind]::DWord)}finally{$desktopSettings.Dispose()}
    $reinstall=Invoke-OwnedProgram $setup '/S';Check ($reinstall.code -eq 0) 'Same-version reinstall failed.';Check-Payload
    $shortcutShell=New-Object -ComObject WScript.Shell
    Check (Test-Path -LiteralPath $desktopLink) 'Existing optional desktop preference was not restored.'
    Check ($shortcutShell.CreateShortcut($desktopLink).TargetPath -eq (Join-Path $installDir 'scene-palette.exe')) 'Desktop shortcut targets wrong app.'
    Passed 'Same-version reinstall preserves identity and payload.'
    Passed 'An existing optional desktop preference creates the correct current-user shortcut.'
    $downgrade=Invoke-OwnedProgram $previous '/S';Check ($downgrade.code -ne 0) 'Downgrade was not rejected.';Check-Payload
    $relocate=Invoke-OwnedProgram $setup ('/S /D='+$otherDir);Check ($relocate.code -ne 0) 'Conflicting upgrade destination was not rejected.';Check-Payload
    Check (-not (Test-Path -LiteralPath $otherDir)) 'Rejected relocation created a destination.'
    Passed 'Downgrade and conflicting destination are rejected without changing the installed application.'
    $help=Invoke-OwnedProgram (Join-Path $installDir 'scene-palette.exe') 'help' -NoRuntime
    Check ($help.code -eq 0 -and $help.stdout -match 'forge') 'Installed help requires an external runtime or failed.'
    $cli=Join-Path $output 'installed-cli-palette.json'
    $forgeArguments='forge -color-policy source-hues -out "'+$cli+'" "'+(Join-Path $repoRoot 'testdata\reference-images\pastel-prisms.png')+'"'
    $forged=Invoke-OwnedProgram (Join-Path $installDir 'scene-palette.exe') $forgeArguments -NoRuntime
    Check ($forged.code -eq 0) 'Installed CLI Forge failed without external runtimes.'
    Check (((Get-Content -LiteralPath $cli -Raw | ConvertFrom-Json).params.colorPolicy) -eq 'source-hues') 'Installed CLI ignored colour policy.'
    Passed 'Installed help and CLI Forge work without Go or Node on PATH.'
    $url=Start-OwnedServer
    Invoke-Uninstall 2
    Check-Payload
    Check (-not $server.HasExited) 'Uninstaller terminated the running app.'
    Passed 'Uninstall refuses a running application without changing files or killing it.'
    if($RunBrowserChecks){
        $browserOutput='bin/e2e/installer-'+$stamp
        Push-Location $repoRoot
        try{$browser=Invoke-OwnedProgram $node ('"'+(Join-Path $repoRoot 'scripts\browser-e2e.mjs')+'" --url '+$url+' --html "'+(Join-Path $installDir 'index.html')+'" --output '+$browserOutput) 360000}
        finally{Pop-Location}
        [IO.File]::WriteAllText((Join-Path $output 'browser.log'),$browser.stdout+"`r`n"+$browser.stderr)
        Check ($browser.code -eq 0) 'Installed browser workflows failed; inspect browser.log.'
        $report.browserReport=Join-Path $repoRoot ($browserOutput+'\report.json')
        Passed 'Full served and standalone browser workflows pass against the installed application.'
    }
    Stop-OwnedServer
    Invoke-Uninstall
    foreach($file in ($payload+@('Uninstall.exe'))){Check (-not (Test-Path -LiteralPath (Join-Path $installDir $file))) "Uninstall left installed file: $file"}
    Check ((FileHash $saved) -eq $savedHash -and (FileHash $unknown) -eq $unknownHash) 'Uninstall changed user files.'
    Check (-not (Test-Path -LiteralPath $startLink) -and -not (Test-Path -LiteralPath $desktopLink)) 'Uninstall left product shortcuts.'
    Check ($null -eq (AppValue $appKey 'InstallLocation') -and $null -eq (AppValue $uninstallKey 'DisplayVersion')) 'Uninstall left product registration.'
    Passed 'Uninstall removes owned app files, shortcuts and registration while preserving exports and unknown files.'
    $installDir=Join-Path $output 'Fresh Current App'
    $report.freshInstallDirectory=$installDir
    $fresh=Invoke-OwnedProgram $setup ('/S /D='+$installDir)
    Check ($fresh.code -eq 0) 'Fresh current-version installation failed.';Check-Payload
    Check (-not (Test-Path -LiteralPath $desktopLink)) 'Fresh current install must leave desktop shortcut optional.'
    Passed 'Current version installs fresh with exact payload and default-off desktop shortcut.'
    Invoke-Uninstall
    Check (-not (Test-Path -LiteralPath $installDir)) 'Clean uninstall did not remove the empty installation directory.'
    Check (-not (Test-Path -LiteralPath $startLink) -and -not (Test-Path -LiteralPath $desktopLink)) 'Fresh uninstall left product shortcuts.'
    Check ($null -eq (AppValue $appKey 'InstallLocation') -and $null -eq (AppValue $uninstallKey 'DisplayVersion')) 'Fresh uninstall left registration.'
    Passed 'A fresh current installation uninstalls without leaving app files, shortcuts or registration.'
    $report.cleanup='passed';$report.status='passed'
}catch{$report.status='failed';$report.failure=$_.ToString();Write-Error $_ -ErrorAction Continue}
finally{
    Stop-OwnedServer
    if((AppValue $appKey 'InstallLocation') -eq $installDir){
        try{Invoke-Uninstall;$report.cleanup='completed-after-failure'}catch{$report.cleanup='failed: '+$_.ToString()}
    }
    $report.finishedAtUtc=[DateTime]::UtcNow.ToString('o')
    $cu64.Dispose();$cu32.Dispose()
    [IO.File]::WriteAllText((Join-Path $output 'report.json'),($report | ConvertTo-Json -Depth 6))
    Write-Output ('Installer lifecycle '+$report.status+': '+(Join-Path $output 'report.json'))
}
if($report.status -ne 'passed'){exit 1}
