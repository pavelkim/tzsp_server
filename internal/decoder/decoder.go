package decoder

import (
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// PacketInfo contains decoded packet information
type PacketInfo struct {
	Timestamp  int64
	Protocol   string
	SrcIP      string
	DstIP      string
	SrcPort    uint16
	DstPort    uint16
	SrcMAC     string
	DstMAC     string
	Length     int
	PayloadLen int
	TCPFlags   string
	PacketData []byte
}

// Decoder decodes encapsulated network packets
type Decoder struct{}

// NewDecoder creates a new packet decoder
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Decode decodes an encapsulated packet and extracts metadata
func (d *Decoder) Decode(data []byte, timestamp int64) (*PacketInfo, error) {
	info := &PacketInfo{
		Timestamp:  timestamp,
		Length:     len(data),
		PacketData: data,
	}

	// Try to decode as Ethernet first
	packet := gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.Default)

	// Extract Ethernet layer
	if ethLayer := packet.Layer(layers.LayerTypeEthernet); ethLayer != nil {
		eth, _ := ethLayer.(*layers.Ethernet)
		info.SrcMAC = eth.SrcMAC.String()
		info.DstMAC = eth.DstMAC.String()
	}

	// Extract IPv4 layer
	if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
		ip, _ := ipLayer.(*layers.IPv4)
		info.SrcIP = ip.SrcIP.String()
		info.DstIP = ip.DstIP.String()
		info.Protocol = ip.Protocol.String()
	}

	// Extract IPv6 layer
	if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
		ip, _ := ipLayer.(*layers.IPv6)
		info.SrcIP = ip.SrcIP.String()
		info.DstIP = ip.DstIP.String()
		info.Protocol = ip.NextHeader.String()
	}

	// Extract TCP layer
	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp, _ := tcpLayer.(*layers.TCP)
		info.SrcPort = uint16(tcp.SrcPort)
		info.DstPort = uint16(tcp.DstPort)
		info.Protocol = "TCP"
		info.TCPFlags = d.formatTCPFlags(tcp)

		if appLayer := packet.ApplicationLayer(); appLayer != nil {
			info.PayloadLen = len(appLayer.Payload())
		}
	}

	// Extract UDP layer
	if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp, _ := udpLayer.(*layers.UDP)
		info.SrcPort = uint16(udp.SrcPort)
		info.DstPort = uint16(udp.DstPort)
		info.Protocol = "UDP"

		if appLayer := packet.ApplicationLayer(); appLayer != nil {
			info.PayloadLen = len(appLayer.Payload())
		}
	}

	// Extract ICMP layer
	if icmpLayer := packet.Layer(layers.LayerTypeICMPv4); icmpLayer != nil {
		info.Protocol = "ICMPv4"
	}

	if icmpLayer := packet.Layer(layers.LayerTypeICMPv6); icmpLayer != nil {
		info.Protocol = "ICMPv6"
	}

	// Check for errors
	if err := packet.ErrorLayer(); err != nil {
		return info, fmt.Errorf("packet decoding error: %v", err.Error())
	}

	return info, nil
}

// formatTCPFlags formats TCP flags into a readable string
func (d *Decoder) formatTCPFlags(tcp *layers.TCP) string {
	flags := ""
	if tcp.SYN {
		flags += "S"
	}
	if tcp.ACK {
		flags += "A"
	}
	if tcp.FIN {
		flags += "F"
	}
	if tcp.RST {
		flags += "R"
	}
	if tcp.PSH {
		flags += "P"
	}
	if tcp.URG {
		flags += "U"
	}
	if tcp.ECE {
		flags += "E"
	}
	if tcp.CWR {
		flags += "C"
	}
	if tcp.NS {
		flags += "N"
	}
	if flags == "" {
		flags = "-"
	}
	return flags
}
