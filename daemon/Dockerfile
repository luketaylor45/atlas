FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOMAXPROCS=1 go build -p 1 -trimpath -ldflags "-s -w" -o /out/atlas-daemon ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=build /out/atlas-daemon ./

# The daemon runs as root: it drives the Docker socket and hands installed
# files to the game user.
CMD ["./atlas-daemon"]
