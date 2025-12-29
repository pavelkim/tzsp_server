package qingping

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pavelkim/tzsp_server/internal/decoder"
	"github.com/pavelkim/tzsp_server/internal/logger"
)

// Filter defines packet filtering criteria
type Filter struct {
	SrcIP    string
	DstIP    string
	DstPort  uint16
	Protocol string // tcp, udp, icmp
}

// Config holds the QingPing exporter configuration
type Config struct {
	Enabled          bool
	Filter           Filter
	StrictJSON       bool // If true, invalid JSON will fail packet processing
	UpstreamURL      string
	IgnoreSSL        bool
	IgnoreHTTPErrors bool // If true, non-2xx responses won't be logged as errors
	Logger           *logger.Logger
}

// Exporter handles QingPing sensor data extraction and forwarding
type Exporter struct {
	config     Config
	httpClient *http.Client
	logger     *logger.Logger
}

// NewExporter creates a new QingPing exporter
func NewExporter(config Config) (*Exporter, error) {
	if !config.Enabled {
		return nil, nil
	}

	if config.UpstreamURL == "" {
		return nil, fmt.Errorf("upstream URL is required")
	}

	// Create HTTP client with optional SSL verification skip
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.IgnoreSSL,
		},
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	e := &Exporter{
		config:     config,
		httpClient: client,
		logger:     config.Logger,
	}

	e.logger.Info("QingPing exporter initialized",
		"upstream_url", config.UpstreamURL,
		"strict_json", config.StrictJSON,
		"ignore_ssl", config.IgnoreSSL,
		"ignore_http_errors", config.IgnoreHTTPErrors)
	e.logger.Info("QingPing filter settings",
		"src_ip", config.Filter.SrcIP,
		"dst_ip", config.Filter.DstIP,
		"dst_port", config.Filter.DstPort,
		"protocol", config.Filter.Protocol)

	return e, nil
}

// matchesFilter checks if a packet matches the configured filter criteria
func (e *Exporter) matchesFilter(pkt *decoder.PacketInfo) bool {
	// Check source IP if configured
	if e.config.Filter.SrcIP != "" {
		if pkt.SrcIP != e.config.Filter.SrcIP {
			return false
		}
	}

	// Check destination IP if configured
	if e.config.Filter.DstIP != "" {
		if pkt.DstIP != e.config.Filter.DstIP {
			return false
		}
	}

	// Check destination port if configured
	if e.config.Filter.DstPort != 0 {
		if pkt.DstPort != e.config.Filter.DstPort {
			return false
		}
	}

	// Check protocol if configured
	if e.config.Filter.Protocol != "" {
		proto := strings.ToLower(e.config.Filter.Protocol)
		pktProto := strings.ToLower(pkt.Protocol)
		if proto != pktProto {
			return false
		}
	}

	return true
}

// extractJSON extracts JSON payload from MQTT or raw packet data
// The QingPing device uses MQTT protocol with length-prefixed strings:
// Format: [control_byte][remaining_length][topic_length_msb][topic_length_lsb][topic_string][payload_length_msb][payload_length_lsb][json_payload]
// This function properly parses the MQTT PUBLISH packet structure
func (e *Exporter) extractJSON(payload []byte) (jsonData []byte, mqttTopic string, err error) {
	// First, try to locate the MQTT topic (starts with '/') to know where to search for JSON
	// This prevents finding '{' bytes in MQTT protocol headers
	searchStartOffset := 0
	topicStart := -1
	topicEnd := -1

	// Look for MQTT topic pattern: '/' followed by printable characters
	firstSlash := bytes.IndexByte(payload, '/')
	if firstSlash != -1 && firstSlash >= 2 {
		// Try to parse MQTT string with length prefix
		lengthMSB := int(payload[firstSlash-2])
		lengthLSB := int(payload[firstSlash-1])
		topicLength := (lengthMSB << 8) | lengthLSB

		// Validate the length seems reasonable
		if topicLength > 0 && topicLength < 256 && firstSlash+topicLength < len(payload) {
			// Verify all bytes in the declared topic are printable ASCII
			allPrintable := true
			for j := firstSlash; j < firstSlash+topicLength && j < len(payload); j++ {
				if payload[j] < 0x20 || payload[j] > 0x7E {
					allPrintable = false
					break
				}
			}

			if allPrintable {
				topicStart = firstSlash
				topicEnd = firstSlash + topicLength
				mqttTopic = string(payload[topicStart:topicEnd])
				// Start searching for JSON after the topic
				searchStartOffset = topicEnd
			}
		}

		// Fallback: if length-based parsing failed, find topic end by scanning for non-printable chars
		if topicStart == -1 {
			topicStart = firstSlash
			topicEnd = firstSlash
			for i := firstSlash; i < len(payload); i++ {
				if payload[i] < 0x20 || payload[i] > 0x7E {
					topicEnd = i
					break
				}
			}
			if topicEnd > topicStart {
				mqttTopic = string(payload[topicStart:topicEnd])
				searchStartOffset = topicEnd
			}
		}
	}

	// Find the first '{' character AFTER the topic (or from start if no topic found)
	// This avoids picking up '{' bytes in MQTT protocol headers
	jsonStart := bytes.IndexByte(payload[searchStartOffset:], '{')
	if jsonStart == -1 {
		return nil, mqttTopic, fmt.Errorf("no JSON data found in payload")
	}
	jsonStart += searchStartOffset // Adjust to absolute offset

	// Find the last '}' character which marks the end of JSON
	jsonEnd := bytes.LastIndexByte(payload, '}')
	if jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, mqttTopic, fmt.Errorf("incomplete JSON data in payload")
	}

	// Extract JSON portion (inclusive of braces)
	jsonData = payload[jsonStart : jsonEnd+1]

	return jsonData, mqttTopic, nil
}

