echo "build linux x64 HerosTime apple..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -trimpath -o ~/Downloads/linux_HerosTime_apple main.go
