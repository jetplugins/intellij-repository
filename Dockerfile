FROM golang:1.16

WORKDIR /app
COPY main.go .
COPY plugins plugins
ENTRYPOINT ["go", "run", "main.go"]