; StarCore IDE - NSIS Installer Script
; Build with: makensis build/installer.nsi

!define PRODUCT_NAME "StarCore IDE"
!define PRODUCT_VERSION "1.0.0"
!define PRODUCT_PUBLISHER "StarCore"
!define PRODUCT_WEB_SITE "https://github.com/StarCore/StarCore"
!define PRODUCT_DIR_REGKEY "Software\Microsoft\Windows\CurrentVersion\App Paths\StarCore.exe"
!define PRODUCT_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"

SetCompressor lzma

; MUI 2 Modern Interface
!include "MUI2.nsh"
!include "FileFunc.nsh"

; General
Name "${PRODUCT_NAME} ${PRODUCT_VERSION}"
OutFile "StarCore-Setup-${PRODUCT_VERSION}.exe"
InstallDir "$PROGRAMFILES\StarCore"
InstallDirRegKey HKLM "${PRODUCT_DIR_REGKEY}" ""
RequestExecutionLevel admin

; Interface
!define MUI_ABORTWARNING
!define MUI_ICON "windows\icon.ico"
!define MUI_UNICON "windows\icon.ico"

; Pages
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "..\README.md"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; Languages
!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "English"

Section "StarCore IDE" SEC01
  SetOutPath "$INSTDIR"

  ; Stop running instance
  nsProcess::_FindProcess "StarCore.exe"
  Pop $R0
  ${If} $R0 == 0
    nsProcess::_KillProcess "StarCore.exe"
    Sleep 1000
  ${EndIf}

  ; Main executable
  File "bin\StarCore.exe"

  ; README
  File "..\README.md"

  ; License for installer
  File /oname=license.txt "..\README.md"

  ; Create shortcuts
  CreateDirectory "$SMPROGRAMS\StarCore IDE"
  CreateShortCut "$SMPROGRAMS\StarCore IDE\StarCore IDE.lnk" "$INSTDIR\StarCore.exe"
  CreateShortCut "$DESKTOP\StarCore IDE.lnk" "$INSTDIR\StarCore.exe"

  ; Register
  WriteRegStr HKLM "${PRODUCT_DIR_REGKEY}" "" "$INSTDIR\StarCore.exe"
  WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "DisplayName" "${PRODUCT_NAME}"
  WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "UninstallString" "$INSTDIR\uninst.exe"
  WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "DisplayIcon" "$INSTDIR\StarCore.exe"
  WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "DisplayVersion" "${PRODUCT_VERSION}"
  WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "Publisher" "${PRODUCT_PUBLISHER}"
  WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "URLInfoAbout" "${PRODUCT_WEB_SITE}"

  ; Estimate size
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKLM "${PRODUCT_UNINST_KEY}" "EstimatedSize" "$0"
SectionEnd

Section "Start Menu Shortcuts" SEC02
  CreateShortCut "$SMPROGRAMS\StarCore IDE\Uninstall.lnk" "$INSTDIR\uninst.exe"
SectionEnd

Section Uninstall
  ; Kill running instance
  nsProcess::_FindProcess "StarCore.exe"
  Pop $R0
  ${If} $R0 == 0
    nsProcess::_KillProcess "StarCore.exe"
  ${EndIf}

  Delete "$INSTDIR\StarCore.exe"
  Delete "$INSTDIR\README.md"
  Delete "$INSTDIR\uninst.exe"
  Delete "$DESKTOP\StarCore IDE.lnk"

  RMDir /r "$SMPROGRAMS\StarCore IDE"
  RMDir "$INSTDIR"

  DeleteRegKey HKLM "${PRODUCT_UNINST_KEY}"
  DeleteRegKey HKLM "${PRODUCT_DIR_REGKEY}"
SectionEnd

; Post-install: write uninstaller
Section -Post
  WriteUninstaller "$INSTDIR\uninst.exe"
SectionEnd
