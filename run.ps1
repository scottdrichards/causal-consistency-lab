./build.ps1

Start-Process powershell -ArgumentList "./bin/server.exe 1001 1002 1003"
Start-Process powershell -ArgumentList "./bin/server.exe 1002 1001 1003"
Start-Process powershell -ArgumentList "./bin/server.exe 1003 1001 1003"
Start-Sleep -s 3
Start-Process powershell -ArgumentList "./bin/client.exe 1001 2001 2002 2003"
Start-Process powershell -ArgumentList "./bin/client.exe 1002 2001 2002 2003"
Start-Process powershell -ArgumentList "./bin/client.exe 1003 2001 2002 2003"