// validateJSON checks if the payload is valid JSON
func (e *Exporter) validateJSON(payload []byte) (bool, error) {
	var js json.RawMessage
	if err := json.Unmarshal(payload, &js); err != nil {
		return false, err
	}
	return true, nil
}

// Export processes a packet and forwards sensor data if it matches criteria
func (e *Exporter) Export(pkt *decoder.PacketInfo) error {
	// Check if packet matches filter
	if !e.matchesFilter(pkt) {
		e.logger.Debug("QingPing packet does not match filter criteria",
			"timestamp", pkt.Timestamp,
			"src_ip", pkt.SrcIP,
			"src_port", pkt.SrcPort,
			"dst_ip", pkt.DstIP,
			"dst_port", pkt.DstPort,
			"protocol", pkt.Protocol,
			"outcome", "skipped")
		return nil
	}

	e.logger.Debug("QingPing filter matched",
		"timestamp", pkt.Timestamp,
		"src_ip", pkt.SrcIP,
		"src_port", pkt.SrcPort,
		"dst_ip", pkt.DstIP,
		"dst_port", pkt.DstPort,
		"protocol", pkt.Protocol,
		"payload_size", len(pkt.PacketData))

	// No payload to process
	if len(pkt.PacketData) == 0 {
		e.logger.Warn("QingPing packet processing failed: no payload",
			"timestamp", pkt.Timestamp,
			"src_ip", pkt.SrcIP,
			"src_port", pkt.SrcPort,
			"dst_ip", pkt.DstIP,
			"dst_port", pkt.DstPort,
			"protocol", pkt.Protocol,
			"outcome", "failed_empty_payload")
		return nil
	}

	// Extract JSON from MQTT payload
	jsonData, mqttTopic, err := e.extractJSON(pkt.PacketData)
	if err != nil {
		if e.config.StrictJSON {
			e.logger.Error("QingPing packet processing failed: JSON extraction error (strict mode)",
				"timestamp", pkt.Timestamp,
				"src_ip", pkt.SrcIP,
				"src_port", pkt.SrcPort,
				"dst_ip", pkt.DstIP,
				"dst_port", pkt.DstPort,
				"protocol", pkt.Protocol,
				"packet_len", pkt.Length,
				"payload_len", len(pkt.PacketData),
				"error", err,
				"payload_preview", string(pkt.PacketData[:min(100, len(pkt.PacketData))]),
				"outcome", "failed_extraction_strict")
			return fmt.Errorf("JSON extraction failed: %v", err)
		}
		e.logger.Warn("QingPing packet processing skipped: JSON extraction error (lenient mode)",
			"timestamp", pkt.Timestamp,
			"src_ip", pkt.SrcIP,
			"src_port", pkt.SrcPort,
			"dst_ip", pkt.DstIP,
			"dst_port", pkt.DstPort,
			"protocol", pkt.Protocol,
			"packet_len", pkt.Length,
			"payload_len", len(pkt.PacketData),
			"error", err,
			"payload_preview", string(pkt.PacketData[:min(100, len(pkt.PacketData))]),
			"outcome", "failed_extraction_lenient")
		return nil
	}

	logFields := []interface{}{
		"src_ip", pkt.SrcIP,
		"dst_ip", pkt.DstIP,
		"json_size", len(jsonData),
		"total_payload_size", len(pkt.PacketData),
	}
	if mqttTopic != "" {
		logFields = append(logFields, "mqtt_topic", mqttTopic)
	}
	e.logger.Debug("QingPing extracted JSON from MQTT payload", logFields...)

	// Validate JSON
	valid, err := e.validateJSON(jsonData)
	if !valid {
		if e.config.StrictJSON {
			e.logger.Error("QingPing packet processing failed: JSON validation error (strict mode)",
				"timestamp", pkt.Timestamp,
				"src_ip", pkt.SrcIP,
				"src_port", pkt.SrcPort,
				"dst_ip", pkt.DstIP,
				"dst_port", pkt.DstPort,
				"protocol", pkt.Protocol,
				"packet_len", pkt.Length,
				"error", err,
				"json_preview", string(jsonData[:min(100, len(jsonData))]),
				"outcome", "failed_validation_strict")
			return fmt.Errorf("strict JSON validation failed: %v", err)
		}
		// Continue processing even with invalid JSON if not strict
		e.logger.Warn("QingPing JSON validation failed but continuing (lenient mode)",
			"timestamp", pkt.Timestamp,
			"src_ip", pkt.SrcIP,
			"src_port", pkt.SrcPort,
			"dst_ip", pkt.DstIP,
			"dst_port", pkt.DstPort,
			"protocol", pkt.Protocol,
			"packet_len", pkt.Length,
			"error", err,
			"json_preview", string(jsonData[:min(100, len(jsonData))]),
			"outcome", "validation_failed_continuing")
	} else {
		e.logger.Debug("QingPing JSON validation passed",
			"src_ip", pkt.SrcIP,
			"dst_ip", pkt.DstIP,
			"json_size", len(jsonData))
	}

	// Submit to upstream server (with extracted JSON)
	if err := e.submitToUpstream(pkt, jsonData, mqttTopic); err != nil {
		if e.config.IgnoreHTTPErrors {
			e.logger.Warn("QingPing packet processed but upstream submit failed (ignored)",
				"timestamp", pkt.Timestamp,
				"src_ip", pkt.SrcIP,
				"src_port", pkt.SrcPort,
				"dst_ip", pkt.DstIP,
				"dst_port", pkt.DstPort,
				"protocol", pkt.Protocol,
				"packet_len", pkt.Length,
				"payload_len", len(pkt.PacketData),
				"upstream_url", e.config.UpstreamURL,
				"json_size", len(jsonData),
				"error", err,
				"outcome", "upstream_failed_ignored")
			return nil
		}
		e.logger.Error("QingPing packet processing failed: upstream submit error",
			"timestamp", pkt.Timestamp,
			"src_ip", pkt.SrcIP,
			"src_port", pkt.SrcPort,
			"dst_ip", pkt.DstIP,
			"dst_port", pkt.DstPort,
			"protocol", pkt.Protocol,
			"packet_len", pkt.Length,
			"payload_len", len(pkt.PacketData),
			"upstream_url", e.config.UpstreamURL,
			"json_size", len(jsonData),
			"error", err,
			"outcome", "failed_upstream")
		return fmt.Errorf("failed to submit to upstream: %v", err)
	}

	e.logger.Info("QingPing packet processed successfully",
		"timestamp", pkt.Timestamp,
		"src_ip", pkt.SrcIP,
		"src_port", pkt.SrcPort,
		"dst_ip", pkt.DstIP,
		"dst_port", pkt.DstPort,
		"protocol", pkt.Protocol,
		"upstream_url", e.config.UpstreamURL,
		"json_size", len(jsonData),
		"outcome", "success")

	return nil
}

