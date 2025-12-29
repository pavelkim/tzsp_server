# TZSP Server

A high-performance TZSP (TaZmen Sniffer Protocol) server implementation in Go that receives, decodes, and exports network packets from Mikrotik devices.

## Features

- TZSP protocol decoding
- UDP server for receiving TZSP packets
- Encapsulated packet decoding (Ethernet, IP, TCP, UDP, etc.)
- PCAP file output (compatible with Wireshark)
- NetFlow v5/v9 export
- Structured logging with packet metadata
- Configuration file in YAML

## Build

```bash
make deps      # Download dependencies
make build     # Build for current platform
```

or

```bash
go build -o tzsp_server ./cmd/tzsp_server
```

## Configuration

Edit `config.yaml` to configure the server:

```yaml
server:
  # UDP port to listen on for TZSP packets
  listen_addr: "0.0.0.0:37008"
  # Buffer size for UDP packets
  buffer_size: 65536

output:
  # File output for packet metadata
  file:
    # Enable file output for packet metadata
    enabled: true
    # Output file path for packet metadata
    output_file: "packets.log"
    # Format: text, json
    format: "json"

  # PCAP file output
  pcap:
    # Enable PCAP file output
    enabled: true
    # Output file path
    output_file: "captured_packets.pcap"
    # Maximum file size in MB before rotation
    max_size_mb: 100
    # Maximum number of backup files
    max_backups: 5

  # NetFlow export
  netflow:
    # Enable NetFlow export
    enabled: true
    # NetFlow collector address
    collector_addr: "127.0.0.1:2055"
    # NetFlow version (5 or 9)
    version: 5
    # Flow timeout in seconds
    flow_timeout: 60
    # Active flow timeout in seconds
    active_timeout: 120

  # QingPing sensor data export
  qingping:
    # Enable QingPing export
    enabled: true
    # Packet filter configuration
    filter:
      # Source IP address (empty = any)
      src_ip: "127.0.0.1"
      # Destination IP address (empty = any)
      dst_ip: "101.102.103.104"
      # Destination port (0 = any)
      dst_port: 11883
      # Protocol: tcp, udp, icmp (empty = any)
      protocol: "tcp"
    # Strict JSON validation - if true, invalid JSON will fail processing
    strict_json: false
    # Upstream HTTP server URL for data submission
    upstream_url: "http://localhost:8080/sensor-data"
    # Ignore SSL certificate validation errors
    ignore_ssl: false
    # Ignore non-OK HTTP response codes (don't log as errors)
    ignore_http_errors: true

logging:
  # Application log level: debug, info, warn, error
  level: "info"
  # Application log format: text, json
  format: "text"
  # Enable console (stdout) logging for application logs
  console_output: true
  # Console log level: debug, info, warn, error
  console_level: "info"
  # Console log format: text, json (text recommended for console)
  console_format: "text"

```

## Usage

```bash
# Run with default config
./tzsp_server

# Run with custom config
./tzsp_server -config /path/to/config.yaml
```

## Mikrotik Configuration

Configure your Mikrotik router to send TZSP packets:

```
/tool sniffer set streaming-enabled=yes streaming-server=<server_ip>:37008
/tool sniffer start
```

## License

MIT
