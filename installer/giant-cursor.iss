; Inno Setup script for Giant Cursor.
; Build with:  iscc installer\giant-cursor.iss
; Expects giant-cursor.exe to already be built at the repository root.

#define AppName "Giant Cursor"
#define AppVersion "1.1.0"
#define AppExe "giant-cursor.exe"

[Setup]
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppName}
DefaultDirName={autopf}\{#AppName}
DefaultGroupName={#AppName}
OutputBaseFilename=GiantCursorSetup
OutputDir=Output
SetupIconFile=..\assets\giant-cursor.ico
UninstallDisplayIcon={app}\{#AppExe}
Compression=lzma2
SolidCompression=yes
; No admin rights needed: installs per-user.
PrivilegesRequired=lowest
ArchitecturesInstallIn64BitMode=x64compatible
WizardStyle=modern
DisableProgramGroupPage=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "spanish"; MessagesFile: "compiler:Languages\Spanish.isl"

[CustomMessages]
; LangCode is passed to the app so the tray menu starts in the chosen language.
english.LangCode=en
spanish.LangCode=es
english.StartupTask=Start Giant Cursor when Windows starts
spanish.StartupTask=Iniciar Giant Cursor con Windows
english.StartupGroup=Startup:
spanish.StartupGroup=Inicio:
english.LaunchApp=Launch Giant Cursor now
spanish.LaunchApp=Iniciar Giant Cursor ahora
english.RestoreCursor=Restore cursor
spanish.RestoreCursor=Restaurar cursor

[Files]
Source: "..\{#AppExe}"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExe}"
Name: "{group}\{cm:RestoreCursor}"; Filename: "{app}\{#AppExe}"; Parameters: "--restore"
Name: "{group}\{cm:UninstallProgram,{#AppName}}"; Filename: "{uninstallexe}"

[Tasks]
Name: "startup"; Description: "{cm:StartupTask}"; GroupDescription: "{cm:StartupGroup}"

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; \
  ValueName: "GiantCursor"; ValueData: """{app}\{#AppExe}"" --silent"; \
  Tasks: startup; Flags: uninsdeletevalue

[Run]
Filename: "{app}\{#AppExe}"; Parameters: "--lang={cm:LangCode}"; \
  Description: "{cm:LaunchApp}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
; Always leave the cursor back to normal when uninstalling.
Filename: "{app}\{#AppExe}"; Parameters: "--restore"; Flags: runhidden; RunOnceId: "RestoreCursor"
