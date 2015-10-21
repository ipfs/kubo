@echo OFF

echo "BUILD GOLANG"
cd "%GOROOT%\src"
./make.bat --dist-tool
