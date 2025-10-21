# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
WORKDIR /app/examples/prometheus
RUN CGO_ENABLED=0 GOOS=linux go build -o snowflake-prometheus main.go

# Run stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/examples/prometheus/snowflake-prometheus .

EXPOSE 8080

CMD ["./snowflake-prometheus"]
