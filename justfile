build:
    env GOOS=linux GOARCH=386 go build -o ./bin/phptooling_linux_386 .
    env GOOS=linux GOARCH=amd64 go build -o ./bin/phptooling_linux_amd64 .
    env GOOS=linux GOARCH=arm64 go build -o ./bin/phptooling_linux_arm64 .
    env GOOS=windows GOARCH=386 go build -o ./bin/phptooling_windows_386 .
    env GOOS=windows GOARCH=amd64 go build -o ./bin/phptooling_windows_amd64 .
    env GOOS=darwin GOARCH=amd64 go build -o ./bin/phptooling_darwin_amd64 .
    env GOOS=darwin GOARCH=arm64 go build -o ./bin/phptooling_darwin_arm64 .