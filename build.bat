@echo off
:: ─── cimbar-recv-pc Windows build script ──────────────────────────────────
:: Run this on Windows (cmd.exe or PowerShell) to compile the release binary.
:: Requires: go (https://go.dev/dl/)
:: Output  : release\cimbar-recv-pc.exe

setlocal enabledelayedexpansion

set BINARY=cimbar-recv-pc
set OUT_DIR=release
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0

:: ── Resolve version from git, fall back to "dev" ──────────────────────────
for /f "delims=" %%v in ('git describe --tags --always --dirty 2^>nul') do set VERSION=%%v
if "%VERSION%"=="" set VERSION=dev

set LDFLAGS=-s -w -X main.version=%VERSION%

echo === cimbar-recv-pc build ===
echo Version  : %VERSION%
echo Output   : %OUT_DIR%\%BINARY%.exe
echo.

:: ── Create output directory ────────────────────────────────────────────────
if not exist %OUT_DIR% mkdir %OUT_DIR%

:: ── Build ──────────────────────────────────────────────────────────────────
go build -trimpath -ldflags "%LDFLAGS%" -o %OUT_DIR%\%BINARY%.exe .

if %errorlevel% neq 0 (
    echo.
    echo [ERR] Build failed. Make sure 'go' is installed and in PATH.
    echo       Download from: https://go.dev/dl/
    exit /b 1
)

:: ── Report size ────────────────────────────────────────────────────────────
for %%f in (%OUT_DIR%\%BINARY%.exe) do set SIZE=%%~zf
set /a SIZE_KB=%SIZE% / 1024
echo [OK] %OUT_DIR%\%BINARY%.exe  (%SIZE_KB% KB)
endlocal
