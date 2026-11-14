@echo off
title WhiteInfo SMS - Stop All Services
cd /D "%~dp0"

echo Stopping Go microservices...
taskkill /F /IM superman-api.exe /T 2>nul
taskkill /F /IM customer-api.exe /T 2>nul
taskkill /F /IM sp-api.exe /T 2>nul

echo Done! Go API servers stopped.

