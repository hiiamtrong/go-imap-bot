# Build stage
FROM golang:1.23-alpine AS builder

# Install git and build dependencies
RUN apk add --no-cache git build-base

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -o main ./cmd/bot/main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs tzdata

# Set timezone
ENV TZ=Asia/Ho_Chi_Minh

# Create app directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/main .
COPY --from=builder /app/.env.example .env

# Create volume for SQLite database
VOLUME ["/app/data"]

# Set environment variables
ENV DB_PATH=/app/data/mail.db \
    DB_DRIVER=sqlite3

# Run the application
CMD ["./main"] 