FROM golang:latest

ADD . /app

WORKDIR /app

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./spotify-status

RUN chmod +x ./spotify-status

EXPOSE 8080

CMD ./main