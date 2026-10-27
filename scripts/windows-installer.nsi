; -*- coding: utf-8 -*-
; Compile only through package-installer.ps1, which verifies this exact payload.
Unicode true
RequestExecutionLevel user
ManifestSupportedOS Win10
SetCompressor /SOLID lzma
AllowSkipFiles off

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"
!include "Sections.nsh"
!include "WordFunc.nsh"
!include "WinVer.nsh"
!include "x64.nsh"

!ifndef PORTABLE_DIR
  !error "Use scripts/package-installer.ps1 to validate the portable payload first."
!endif
!ifndef OUTPUT_FILE
  !error "OUTPUT_FILE is required."
!endif
!ifndef APP_VERSION
  !error "APP_VERSION is required."
!endif
!ifndef APP_VERSION_QUAD
  !error "APP_VERSION_QUAD is required."
!endif
!ifndef SOURCE_COMMIT
  !error "SOURCE_COMMIT is required."
!endif
!ifndef PORTABLE_SIZE_KB
  !error "PORTABLE_SIZE_KB is required."
!endif

!define APP_KEY "Software\Scene Palette"
!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\ScenePalette"

Name "Scene Palette ${APP_VERSION}"
OutFile "${OUTPUT_FILE}"
InstallDir "$LOCALAPPDATA\Programs\Scene Palette"
BrandingText "Scene Palette - unsigned build"
ShowInstDetails show
ShowUninstDetails show
VIProductVersion "${APP_VERSION_QUAD}"
VIAddVersionKey /LANG=1033 "ProductName" "Scene Palette"
VIAddVersionKey /LANG=1033 "ProductVersion" "${APP_VERSION}"
VIAddVersionKey /LANG=1033 "FileVersion" "${APP_VERSION}"
VIAddVersionKey /LANG=1033 "FileDescription" "Scene Palette per-user installer (unsigned)"
VIAddVersionKey /LANG=1033 "LegalCopyright" "Copyright (c) 2026 Scene Palette contributors"
VIAddVersionKey /LANG=1033 "Comments" "Windows x64; source ${SOURCE_COMMIT}; unsigned"

!define MUI_ABORTWARNING
!define MUI_WELCOMEPAGE_TEXT "Install Scene Palette for your Windows account.$\r$\n$\r$\nThe app opens your default browser and runs a local server in a console window. Close that console to stop it.$\r$\n$\r$\nThis installer and the application are unsigned."
!define MUI_FINISHPAGE_TEXT "Scene Palette is installed.$\r$\n$\r$\nOpen Scene Palette from the Start Menu. Keep its console window open while using the app.$\r$\n$\r$\nThe installer does not start the app automatically."
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "${PORTABLE_DIR}\LICENSE"
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Var InstallerMutex
Var InstallPathOverride

; Apply identical safe preflight checks to install and uninstall. Never terminate
; processes or schedule a reboot replacement of the user's running application.
!macro DEFINE_PREFLIGHT PREFIX
Function ${PREFIX}CheckInstallPath
  ; NSIS GetFullPathName requires the target path to exist. The Win32 API also
  ; normalizes a new installation directory before it has been created.
  System::Call 'kernel32::GetFullPathNameW(w "$INSTDIR", i ${NSIS_MAX_STRLEN}, w.r3, p 0) i.r2'
  ${If} $2 == 0
  ${OrIf} $2 >= ${NSIS_MAX_STRLEN}
    MessageBox MB_ICONSTOP "The installation path could not be resolved or is too long." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
  StrCpy $INSTDIR "$3"
  ${GetRoot} "$INSTDIR" $0
  StrCpy $2 "$0\"
  ${If} $INSTDIR == $0
  ${OrIf} $INSTDIR == $2
  ${OrIf} $0 == ""
    MessageBox MB_ICONSTOP "An installation cannot use a drive or share root." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
  ; Check every existing ancestor, not just the leaf, before writing/deleting.
  StrCpy $1 "$INSTDIR"
  path_loop:
    System::Call 'kernel32::GetFileAttributesW(w r1) i.r2'
    ${If} $2 != -1
      IntOp $2 $2 & 0x400
      ${If} $2 != 0
        MessageBox MB_ICONSTOP "The installation path cannot contain a junction or symbolic link: $1" /SD IDOK
        SetErrorLevel 2
        Abort
      ${EndIf}
    ${EndIf}
    ${If} $1 == $0
      Return
    ${EndIf}
    ${GetParent} "$1" $1
    ${If} $1 == ""
      SetErrorLevel 2
      Abort
    ${EndIf}
    Goto path_loop
FunctionEnd

