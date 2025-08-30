@echo off
REM Build script for Windows

setlocal EnableDelayedExpansion

set APP_NAME=lidarr-deduper
set VERSION=%1
if "%VERSION%"=="" set VERSION=dev
set BUILD_DIR=build

echo Building %APP_NAME% version %VERSION%

REM Clean build directory
if exist %BUILD_DIR% rmdir /s /q %BUILD_DIR%
mkdir %BUILD_DIR%

echo Building for Windows/AMD64...
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-w -s -X main.version=%VERSION%" -o %BUILD_DIR%\%APP_NAME%-windows-amd64.exe .

if !errorlevel! neq 0 (
    echo Failed to build for Windows/AMD64
    exit /b 1
)

echo Building for Linux/AMD64...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-w -s -X main.version=%VERSION%" -o %BUILD_DIR%\%APP_NAME%-linux-amd64 .

if !errorlevel! neq 0 (
    echo Failed to build for Linux/AMD64
    exit /b 1
)

echo Build complete! Artifacts in %BUILD_DIR%\
dir %BUILD_DIR%

endlocal
