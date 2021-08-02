FROM golang:1.16

WORKDIR /app
COPY main.go .
COPY plugins.xml .
COPY plugins plugins
ENTRYPOINT ["go", "run", "main.go"]