$ErrorActionPreference = 'Stop'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$null = New-Item -ItemType Directory -Force -Path .\dist
go build -trimpath -ldflags "-s -w" -o .\dist\itportal-agent.exe .\cmd\itportal-agent
