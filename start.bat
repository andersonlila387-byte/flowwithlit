@echo off
echo ===================================================
echo      FlowWithLit Backend - Startup Engine
echo ===================================================
echo.
echo [1/3] Stopping any running Go servers...
taskkill /f /im main.exe >nul 2>&1
taskkill /f /im go.exe >nul 2>&1

echo [2/3] Waiting for ports to clear...
timeout /t 2 /nobreak >nul

echo [3/3] Starting Payment Gateway Engine...
echo.
go run cmd/api/main.go
pause