Function ${PREFIX}CheckAppClosed
  retry_check:
    IfFileExists "$INSTDIR\scene-palette.exe" 0 done
    ; A mapped/running executable cannot be opened for write/delete exclusively.
    System::Call 'kernel32::CreateFileW(w "$INSTDIR\scene-palette.exe", i 0x40010000, i 0, p 0, i 3, i 0x80, p 0) p.r0'
    ${If} $0 == -1
      IfSilent failed
      MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "Close Scene Palette's console window before continuing. The executable may also be locked by another program. No process will be stopped by this installer." IDRETRY retry_check
      Goto failed
    ${EndIf}
    System::Call 'kernel32::CloseHandle(p r0)'
  done:
    Return
  failed:
    SetErrorLevel 2
    Abort
FunctionEnd

Function ${PREFIX}AcquireInstallerMutex
  System::Call 'kernel32::CreateMutexW(p 0, i 0, w "Local\ScenePalette-Installer") p.r0 ?e'
  Pop $1
  StrCpy $InstallerMutex $0
  ${If} $0 == 0
  ${OrIf} $1 == 183
    MessageBox MB_ICONSTOP "Another Scene Palette install or uninstall is already running. Close it before continuing." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
FunctionEnd
!macroend

!insertmacro DEFINE_PREFLIGHT ""
!insertmacro DEFINE_PREFLIGHT "un."

Section "Scene Palette (required)" SEC_APP
  SectionIn RO
  Call CheckInstallPath
  Call CheckAppClosed
  SetOutPath "$INSTDIR"
  ; Try + checked error flags prevents silent skips or an Ignore button from
  ; registering a partial upgrade. Extract the locked executable before any
  ; other installed file, closing the race after the initial lock preflight.
  SetOverwrite try
  ClearErrors
  File "${PORTABLE_DIR}\scene-palette.exe"
  IfErrors install_failed
  File "${PORTABLE_DIR}\index.html"
  File "${PORTABLE_DIR}\Run Scene Palette.cmd"
  File "${PORTABLE_DIR}\README.txt"
  File "${PORTABLE_DIR}\LICENSE"
  File "${PORTABLE_DIR}\THIRD_PARTY_NOTICES.md"
  File "${PORTABLE_DIR}\build-info.json"
  File "${PORTABLE_DIR}\SHA256SUMS.txt"
  IfErrors install_failed
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  IfErrors install_failed
  CreateShortCut "$SMPROGRAMS\Scene Palette.lnk" "$INSTDIR\scene-palette.exe" "" "$INSTDIR\scene-palette.exe" 0 SW_SHOWNORMAL "" "Open Scene Palette; close its console to stop the server"
  IfErrors install_failed
  WriteRegStr HKCU "${APP_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayName" "Scene Palette"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayVersion" "${APP_VERSION}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "Publisher" "Scene Palette contributors"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\scene-palette.exe"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKCU "${UNINSTALL_KEY}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoRepair" 1
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "EstimatedSize" ${PORTABLE_SIZE_KB}
  IfErrors install_failed
  Goto install_done
  install_failed:
    MessageBox MB_ICONSTOP "Installation could not write all required files. Close Scene Palette, check the destination is writable, and run the installer again." /SD IDOK
    SetErrorLevel 2
    Abort
  install_done:
SectionEnd

Section /o "Desktop shortcut" SEC_DESKTOP
  ClearErrors
  CreateShortCut "$DESKTOP\Scene Palette.lnk" "$INSTDIR\scene-palette.exe" "" "$INSTDIR\scene-palette.exe" 0 SW_SHOWNORMAL "" "Open Scene Palette; close its console to stop the server"
  ${If} ${Errors}
    MessageBox MB_ICONSTOP "The desktop shortcut could not be created. Run the installer again to retry." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
  WriteRegDWORD HKCU "${APP_KEY}" "DesktopShortcut" 1
  ${If} ${Errors}
    MessageBox MB_ICONSTOP "The desktop shortcut setting could not be saved. Run the installer again to retry." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
SectionEnd

Section "-Desktop shortcut settings"
  SectionGetFlags ${SEC_DESKTOP} $0
  IntOp $0 $0 & ${SF_SELECTED}
  ${If} $0 == 0
    ReadRegDWORD $0 HKCU "${APP_KEY}" "DesktopShortcut"
    ClearErrors
    ${If} $0 == 1
      Delete "$DESKTOP\Scene Palette.lnk"
      ${If} ${Errors}
        MessageBox MB_ICONSTOP "The desktop shortcut could not be removed. Run the installer again after unlocking the shortcut." /SD IDOK
        SetErrorLevel 2
        Abort
      ${EndIf}
    ${EndIf}
    WriteRegDWORD HKCU "${APP_KEY}" "DesktopShortcut" 0
    ${If} ${Errors}
      MessageBox MB_ICONSTOP "The desktop shortcut setting could not be updated. Run the installer again to retry." /SD IDOK
      SetErrorLevel 2
      Abort
    ${EndIf}
  ${EndIf}
