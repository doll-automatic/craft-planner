@echo off
setlocal EnableExtensions
cd /d "%~dp0"

rem Machine/User PATH is loaded so go works even when this cmd has a stale PATH.
rem Japanese text is Base64 so this file stays ASCII and cmd can parse it.
set "PS=%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe"
set "PATHFILE=%TEMP%\gpm-path.txt"

"%PS%" -NoProfile -Command "$p = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User'); [IO.File]::WriteAllText($env:TEMP + '\gpm-path.txt', $p, [Text.Encoding]::Default)"
if errorlevel 1 (
  call :say "UEFUSCDjga7oqq3jgb/ovrzjgb/jgavlpLHmlZfjgZfjgb7jgZfjgZ/jgII="
  set "EXITCODE=1"
  goto :finish
)
for /f "usebackq eol=# delims=" %%I in ("%PATHFILE%") do set "PATH=%%I"
del /f /q "%PATHFILE%" >nul 2>&1

where go >nul 2>&1
if errorlevel 1 (
  call :say "Z28g44Kz44Oe44Oz44OJ44GM6KaL44Gk44GL44KK44G+44Gb44KT44CCTWFjaGluZSDjgaggVXNlciDjga4gUEFUSCDjgpLnorroqo3jgZfjgabjgY/jgaDjgZXjgYTjgII="
  set "EXITCODE=1"
  goto :finish
)

call :say "44OT44Or44OJ44GX44Gm44GE44G+44GZLi4u"
go build -o game-production-manager.exe .
if errorlevel 1 (
  call :say "44OT44Or44OJ44Gr5aSx5pWX44GX44Gf44Gf44KBIFpJUCDjga/kvZzmiJDjgZfjgb7jgZvjgpPjgII="
  set "EXITCODE=1"
  goto :finish
)
if not exist "game-production-manager.exe" (
  call :say "44OT44Or44OJ5b6M44GuIGV4ZSDjgYzopovjgaTjgYvjgonjgarjgYTjgZ/jgoEgWklQIOOBr+S9nOaIkOOBl+OBvuOBm+OCk+OAgg=="
  set "EXITCODE=1"
  goto :finish
)

call :say "WklQIOOCkuS9nOaIkOOBl+OBpuOBhOOBvuOBmS4uLg=="
"%PS%" -NoProfile -ExecutionPolicy Bypass -Command ^
  "$ErrorActionPreference='Stop';" ^
  "$root=(Get-Location).Path;" ^
  "$stage=Join-Path $root 'dist\stage';" ^
  "$zip=Join-Path $root 'dist\game-production-manager.zip';" ^
  "if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force };" ^
  "New-Item -ItemType Directory -Path (Join-Path $root 'dist') -Force | Out-Null;" ^
  "New-Item -ItemType Directory -Path $stage | Out-Null;" ^
  "Copy-Item -LiteralPath (Join-Path $root 'game-production-manager.exe') -Destination (Join-Path $stage 'game-production-manager.exe');" ^
  "Copy-Item -LiteralPath (Join-Path $root 'data') -Destination $stage -Recurse;" ^
  "$usage=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('5L2/44GE5pa5CjEuIOOBk+OBruODleOCqeODq+ODgOOBpyBnYW1lLXByb2R1Y3Rpb24tbWFuYWdlci5leGUg44KS6LW35YuV44GZ44KL77yI44OA44OW44Or44Kv44Oq44OD44Kv5Y+v77yJ44CCCjIuIOihqOekuuOBleOCjOOBnyBVUkwg44KS44OW44Op44Km44K244Gn6ZaL44GP44CC5pei5a6a44GvIGh0dHA6Ly9sb2NhbGhvc3Q6ODA4MC8KMy4g44Od44O844OI44Go44OH44O844K/44OV44Kp44Or44OA44KS5oyH5a6a44GZ44KLOiBnYW1lLXByb2R1Y3Rpb24tbWFuYWdlci5leGUgODA4MCBkYXRhCuOCsuODvOODoOODh+ODvOOCv+OBryBkYXRhIOODleOCqeODq+ODgOOBriBZQU1M44CC44Ki44Kk44Kz44Oz44GvIGRhdGFcaWNvblzjgrLjg7zjg6BJRFzjgqLjgqTjg4bjg6BJRC5wbmcK'));" ^
  "$note=-join ([char]0x4F7F,[char]0x3044,[char]0x65B9)+'.txt';" ^
  "[IO.File]::WriteAllText((Join-Path $stage $note), $usage, (New-Object Text.UTF8Encoding $false));" ^
  "if (Test-Path -LiteralPath $zip) { Remove-Item -LiteralPath $zip -Force };" ^
  "Push-Location $stage;" ^
  "try { Compress-Archive -Path @('game-production-manager.exe','data',$note) -DestinationPath $zip -Force } catch { if (Test-Path -LiteralPath $zip) { Remove-Item -LiteralPath $zip -Force }; throw } finally { Pop-Location };" ^
  "Remove-Item -LiteralPath $stage -Recurse -Force;" ^
  "Write-Output ('created: ' + $zip)"
if errorlevel 1 (
  call :say "WklQIOOBruS9nOaIkOOBq+WkseaVl+OBl+OBvuOBl+OBn+OAgg=="
  set "EXITCODE=1"
  goto :finish
)

echo.
call :say "5a6M5LqGOiBkaXN0XGdhbWUtcHJvZHVjdGlvbi1tYW5hZ2VyLnppcA=="
call :say "5bGV6ZaL44GX44Gf44OV44Kp44Or44OA44GnIGdhbWUtcHJvZHVjdGlvbi1tYW5hZ2VyLmV4ZSDjgpLotbfli5XjgZfjgIHooajnpLrjgZXjgozjgZ8gVVJMIOOCkuODluODqeOCpuOCtuOBp+mWi+OBhOOBpuOBj+OBoOOBleOBhOOAgg=="
call :say "5pei5a6a44Gv44Od44O844OIIDgwODDjgIHjg4fjg7zjgr/jga/lkIzjgZjpmo7lsaTjga4gZGF0YSDjgafjgZnjgILlpInmm7TjgZnjgovjgajjgY3jga8gZ2FtZS1wcm9kdWN0aW9uLW1hbmFnZXIuZXhlIDgwODAgZGF0YQ=="
set "EXITCODE=0"
goto :finish

:finish
> "%TEMP%\gpm-cmd.txt" 2>nul set cmdcmdline
findstr /i /c:"%~f0" "%TEMP%\gpm-cmd.txt" >nul
if not errorlevel 1 pause
del /f /q "%TEMP%\gpm-cmd.txt" >nul 2>&1
exit /b %EXITCODE%

:say
"%PS%" -NoProfile -Command "Write-Host ([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%~1')))"
exit /b 0
