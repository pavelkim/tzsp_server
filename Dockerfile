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

# Create directories for output files
RUN mkdir -p /app/data /app/logs && \
    chown -R tzsp:tzsp /app

# Switch to non-root user
USER tzsp

# Expose TZSP UDP port
EXPOSE 37008/udp

# Health check (check if process is running)
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD pgrep -f tzsp_server || exit 1

# Run the application
# Config file is optional - defaults will be used if not provided
ENTRYPOINT ["/app/tzsp_server"]
