# Dockerfile.node
FROM golang:1.20-alpine

WORKDIR /app

COPY go.mod go.work ./
RUN go mod download

COPY ./nodes .

RUN go build -o node *.go

CMD ["./node"]