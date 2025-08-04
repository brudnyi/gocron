# --- Build Stage ---
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# -ldflags="-w -s" strips debug information and symbols, reducing the binary size
# CGO_ENABLED=0 disables CGO to create a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /gocron ./cmd/gocron

# --- Final Stage ---
FROM alpine:latest

# Add a non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy the configuration file
WORKDIR /app
COPY --from=builder /app/config.yaml ./config.yaml

# Copy the binary from the builder stage
COPY --from=builder /gocron /app/gocron

# Set ownership to the non-root user
RUN chown -R appuser:appgroup /app

# Switch to the non-root user
USER appuser

# Expose the server port
EXPOSE 8080

# Run the application
CMD ["/app/gocron"]
