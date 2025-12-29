#!/usr/bin/env python3
"""
QingPing Sensor Data Server

A simple HTTP server that receives sensor data from the TZSP QingPing plugin
and displays it in a human-readable format.

Usage:
    python3 qingping_sensor_server.py [--host HOST] [--port PORT]

The server accepts POST requests with JSON sensor data and displays
the latest readings on the root URL (/).
"""

import argparse
import json
import sys
from datetime import datetime
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Dict, Any, Optional


class SensorDataStore:
    """Stores and manages sensor data readings."""
    
    def __init__(self):
        self.latest_data: Optional[Dict[str, Any]] = None
        self.last_updated: Optional[datetime] = None
        self.total_received: int = 0
        self.error_count: int = 0
    
    def add_reading(self, data: Dict[str, Any], headers: Dict[str, str]):
        """Add a new sensor reading."""
        self.latest_data = {
            'sensor_data': data,
            'metadata': {
                'source_ip': headers.get('X-Source-IP', 'unknown'),
                'destination_ip': headers.get('X-Destination-IP', 'unknown'),
                'destination_port': headers.get('X-Destination-Port', 'unknown'),
                'protocol': headers.get('X-Protocol', 'unknown'),
                'timestamp': headers.get('X-Timestamp', 'unknown'),
                'mqtt_topic': headers.get('X-MQTT-Topic', 'N/A'),
            }
        }
        self.last_updated = datetime.now()
        self.total_received += 1
    
    def increment_errors(self):
        """Increment error counter."""
        self.error_count += 1
    
    def get_html_display(self) -> str:
        """Generate HTML display of sensor data."""
        if not self.latest_data:
            return self._get_no_data_html()
        
        return self._get_data_html()
    
    def _get_no_data_html(self) -> str:
        """Generate HTML for no data state."""
        return f"""
<!DOCTYPE html>
<html>
<head>
    <title>QingPing Sensor Monitor</title>
    <meta http-equiv="refresh" content="5">
    <style>
        body {{ font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }}
        .container {{ max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }}
        h1 {{ color: #333; border-bottom: 2px solid #4CAF50; padding-bottom: 10px; }}
        .status {{ padding: 15px; background: #fff3cd; border-left: 4px solid #ffc107; margin: 20px 0; }}
        .stats {{ display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin: 20px 0; }}
        .stat-box {{ background: #e3f2fd; padding: 15px; border-radius: 5px; text-align: center; }}
        .stat-label {{ font-size: 12px; color: #666; text-transform: uppercase; }}
        .stat-value {{ font-size: 24px; font-weight: bold; color: #1976d2; }}
        .footer {{ margin-top: 30px; padding-top: 20px; border-top: 1px solid #ddd; color: #666; font-size: 12px; text-align: center; }}
    </style>
</head>
<body>
    <div class="container">
        <h1>QingPing Sensor Monitor</h1>
        
        <div class="status">
            <strong>Status:</strong> Waiting for sensor data...
            <br>
            <small>The page will auto-refresh every 5 seconds.</small>
        </div>
        
        <div class="stats">
            <div class="stat-box">
                <div class="stat-label">Total Received</div>
                <div class="stat-value">{self.total_received}</div>
            </div>
            <div class="stat-box">
                <div class="stat-label">Errors</div>
                <div class="stat-value">{self.error_count}</div>
            </div>
        </div>
        
        <div class="footer">
            QingPing Sensor Data Server | Last refresh: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}
        </div>
    </div>
</body>
</html>
"""
    
    def _get_data_html(self) -> str:
        """Generate HTML with sensor data."""
        data = self.latest_data['sensor_data']
        meta = self.latest_data['metadata']
        
        # Extract sensor readings if available
        sensor_data_html = ""
        if 'sensorData' in data and isinstance(data['sensorData'], list) and len(data['sensorData']) > 0:
            latest_reading = data['sensorData'][0]
            sensor_data_html = f"""
        <div class="sensors">
            <h2>Latest Sensor Readings</h2>
            <div class="sensor-grid">
                {self._format_sensor_value('Temperature', latest_reading.get('temperature', {}), '°C')}
                {self._format_sensor_value('Humidity', latest_reading.get('humidity', {}), '%')}
                {self._format_sensor_value('CO2', latest_reading.get('co2', {}), 'ppm')}
                {self._format_sensor_value('PM2.5', latest_reading.get('pm25', {}), 'μg/m³')}
                {self._format_sensor_value('PM10', latest_reading.get('pm10', {}), 'μg/m³')}
                {self._format_sensor_value('TVOC', latest_reading.get('tvoc', {}), 'ppb')}
                {self._format_sensor_value('Battery', latest_reading.get('battery', {}), '%')}
            </div>
        </div>
"""
        
        # Format raw JSON
        raw_json = json.dumps(data, indent=2)
        
        return f"""
<!DOCTYPE html>
<html>
<head>
    <title>QingPing Sensor Monitor</title>
    <meta http-equiv="refresh" content="5">
    <style>
        body {{ font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }}
        .container {{ max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }}
        h1 {{ color: #333; border-bottom: 2px solid #4CAF50; padding-bottom: 10px; }}
        h2 {{ color: #555; margin-top: 30px; border-bottom: 1px solid #ddd; padding-bottom: 8px; }}
        .status {{ padding: 15px; background: #d4edda; border-left: 4px solid #28a745; margin: 20px 0; }}
        .status.online {{ background: #d4edda; border-left-color: #28a745; }}
        .stats {{ display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin: 20px 0; }}
        .stat-box {{ background: #e3f2fd; padding: 15px; border-radius: 5px; text-align: center; }}
        .stat-label {{ font-size: 12px; color: #666; text-transform: uppercase; }}
        .stat-value {{ font-size: 24px; font-weight: bold; color: #1976d2; }}
        .sensor-grid {{ display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin: 15px 0; }}
        .sensor-card {{ background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 20px; border-radius: 8px; text-align: center; }}
        .sensor-card.warning {{ background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%); }}
        .sensor-card.good {{ background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%); }}
        .sensor-name {{ font-size: 14px; opacity: 0.9; margin-bottom: 5px; }}
        .sensor-value {{ font-size: 32px; font-weight: bold; margin: 10px 0; }}
        .sensor-unit {{ font-size: 14px; opacity: 0.8; }}
        .sensor-status {{ font-size: 12px; margin-top: 5px; }}
        .metadata {{ background: #f8f9fa; padding: 15px; border-radius: 5px; margin: 20px 0; }}
        .metadata-row {{ display: grid; grid-template-columns: 150px 1fr; padding: 8px 0; border-bottom: 1px solid #dee2e6; }}
        .metadata-label {{ font-weight: bold; color: #495057; }}
        .metadata-value {{ color: #6c757d; font-family: monospace; }}
        .json-data {{ background: #282c34; color: #abb2bf; padding: 20px; border-radius: 5px; overflow-x: auto; margin: 20px 0; }}
        .json-data pre {{ margin: 0; font-family: 'Courier New', monospace; font-size: 13px; }}
        .footer {{ margin-top: 30px; padding-top: 20px; border-top: 1px solid #ddd; color: #666; font-size: 12px; text-align: center; }}
    </style>
</head>
<body>
    <div class="container">
        <h1>QingPing Sensor Monitor</h1>
        
        <div class="status online">
            <strong>Status:</strong> Receiving data
            <br>
            <small>Last updated: {self.last_updated.strftime('%Y-%m-%d %H:%M:%S')}</small>
        </div>
        
        <div class="stats">
            <div class="stat-box">
                <div class="stat-label">Total Received</div>
                <div class="stat-value">{self.total_received}</div>
            </div>
            <div class="stat-box">
                <div class="stat-label">Errors</div>
                <div class="stat-value">{self.error_count}</div>
            </div>
            <div class="stat-box">
                <div class="stat-label">Device MAC</div>
                <div class="stat-value" style="font-size: 16px;">{data.get('mac', 'N/A')}</div>
            </div>
        </div>
        
        {sensor_data_html}
        
        <h2>Packet Metadata</h2>
        <div class="metadata">
            <div class="metadata-row">
                <div class="metadata-label">Source IP:</div>
                <div class="metadata-value">{meta['source_ip']}</div>
            </div>
            <div class="metadata-row">
                <div class="metadata-label">Destination IP:</div>
                <div class="metadata-value">{meta['destination_ip']}</div>
            </div>
            <div class="metadata-row">
                <div class="metadata-label">Destination Port:</div>
                <div class="metadata-value">{meta['destination_port']}</div>
            </div>
            <div class="metadata-row">
                <div class="metadata-label">Protocol:</div>
                <div class="metadata-value">{meta['protocol']}</div>
            </div>
            <div class="metadata-row">
                <div class="metadata-label">MQTT Topic:</div>
                <div class="metadata-value">{meta['mqtt_topic']}</div>
            </div>
            <div class="metadata-row">
                <div class="metadata-label">Timestamp:</div>
                <div class="metadata-value">{meta['timestamp']}</div>
            </div>
        </div>
        
        <h2>Raw JSON Data</h2>
        <div class="json-data">
            <pre>{raw_json}</pre>
        </div>
        
        <div class="footer">
            QingPing Sensor Data Server | Auto-refresh every 5 seconds
        </div>
    </div>
</body>
</html>
"""
    
    def _format_sensor_value(self, name: str, sensor_obj: Dict[str, Any], unit: str) -> str:
        """Format a single sensor value as HTML."""
        if not sensor_obj or 'value' not in sensor_obj:
            return ""
        
        value = sensor_obj['value']
        status = sensor_obj.get('status', 0)
        
        # Determine card class based on sensor type and value
        card_class = "sensor-card"
        if name == 'CO2' and value > 1000:
            card_class += " warning"
        elif name in ('PM2.5', 'PM10') and value > 35:
            card_class += " warning"
        else:
            card_class += " good"
        
        status_text = "Normal" if status == 0 else f"Status: {status}"
        
        return f"""
                <div class="{card_class}">
                    <div class="sensor-name">{name}</div>
                    <div class="sensor-value">{value}</div>
                    <div class="sensor-unit">{unit}</div>
                    <div class="sensor-status">{status_text}</div>
                </div>
"""


