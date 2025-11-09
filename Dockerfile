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

# Build bot binary
RUN go build -a -o bot ./cmd/bot/main.go

# Build API server binary
RUN go build -a -o server ./cmd/server/main.go

# Bot stage
FROM alpine:latest AS bot

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs tzdata

# Set timezone
ENV TZ=Asia/Ho_Chi_Minh

# Create app directory
WORKDIR /app

# Copy bot binary from builder
COPY --from=builder /app/bot .
COPY --from=builder /app/init/init.sql ./init/init.sql

# Create volume for SQLite database
VOLUME ["/app/data"]

# Set environment variables
ENV DB_PATH=/app/data/mail.db \
    DB_DRIVER=sqlite3

# Run the bot
CMD ["./bot"]

# API server stage
FROM alpine:latest AS api

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs tzdata

# Set timezone
ENV TZ=Asia/Ho_Chi_Minh

# Create app directory
WORKDIR /app

# Copy server binary from builder
COPY --from=builder /app/server .
COPY --from=builder /app/init/init.sql ./init/init.sql
COPY --from=builder /app/internal/smtp/templates ./internal/smtp/templates

# Create volume for SQLite database
VOLUME ["/app/data"]

# Set environment variables
ENV DB_PATH=/app/data/mail.db \
    DB_DRIVER=sqlite3

# Expose API port
EXPOSE 8080

# Run the API server
CMD ["./server"] 