# Build stage
FROM golang:1.25-alpine3.22 AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN make build

# Runtime stage
FROM alpine:3.22

# Install ca-certificates for HTTPS calls (QingPing upstream)
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 tzsp && \
    adduser -D -u 1000 -G tzsp tzsp

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/tzsp_server /app/tzsp_server

# Copy default config (can be overridden with volume mount)
COPY --from=builder /build/config.yaml /app/config.yaml

# Create directories for output files
RUN mkdir -p /app/data /app/logs && \
    chown -R tzsp:tzsp /app

# Switch to non-root user
USER tzsp

# Expose TZSP UDP port
EXPOSE 37008/udp

# Set environment variables
ENV CONFIG_PATH=/app/config.yaml

# Health check (check if process is running)
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD pgrep -f tzsp_server || exit 1

# Run the application
ENTRYPOINT ["/app/tzsp_server"]
CMD ["-config", "/app/config.yaml"]