SectionEnd

Function .onInit
  SetShellVarContext current
  SetRegView 64
  ${IfNot} ${RunningX64}
    MessageBox MB_ICONSTOP "Scene Palette requires 64-bit Windows." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
  ${IfNot} ${AtLeastWin10}
    MessageBox MB_ICONSTOP "Scene Palette requires Windows 10 or later." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
  Call AcquireInstallerMutex
  ; NSIS strips /D from $CMDLINE. Read the original command line only to detect
  ; that explicit override; NSIS has already parsed its value into $INSTDIR.
  StrCpy $InstallPathOverride 0
  System::Call 'kernel32::GetCommandLineW() w.r0'
  ClearErrors
  ${GetOptions} "$0" "/D=" $1
  ${IfNot} ${Errors}
    StrCpy $InstallPathOverride 1
  ${EndIf}
  ReadRegStr $0 HKCU "${APP_KEY}" "InstallLocation"
  ${If} $0 != ""
    ${If} $InstallPathOverride == 0
      StrCpy $INSTDIR "$0"
    ${ElseIf} $0 != $INSTDIR
      MessageBox MB_ICONSTOP "Scene Palette is already installed at $0. Upgrade using that location, or uninstall it first." /SD IDOK
      SetErrorLevel 2
      Abort
    ${EndIf}
  ${EndIf}
  Call CheckInstallPath
  ReadRegStr $0 HKCU "${UNINSTALL_KEY}" "DisplayVersion"
  ${If} $0 != ""
    ${VersionCompare} "$0" "${APP_VERSION}" $1
    ${If} $1 == 1
      MessageBox MB_ICONSTOP "A newer Scene Palette version is installed. Uninstall it before installing an older version." /SD IDOK
      SetErrorLevel 2
      Abort
    ${EndIf}
  ${EndIf}
  ReadRegDWORD $0 HKCU "${APP_KEY}" "DesktopShortcut"
  ${If} $0 == 1
    !insertmacro SelectSection ${SEC_DESKTOP}
  ${EndIf}
  Call CheckAppClosed
FunctionEnd

Function un.onInit
  SetShellVarContext current
  SetRegView 64
  Call un.AcquireInstallerMutex
  Call un.CheckInstallPath
  ReadRegStr $0 HKCU "${APP_KEY}" "InstallLocation"
  ${If} $0 != $INSTDIR
    MessageBox MB_ICONSTOP "This uninstaller does not match the registered Scene Palette installation. Run Uninstall.exe from the installed app folder." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
  Call un.CheckAppClosed
FunctionEnd

Section "Uninstall"
  Call un.CheckInstallPath
  Call un.CheckAppClosed
  SetOutPath "$TEMP"
  ClearErrors
  Delete "$INSTDIR\scene-palette.exe"
  IfErrors uninstall_failed
  Delete "$INSTDIR\index.html"
  Delete "$INSTDIR\Run Scene Palette.cmd"
  Delete "$INSTDIR\README.txt"
  Delete "$INSTDIR\LICENSE"
  Delete "$INSTDIR\THIRD_PARTY_NOTICES.md"
  Delete "$INSTDIR\build-info.json"
  Delete "$INSTDIR\SHA256SUMS.txt"
  IfErrors uninstall_failed
  Delete "$SMPROGRAMS\Scene Palette.lnk"
  IfErrors uninstall_failed
  ReadRegDWORD $0 HKCU "${APP_KEY}" "DesktopShortcut"
  ; Missing optional state is not a file-removal failure (for older/partial installs).
  ClearErrors
  ${If} $0 == 1
    Delete "$DESKTOP\Scene Palette.lnk"
  ${EndIf}
  Delete "$INSTDIR\Uninstall.exe"
  IfErrors uninstall_failed
  DeleteRegKey HKCU "${UNINSTALL_KEY}"
  DeleteRegKey HKCU "${APP_KEY}"
  ; Nonrecursive removal deliberately preserves exports and any unknown files.
  RMDir "$INSTDIR"
  Goto uninstall_done
  uninstall_failed:
    MessageBox MB_ICONSTOP "Uninstall could not remove all app files. Close Scene Palette and run Uninstall.exe again." /SD IDOK
    SetErrorLevel 2
    Abort
  uninstall_done:
SectionEnd
