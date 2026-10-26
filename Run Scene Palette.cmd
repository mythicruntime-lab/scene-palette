@echo off
setlocal
cd /d "%~dp0"

if exist "%~dp0scene-palette.exe" goto executable
if not exist "%~dp0go.mod" goto standalone
if not exist "%~dp0main.go" goto standalone

set "SP_GO=go"
where go >nul 2>nul
if errorlevel 1 (
  if exist "%ProgramFiles%\Go\bin\go.exe" (
    set "SP_GO=%ProgramFiles%\Go\bin\go.exe"
  ) else (
    goto standalone
  )
)

echo Starting Scene Palette...
echo Close this window to stop the local server.
"%SP_GO%" run . serve -open
goto finished

:executable
echo Starting Scene Palette...
echo Close this window to stop the local server.
"%~dp0scene-palette.exe" serve -open
goto finished

:standalone
if not exist "%~dp0index.html" (
  echo Scene Palette could not find its executable or index.html.
  pause
  exit /b 1
)
echo Opening the standalone JavaScript frontend.
start "" "%~dp0index.html"
exit /b 0

:finished
if not errorlevel 1 exit /b 0
echo.
echo Scene Palette stopped with an error.
pause
exit /b 1
