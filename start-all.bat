@echo off
title WhiteInfo SMS - Local Launcher
cd /D "%~dp0"

echo ========================================================
echo   WhiteInfo SMS Platform - Starting Full Local Setup
echo ========================================================

echo.
echo [1/5] Checking MySQL Database (:3306)...
netstat -ano | findstr 127.0.0.1:3306 >nul
if %errorlevel% neq 0 (
    if exist "C:\xampp\mysql\bin\mysqld.exe" (
        echo Starting local MySQL from XAMPP...
        start "" /B "C:\xampp\mysql\bin\mysqld.exe" --defaults-file="C:\xampp\mysql\bin\my.ini" --standalone
        timeout /t 2 /nobreak >nul
    ) else (
        echo [WARNING] MySQL not detected on port 3306. Please start MySQL in XAMPP Control Panel.
    )
) else (
    echo MySQL is already running on port 3306.
)

echo.
echo [2/5] Starting Superman Admin API (:8081)...
cd /D "%~dp0superman-api"
start "Superman API (:8081)" cmd /k "superman-api.exe"

echo.
echo [3/5] Starting Customer API (:8082)...
cd /D "%~dp0customer-api"
start "Customer API (:8082)" cmd /k "customer-api.exe"

echo.
echo [4/5] Starting SP Reseller API (:8083)...
cd /D "%~dp0sp-api"
start "SP API (:8083)" cmd /k "sp-api.exe"

echo.
echo [5/5] Starting Frontend (:5173)...
cd /D "%~dp0frontend"
start "Frontend (:5173)" cmd /k "npm run dev"

echo.
echo ========================================================
echo   All 5 components launched!
echo.
echo   * Superman Admin : http://localhost:5173/superman
echo     Login: superman@example.com / superman
echo.
echo   * Customer Portal: http://localhost:5173/customer
echo     Login: otp@whiteinfobd.com / password
echo.
echo   * SP Reseller    : http://localhost:5173/sp
echo     Login: kylee92@example.org / password
echo ========================================================
echo.
echo Opening browser in 3 seconds...
timeout /t 3 /nobreak >nul
start http://localhost:5173/superman

