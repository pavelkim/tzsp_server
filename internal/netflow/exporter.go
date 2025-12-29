package netflow

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pavelkim/tzsp_server/internal/decoder"
)

// Exporter handles NetFlow export
type Exporter struct {
	collectorAddr string
	version       int
	flowTimeout   time.Duration
	activeTimeout time.Duration
	conn          *net.UDPConn
	flows         map[string]*Flow
	mu            sync.Mutex
	sequenceNum   uint32
}

// Flow represents a NetFlow flow record
type Flow struct {
	SrcIP     net.IP
	DstIP     net.IP
	SrcPort   uint16
	DstPort   uint16
	Protocol  uint8
	FirstSeen time.Time
	LastSeen  time.Time
	Packets   uint32
	Bytes     uint32
	TCPFlags  uint8
}

// NewExporter creates a new NetFlow exporter
func NewExporter(collectorAddr string, version int, flowTimeout, activeTimeout int) (*Exporter, error) {
	addr, err := net.ResolveUDPAddr("udp", collectorAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve collector address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to collector: %w", err)
	}

	e := &Exporter{
		collectorAddr: collectorAddr,
		version:       version,
		flowTimeout:   time.Duration(flowTimeout) * time.Second,
		activeTimeout: time.Duration(activeTimeout) * time.Second,
		conn:          conn,
		flows:         make(map[string]*Flow),
	}

	// Start flow expiration goroutine
	go e.expireFlows()

	return e, nil
}

// ProcessPacket processes a packet and updates flow records
func (e *Exporter) ProcessPacket(info *decoder.PacketInfo) error {
	if info.SrcIP == "" || info.DstIP == "" {
		return nil // Skip non-IP packets
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Create flow key
	flowKey := e.makeFlowKey(info)

	// Get or create flow
	flow, exists := e.flows[flowKey]
	if !exists {
		flow = &Flow{
			SrcIP:     net.ParseIP(info.SrcIP),
			DstIP:     net.ParseIP(info.DstIP),
			SrcPort:   info.SrcPort,
			DstPort:   info.DstPort,
			Protocol:  e.getProtocolNumber(info.Protocol),
			FirstSeen: time.Unix(0, info.Timestamp),
			LastSeen:  time.Unix(0, info.Timestamp),
			Packets:   0,
			Bytes:     0,
		}
		e.flows[flowKey] = flow
	}

	// Update flow
	flow.LastSeen = time.Unix(0, info.Timestamp)
	flow.Packets++
	flow.Bytes += uint32(info.Length)
	flow.TCPFlags |= e.parseTCPFlags(info.TCPFlags)

	// Check for active timeout
	if time.Since(flow.FirstSeen) >= e.activeTimeout {
		e.exportFlow(flow)
		delete(e.flows, flowKey)
	}

	return nil
}

// Close closes the NetFlow exporter
func (e *Exporter) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Export remaining flows
	for key, flow := range e.flows {
		e.exportFlow(flow)
		delete(e.flows, key)
	}

	if e.conn != nil {
		return e.conn.Close()
	}
	return nil
}

// makeFlowKey creates a unique key for the flow
func (e *Exporter) makeFlowKey(info *decoder.PacketInfo) string {
	return fmt.Sprintf("%s:%d-%s:%d-%s",
		info.SrcIP, info.SrcPort,
		info.DstIP, info.DstPort,
		info.Protocol)
}

// getProtocolNumber converts protocol name to number
func (e *Exporter) getProtocolNumber(protocol string) uint8 {
	switch protocol {
	case "TCP":
		return 6
	case "UDP":
		return 17
	case "ICMPv4":
		return 1
	case "ICMPv6":
		return 58
	default:
		return 0
	}
}

// parseTCPFlags parses TCP flags string to byte
func (e *Exporter) parseTCPFlags(flags string) uint8 {
	var result uint8
	for _, c := range flags {
		switch c {
		case 'F':
			result |= 0x01 // FIN
		case 'S':
			result |= 0x02 // SYN
		case 'R':
			result |= 0x04 // RST
		case 'P':
			result |= 0x08 // PSH
		case 'A':
			result |= 0x10 // ACK
		case 'U':
			result |= 0x20 // URG
		}
	}
	return result
}

// exportFlow exports a flow record using NetFlow v5
func (e *Exporter) exportFlow(flow *Flow) error {
	if e.version != 5 {
		// Only NetFlow v5 is implemented for simplicity
		return nil
	}

	// NetFlow v5 header (24 bytes) + 1 record (48 bytes) = 72 bytes
	buf := make([]byte, 72)

	// Header
	binary.BigEndian.PutUint16(buf[0:2], 5)                                 // Version
	binary.BigEndian.PutUint16(buf[2:4], 1)                                 // Count (1 record)
	binary.BigEndian.PutUint32(buf[4:8], uint32(time.Now().Unix()*1000))    // SysUptime
	binary.BigEndian.PutUint32(buf[8:12], uint32(time.Now().Unix()))        // Unix secs
	binary.BigEndian.PutUint32(buf[12:16], uint32(time.Now().Nanosecond())) // Unix nsecs
	e.sequenceNum++
	binary.BigEndian.PutUint32(buf[16:20], e.sequenceNum) // Flow sequence
	// Engine type, ID, and sampling interval = 0

	// Flow record (starts at offset 24)
	offset := 24
	copy(buf[offset:offset+4], flow.SrcIP.To4())
	copy(buf[offset+4:offset+8], flow.DstIP.To4())
	// Next hop = 0.0.0.0
	binary.BigEndian.PutUint16(buf[offset+12:offset+14], 0) // Input interface
	binary.BigEndian.PutUint16(buf[offset+14:offset+16], 0) // Output interface
	binary.BigEndian.PutUint32(buf[offset+16:offset+20], flow.Packets)
	binary.BigEndian.PutUint32(buf[offset+20:offset+24], flow.Bytes)
	binary.BigEndian.PutUint32(buf[offset+24:offset+28], uint32(flow.FirstSeen.Unix()))
	binary.BigEndian.PutUint32(buf[offset+28:offset+32], uint32(flow.LastSeen.Unix()))
	binary.BigEndian.PutUint16(buf[offset+32:offset+34], flow.SrcPort)
	binary.BigEndian.PutUint16(buf[offset+34:offset+36], flow.DstPort)
	buf[offset+36] = 0 // Pad
	buf[offset+37] = flow.TCPFlags
	buf[offset+38] = flow.Protocol
	buf[offset+39] = 0 // TOS
	// AS numbers and mask = 0

	// Send to collector
	_, err := e.conn.Write(buf)
	return err
}

// expireFlows periodically expires old flows
func (e *Exporter) expireFlows() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		e.mu.Lock()
		now := time.Now()
		for key, flow := range e.flows {
			if now.Sub(flow.LastSeen) >= e.flowTimeout {
				e.exportFlow(flow)
				delete(e.flows, key)
			}
		}
		e.mu.Unlock()
	}
}
