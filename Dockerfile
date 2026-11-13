# Build stage: keep the Go version in step with go.mod (go 1.26.2).
FROM golang:1.26.2-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/worker ./cmd/worker

# Run stage. The schema is embedded in the binaries; configuration comes from
# the container environment only (no .env file is copied into the image).
FROM alpine:3.20

RUN adduser -D -H pulse
WORKDIR /app
COPY --from=builder /app/api /app/api
COPY --from=builder /app/worker /app/worker
USER pulse

EXPOSE 8080
CMD ["/app/api"]
