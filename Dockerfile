# Build stage
FROM golang:1.20-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o streamer ./cmd/streamer

# Final stage
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/streamer .
COPY config.yaml .

EXPOSE 8081

CMD ["./streamer"]
