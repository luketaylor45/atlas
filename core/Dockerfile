FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOMAXPROCS=1 go build -p 1 -trimpath -ldflags "-s -w" -o /out/atlas-core ./cmd/server \
 && CGO_ENABLED=0 GOMAXPROCS=1 go build -p 1 -trimpath -ldflags "-s -w" -o /out/atlas-seed ./cmd/seed

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H -u 10001 atlas

WORKDIR /app
COPY --from=build /out/atlas-core /out/atlas-seed ./

# Core needs no privileges: it only talks to the database and the nodes.
USER atlas

# The catalog seeder runs first; it also applies database migrations.
CMD ["sh", "-c", "./atlas-seed && exec ./atlas-core"]
