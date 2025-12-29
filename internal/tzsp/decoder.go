package tzsp

import (
	"encoding/binary"
	"fmt"
	"time"
)

// TZSP protocol constants
const (
	// TZSP version
	Version = 1

	// TZSP packet types
	TypeReceivedTaggedPacket = 0 // Packet received by sniffer with tags
	TypePacketForTransmit    = 1 // Packet to be transmitted
	TypeReserved             = 2 // Reserved
	TypeConfiguration        = 3 // Configuration
	TypeKeepalive            = 4 // Keepalive
	TypePortOpener           = 5 // Port opener

	// Tag types
	TagPad       = 0  // Padding (no data)
	TagEnd       = 1  // End of tag list
	TagRawRSSI   = 10 // Raw RSSI
	TagSNR       = 11 // Signal to Noise Ratio
	TagDataRate  = 12 // Data rate
	TagTimestamp = 13 // Timestamp
	TagPacketLen = 40 // Packet length
	TagSensor    = 60 // Sensor MAC address
)

// Packet represents a decoded TZSP packet
type Packet struct {
	Version      uint8
	Type         uint8
	Protocol     uint16
	Tags         []Tag
	EncapPacket  []byte
	ReceivedTime time.Time
	SourceAddr   string
}

// Tag represents a TZSP tag
type Tag struct {
	Type   uint8
	Length uint8
	Data   []byte
}

// Decoder decodes TZSP packets
type Decoder struct{}

// NewDecoder creates a new TZSP decoder
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Decode decodes a TZSP packet from raw bytes
func (d *Decoder) Decode(data []byte, sourceAddr string) (*Packet, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("packet too short: %d bytes", len(data))
	}

	pkt := &Packet{
		Version:      data[0],
		Type:         data[1],
		Protocol:     binary.BigEndian.Uint16(data[2:4]),
		ReceivedTime: time.Now(),
		SourceAddr:   sourceAddr,
	}

	// Verify version
	if pkt.Version != Version {
		return nil, fmt.Errorf("unsupported TZSP version: %d", pkt.Version)
	}

	// Parse tags
	offset := 4
	for offset < len(data) {
		tagType := data[offset]
		offset++

		// End of tag list
		if tagType == TagEnd {
			break
		}

		// Padding tag (no data)
		if tagType == TagPad {
			continue
		}

		// Other tags have length byte
		if offset >= len(data) {
			return nil, fmt.Errorf("incomplete tag at offset %d", offset-1)
		}

		tagLen := data[offset]
		offset++

		// Read tag data
		if offset+int(tagLen) > len(data) {
			return nil, fmt.Errorf("tag data exceeds packet length")
		}

		tagData := make([]byte, tagLen)
		copy(tagData, data[offset:offset+int(tagLen)])
		offset += int(tagLen)

		pkt.Tags = append(pkt.Tags, Tag{
			Type:   tagType,
			Length: tagLen,
			Data:   tagData,
		})
	}

	// Remaining bytes are the encapsulated packet
	if offset < len(data) {
		pkt.EncapPacket = make([]byte, len(data)-offset)
		copy(pkt.EncapPacket, data[offset:])
	}

	return pkt, nil
}

// GetTag returns the first tag of the specified type, or nil if not found
func (p *Packet) GetTag(tagType uint8) *Tag {
	for i := range p.Tags {
		if p.Tags[i].Type == tagType {
			return &p.Tags[i]
		}
	}
	return nil
}

// GetTimestamp extracts timestamp from tags if available
func (p *Packet) GetTimestamp() *time.Time {
	tag := p.GetTag(TagTimestamp)
	if tag == nil || len(tag.Data) < 4 {
		return nil
	}

	// Timestamp is in seconds since epoch
	ts := binary.BigEndian.Uint32(tag.Data)
	t := time.Unix(int64(ts), 0)
	return &t
}

// GetRSSI extracts RSSI value from tags if available
func (p *Packet) GetRSSI() *int8 {
	tag := p.GetTag(TagRawRSSI)
	if tag == nil || len(tag.Data) < 1 {
		return nil
	}

	rssi := int8(tag.Data[0])
	return &rssi
}

// ProtocolName returns a human-readable protocol name
func (p *Packet) ProtocolName() string {
	switch p.Protocol {
	case 1:
		return "Ethernet"
	case 2:
		return "802.11"
	case 3:
		return "Prism"
	case 18:
		return "IEEE 802.11 + RadioTap"
	default:
		return fmt.Sprintf("Unknown(%d)", p.Protocol)
	}
}
