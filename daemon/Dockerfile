FROM golang:1.22-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o atlas-daemon cmd/server/main.go

CMD ["./atlas-daemon"]
