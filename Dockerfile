# Build stage
FROM golang:1.25.4@sha256:5d73b7b83dd6e0258ff62832c93b6ea208fbb7727985d265fb49f75f81fc3d1f AS builder

WORKDIR /app

# Install dependencies for building
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o lidarr-deduper .

# Final stage
FROM alpine:latest@sha256:4b7ce07002c69e8f3d704a9c5d6fd3053be500b7f1c69fc0d80990c2ad8dd412

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Create a non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Copy the binary from builder stage
COPY --from=builder /app/lidarr-deduper .

# Copy example config
COPY --from=builder /app/config.example.yaml ./config.example.yaml

# Create directory for config
RUN mkdir -p /app/config && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port (if needed for health checks)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ./lidarr-deduper --help || exit 1

# Default command
ENTRYPOINT ["./lidarr-deduper"]
CMD ["--help"]
