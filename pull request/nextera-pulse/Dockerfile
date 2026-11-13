# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/seed ./cmd/seed

# Run stage
FROM alpine:3.20

WORKDIR /app
COPY --from=builder /app/api /app/api
COPY --from=builder /app/seed /app/seed
COPY migrations/ /app/migrations/
COPY .env.example /app/.env

EXPOSE 8080
CMD ["/app/api"]
