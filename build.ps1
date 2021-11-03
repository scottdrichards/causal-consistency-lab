Set-Location ./server
go build -o ../bin/server.exe -gcflags="all=-N -l" 
Set-Location ../
Set-Location ./client
go build -o ../bin/client.exe -gcflags="all=-N -l" 
Set-Location ../
