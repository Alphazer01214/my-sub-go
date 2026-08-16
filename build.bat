@echo off
rem 构建发布版 my-sub-go.exe：
rem -H windowsgui 去掉黑色控制台窗口（启动信息见 GUI 与 logs/ 目录）
rem -s -w 去除符号表、缩小体积
setlocal
cd /d "%~dp0"

echo [1/2] go build -ldflags "-H windowsgui -s -w" ...
go build -ldflags "-H windowsgui -s -w" -o my-sub-go.exe .
if errorlevel 1 (
    echo 构建失败
    exit /b 1
)

echo [2/2] 复制到 release\ 目录 ...
if not exist release mkdir release
copy /y my-sub-go.exe release\my-sub-go.exe >nul
if exist release\config (
    xcopy /y /q config\conf.json release\config\ >nul 2>nul
)

echo 构建完成: my-sub-go.exe（发布版，双击启动无控制台窗口）
echo 提示: 调试时可不加 ldflags 直接 go build，保留控制台查看日志。
