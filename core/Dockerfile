FROM golang:1.25-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o atlas-core cmd/server/main.go
RUN go build -o atlas-seed cmd/seed/main.go

# Run seeder before starting core (in a real production setup, migration jobs are better)
CMD ["sh", "-c", "./atlas-seed && ./atlas-core"]
