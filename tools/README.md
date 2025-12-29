# QingPing Sensor Server

A simple Python HTTP server for receiving and displaying QingPing sensor data from the TZSP server's QingPing plugin.

## Features

- **POST endpoint** at `/` or `/sensor-data` - Receives JSON sensor data from the TZSP QingPing plugin

## Testing

You can test the server with curl:

```bash
# Send test sensor data
curl -X POST http://localhost:8080/sensor-data \
  -H "Content-Type: application/json" \
  -H "X-Source-IP: 127.0.0.1" \
  -H "X-Destination-IP: 100.101.102.103" \
  -H "X-MQTT-Topic: /test/sensor/update" \
  -d '{
    "id": 1,
    "mac": "582D34009704",
    "sensorData": [{
      "temperature": {"status": 0, "value": 22.5},
      "humidity": {"status": 0, "value": 45.2},
      "co2": {"status": 0, "value": 450},
      "pm25": {"status": 0, "value": 12},
      "pm10": {"status": 0, "value": 15},
      "tvoc": {"status": 0, "value": 100},
      "battery": {"status": 1, "value": 85}
    }]
  }'

# Check stats
curl http://localhost:8080/stats
```