class SensorHTTPRequestHandler(BaseHTTPRequestHandler):
    """HTTP request handler for sensor data."""
    
    # Class-level data store
    data_store = SensorDataStore()
    
    def log_message(self, format, *args):
        """Override to customize logging."""
        timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
        sys.stderr.write(f"[{timestamp}] {format % args}\n")
    
    def do_GET(self):
        """Handle GET requests."""
        if self.path == '/' or self.path == '/index.html':
            self.send_response(200)
            self.send_header('Content-type', 'text/html')
            self.end_headers()
            
            html = self.data_store.get_html_display()
            self.wfile.write(html.encode('utf-8'))
        
        elif self.path == '/stats':
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            
            stats = {
                'total_received': self.data_store.total_received,
                'error_count': self.data_store.error_count,
                'last_updated': self.data_store.last_updated.isoformat() if self.data_store.last_updated else None,
                'has_data': self.data_store.latest_data is not None
            }
            self.wfile.write(json.dumps(stats, indent=2).encode('utf-8'))
        
        else:
            self.send_error(404, 'Not Found')
    
    def do_POST(self):
        """Handle POST requests with sensor data."""
        if self.path == '/' or self.path == '/sensor-data':
            content_length = int(self.headers.get('Content-Length', 0))
            
            if content_length == 0:
                self.send_error(400, 'Empty request body')
                self.data_store.increment_errors()
                return
            
            try:
                # Read and parse JSON data
                post_data = self.rfile.read(content_length)
                data = json.loads(post_data.decode('utf-8'))
                
                # Extract headers
                headers = {
                    'X-Source-IP': self.headers.get('X-Source-IP', ''),
                    'X-Destination-IP': self.headers.get('X-Destination-IP', ''),
                    'X-Destination-Port': self.headers.get('X-Destination-Port', ''),
                    'X-Protocol': self.headers.get('X-Protocol', ''),
                    'X-Timestamp': self.headers.get('X-Timestamp', ''),
                    'X-MQTT-Topic': self.headers.get('X-MQTT-Topic', ''),
                }
                
                # Store the data
                self.data_store.add_reading(data, headers)
                
                # Log receipt
                mac = data.get('mac', 'unknown')
                self.log_message(f"Received sensor data from MAC: {mac} (Source: {headers['X-Source-IP']})")
                
                # Send success response
                self.send_response(200)
                self.send_header('Content-type', 'application/json')
                self.end_headers()
                
                response = {
                    'status': 'success',
                    'message': 'Sensor data received',
                    'received_at': datetime.now().isoformat()
                }
                self.wfile.write(json.dumps(response).encode('utf-8'))
            
            except json.JSONDecodeError as e:
                self.send_error(400, f'Invalid JSON: {str(e)}')
                self.data_store.increment_errors()
                self.log_message(f"JSON parse error: {str(e)}")
            
            except Exception as e:
                self.send_error(500, f'Internal server error: {str(e)}')
                self.data_store.increment_errors()
                self.log_message(f"Error processing request: {str(e)}")
        
        else:
            self.send_error(404, 'Not Found')


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='QingPing Sensor Data Server - Receives and displays sensor data from TZSP QingPing plugin'
    )
    parser.add_argument(
        '--host',
        default='0.0.0.0',
        help='Host to bind to (default: 0.0.0.0)'
    )
    parser.add_argument(
        '--port',
        type=int,
        default=8080,
        help='Port to listen on (default: 8080)'
    )
    
    args = parser.parse_args()
    
    server_address = (args.host, args.port)
    httpd = HTTPServer(server_address, SensorHTTPRequestHandler)
    
    print(f"========================================")
    print(f"QingPing Sensor Data Server")
    print(f"========================================")
    print(f"Listening on: http://{args.host}:{args.port}")
    print(f"View dashboard: http://localhost:{args.port}/")
    print(f"Stats endpoint: http://localhost:{args.port}/stats")
    print(f"========================================")
    print(f"Waiting for sensor data...")
    print(f"Press Ctrl+C to stop")
    print(f"========================================")
    
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        print("\n\nShutting down server...")
        httpd.shutdown()
        print("Server stopped.")


if __name__ == '__main__':
    main()
