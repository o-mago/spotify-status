build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/spotify-status ./src

deploy: build
	fly deploy