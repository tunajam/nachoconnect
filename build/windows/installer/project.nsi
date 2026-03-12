Unicode true

####
## NachoConnect NSIS Installer
## Customized to bundle Npcap for packet capture
####

!define INFO_PRODUCTNAME    "NachoConnect"
!define INFO_COMPANYNAME    "TunaJam"
!define INFO_PROJECTNAME    "nachoconnect"
!define INFO_PRODUCTVERSION "0.3.2"
!define INFO_COPYRIGHT      "Copyright 2025 TunaJam"

!include "wails_tools.nsh"

VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

ManifestDPIAware true

!include "MUI2.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"

# Branding
!define MUI_WELCOMEPAGE_TITLE "Welcome to NachoConnect Setup"
!define MUI_WELCOMEPAGE_TEXT "NachoConnect lets you play original Xbox system link games with friends over the internet.$\r$\n$\r$\nThis installer will set up NachoConnect and the Npcap packet capture driver (required for Xbox detection).$\r$\n$\r$\nClick Next to continue."
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_FINISHPAGE_RUN "$INSTDIR\${INFO_PROJECTNAME}.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Launch NachoConnect"
!define MUI_ABORTWARNING

# Pages
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe"
InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
RequestExecutionLevel admin
ShowInstDetails show

Function .onInit
   !insertmacro wails.checkArchitecture
FunctionEnd

# Main install section (required)
Section "NachoConnect (required)" SecMain
    SectionIn RO  ; Read-only, always installed

    !insertmacro wails.setShellContext
    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR
    !insertmacro wails.files

    # Bundle l2tunnel binary
    File "..\..\bin\l2tunnel.exe"

    # Bundle npcap installer
    SetOutPath "$INSTDIR\redist"
    File "..\..\bin\npcap-installer.exe"

    # Install Npcap silently (skip if already installed)
    IfFileExists "$SYSDIR\Npcap\NPFInstall.exe" npcap_done
        DetailPrint "Installing Npcap packet capture driver..."
        nsExec::ExecToLog '"$INSTDIR\redist\npcap-installer.exe" /S /winpcap_mode=yes'
        Pop $0
        DetailPrint "Npcap installer returned: $0"
    npcap_done:

    # Clean up npcap installer from install dir
    RMDir /r "$INSTDIR\redist"

    # Start menu shortcut (always)
    CreateDirectory "$SMPROGRAMS\${INFO_PRODUCTNAME}"
    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${INFO_PROJECTNAME}.exe"
    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}\Uninstall.lnk" "$INSTDIR\uninstall.exe"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols
    !insertmacro wails.writeUninstaller
SectionEnd

# Optional desktop shortcut
Section "Desktop Shortcut" SecDesktop
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${INFO_PROJECTNAME}.exe"
SectionEnd

# Component descriptions
!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
    !insertmacro MUI_DESCRIPTION_TEXT ${SecMain} "Install NachoConnect and the Npcap driver."
    !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} "Create a shortcut on your desktop."
!insertmacro MUI_FUNCTION_DESCRIPTION_END

# Uninstaller
Section "uninstall"
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${INFO_PROJECTNAME}.exe"
    Delete "$INSTDIR\l2tunnel.exe"
    RMDir /r $INSTDIR

    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"
    RMDir /r "$SMPROGRAMS\${INFO_PRODUCTNAME}"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols
    !insertmacro wails.deleteUninstaller
SectionEnd
