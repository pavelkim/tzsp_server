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

## Build and Run

```bash
make deps      # Download dependencies
make build     # Build for current platform
```

```bash
# Run with default config
./tzsp_server

# Run with custom config
./tzsp_server -config /path/to/config.yaml
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
  # File logging configuration
  file:
    # Enable file logging
    enabled: true
    # Log level: debug, info, warn, error
    level: "info"
    # Log format: text, json
    format: "text"
    # File path for application logs
    path: "tzsp_server.log"
  
  # Console logging configuration
  console:
    # Enable console (stdout) logging
    enabled: true
    # Log level: debug, info, warn, error
    level: "info"
    # Log format: text, json (text recommended for console)
    format: "text"
```

## Usage examples

### Mikrotik and Packet Sniffer

Configure the Packet Sniffer on your Mikrotik router to stream TZSP packets:

```mikrotik
/tool sniffer set streaming-enabled=yes streaming-server=<server_ip>:37008
/tool sniffer start
```

### Mikrotik firewall rule

Configure firewall on your Mikrotik router to send TZSP packets:

```mikrotik
/ip firewall mangle
add action=sniff-tzsp chain=prerouting disabled=yes sniff-target=<server_ip> sniff-target-port=37008 src-address=<source_device_ip>
```

### TZSP Server

Configure the TZSP Server in `config.yaml`:

```yaml
server:
  listen_addr: "0.0.0.0:37008"
  buffer_size: 65536

output:
  file:
    enabled: true
    output_file: "packets.log"
    format: "json"

logging:
  console:
    enabled: true
    level: "info"
    format: "json"
```

Prepare directories for logs and data:

```bash
mkdir -pv ./logs ./data
```

Run the TZSP Server in Docker:

```bash
docker run -d \
  -ti \
  --name tzsp_server \
  --network host \
  --restart unless-stopped \
  -v "${pwd}/config.docker.yaml:/app/config.yaml:ro" \
  -v "${pwd}/data:/app/data" \
  -v "${pwd}/logs:/app/logs" \
  -e TZ=UTC \
  ghcr.io/pavelkim/tzsp_server:latest
```

## License

MIT
