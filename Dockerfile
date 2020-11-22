FROM golang:latest

ADD . /app

WORKDIR /app

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/spotify-status

RUN chmod +x ./bin/spotify-status

EXPOSE 8080

CMD ["./bin/spotify-status"]