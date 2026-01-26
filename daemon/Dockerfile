FROM golang:1.25-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN GOMAXPROCS=1 go build -p 1 -o atlas-daemon cmd/server/main.go

CMD ["./atlas-daemon"]