// submitToUpstream sends the extracted JSON payload to the upstream server via HTTP POST
func (e *Exporter) submitToUpstream(pkt *decoder.PacketInfo, jsonData []byte, mqttTopic string) error {
	// Prepare the request with extracted JSON data
	req, err := http.NewRequest("POST", e.config.UpstreamURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "tzsp-qingping-exporter/1.0")

	// Add custom headers with packet metadata
	if pkt.SrcIP != "" {
		req.Header.Set("X-Source-IP", pkt.SrcIP)
	}
	if pkt.DstIP != "" {
		req.Header.Set("X-Destination-IP", pkt.DstIP)
	}
	req.Header.Set("X-Destination-Port", fmt.Sprintf("%d", pkt.DstPort))
	req.Header.Set("X-Protocol", pkt.Protocol)
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", pkt.Timestamp))

	// Add MQTT topic if extracted
	if mqttTopic != "" {
		req.Header.Set("X-MQTT-Topic", mqttTopic)
	}

	// Send the request
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, _ := io.ReadAll(resp.Body)

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upstream returned non-OK status: %d, body: %s", resp.StatusCode, string(body))
	}

	e.logger.Debug("QingPing upstream response",
		"status_code", resp.StatusCode,
		"response_body", string(body))

	return nil
}

// Close cleans up the exporter resources
func (e *Exporter) Close() error {
	if e == nil {
		return nil
	}
	e.httpClient.CloseIdleConnections()
	e.logger.Info("QingPing exporter closed")
	return nil
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetStats returns statistics about the exporter (placeholder for future implementation)
func (e *Exporter) GetStats() map[string]interface{} {
	if e == nil {
		return nil
	}
	return map[string]interface{}{
		"enabled":      e.config.Enabled,
		"upstream_url": e.config.UpstreamURL,
		"strict_json":  e.config.StrictJSON,
	}
}
