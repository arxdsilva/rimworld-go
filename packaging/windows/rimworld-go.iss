; The Windows installer (Inno Setup 6). scripts/package_windows.ps1 compiles
; it with these defines:
;   /DAppTag=v0.3.0-rc1      the release tag (file name, shown version)
;   /DAppNumVersion=0.3.0.0  digits only, for the file's version resource
;   /DStageDir=...           the folder scripts/package_release.sh staged
;   /DOutDir=...             where the setup .exe goes
;   /DIconFile=...           packaging\icons\rimworld-go.ico (left out only
;                            under ALLOW_NO_ICON=1, for a local test)
; It installs for the current user by default (no administrator prompt,
; %LOCALAPPDATA%\Programs\rimworld-go); the dialog offers all users too.
; The AppId never changes: it is how Windows knows an update from a new app.

#ifndef AppTag
  #error Pass /DAppTag=<tag>
#endif
#ifndef StageDir
  #error Pass /DStageDir=<staged folder>
#endif
#ifndef AppNumVersion
  #define AppNumVersion "0.0.0.0"
#endif
#ifndef OutDir
  #define OutDir "..\..\dist"
#endif

[Setup]
AppId={{8E9F4F50-1F65-4B3D-80B2-68B6D6C84520}
AppName=rimworld-go
AppVersion={#AppTag}
AppVerName=rimworld-go {#AppTag}
AppPublisher=rimworld-go
AppPublisherURL=https://github.com/arxdsilva/rimworld-go
AppSupportURL=https://github.com/arxdsilva/rimworld-go/issues
AppUpdatesURL=https://github.com/arxdsilva/rimworld-go/releases
VersionInfoVersion={#AppNumVersion}
VersionInfoProductVersion={#AppNumVersion}
VersionInfoProductTextVersion={#AppTag}
VersionInfoDescription=rimworld-go setup
DefaultDirName={autopf}\rimworld-go
DefaultGroupName=rimworld-go
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=commandline dialog
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
OutputDir={#OutDir}
OutputBaseFilename=rimworld-go-{#AppTag}-windows-setup
#ifdef IconFile
SetupIconFile={#IconFile}
#endif
UninstallDisplayIcon={app}\rimworld-go.exe
UninstallDisplayName=rimworld-go
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#StageDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[InstallDelete]
; a new version replaces the old assets whole, so no stale file outlives it
Type: filesandordirs; Name: "{app}\assets"

[Icons]
Name: "{autoprograms}\rimworld-go"; Filename: "{app}\rimworld-go.exe"; WorkingDir: "{app}"
Name: "{autodesktop}\rimworld-go"; Filename: "{app}\rimworld-go.exe"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\rimworld-go.exe"; Description: "{cm:LaunchProgram,rimworld-go}"; WorkingDir: "{app}"; Flags: nowait postinstall skipifsilent
