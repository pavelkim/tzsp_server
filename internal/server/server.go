package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/pavelkim/tzsp_server/internal/decoder"
	"github.com/pavelkim/tzsp_server/internal/logger"
	"github.com/pavelkim/tzsp_server/internal/netflow"
	"github.com/pavelkim/tzsp_server/internal/output"
	"github.com/pavelkim/tzsp_server/internal/pcap"
	"github.com/pavelkim/tzsp_server/internal/qingping"
	"github.com/pavelkim/tzsp_server/internal/tzsp"
)

// Server represents the TZSP server
type Server struct {
	listenAddr    string
	bufferSize    int
	conn          *net.UDPConn
	tzspDecoder   *tzsp.Decoder
	packetDecoder *decoder.Decoder
	fileWriter    *output.FileWriter
	pcapWriter    *pcap.Writer
	netflowExp    *netflow.Exporter
	qingpingExp   *qingping.Exporter
	logger        *logger.Logger

	packetsReceived uint64
	packetsDecoded  uint64
	packetsWritten  uint64
}

// Config contains server configuration
type Config struct {
	ListenAddr  string
	BufferSize  int
	FileWriter  *output.FileWriter
	PcapWriter  *pcap.Writer
	NetFlowExp  *netflow.Exporter
	QingPingExp *qingping.Exporter
	Logger      *logger.Logger
}

// NewServer creates a new TZSP server
func NewServer(cfg *Config) *Server {
	return &Server{
		listenAddr:    cfg.ListenAddr,
		bufferSize:    cfg.BufferSize,
		tzspDecoder:   tzsp.NewDecoder(),
		packetDecoder: decoder.NewDecoder(),
		fileWriter:    cfg.FileWriter,
		pcapWriter:    cfg.PcapWriter,
		netflowExp:    cfg.NetFlowExp,
		qingpingExp:   cfg.QingPingExp,
		logger:        cfg.Logger,
	}
}

// Start starts the TZSP server
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("========================================")
	s.logger.Info("Starting TZSP server...")

	// Resolve UDP address
	addr, err := net.ResolveUDPAddr("udp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve address: %w", err)
	}
	s.logger.Info("[OK] UDP address resolved", "address", addr.String())

	// Listen on UDP
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP: %w", err)
	}
	s.conn = conn
	s.logger.Info("[OK] UDP socket opened", "port", addr.Port)

	s.logger.Info("========================================")
	s.logger.Info("*** TZSP server is now listening for packets ***")
	s.logger.Info("========================================")
	s.logger.Info("Configuration:")
	s.logger.Info("  - Listen address:", "addr", s.listenAddr)
	s.logger.Info("  - Buffer size:", "bytes", s.bufferSize)
	if s.fileWriter != nil {
		s.logger.Info("  - File output (packet metadata): ENABLED")
	} else {
		s.logger.Info("  - File output (packet metadata): disabled")
	}
	if s.pcapWriter != nil {
		s.logger.Info("  - PCAP output: ENABLED")
	} else {
		s.logger.Info("  - PCAP output: disabled")
	}
	if s.netflowExp != nil {
		s.logger.Info("  - NetFlow export: ENABLED")
	} else {
		s.logger.Info("  - NetFlow export: disabled")
	}
	if s.qingpingExp != nil {
		s.logger.Info("  - QingPing export: ENABLED")
	} else {
		s.logger.Info("  - QingPing export: disabled")
	}
	s.logger.Info("========================================")
	s.logger.Info("Waiting for TZSP packets... (Press Ctrl+C to stop)")
	s.logger.Info("========================================")

	// Start statistics reporter
	go s.reportStats(ctx)

	// Main receive loop
	buf := make([]byte, s.bufferSize)
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Context cancelled, stopping receiver loop...")
			return nil
		default:
			// Set read deadline to allow checking context
			s.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

			n, remoteAddr, err := s.conn.ReadFromUDP(buf)
			if err != nil {
				// Check if it's a timeout (expected)
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				s.logger.Error("Failed to read UDP packet", "error", err)
				continue
			}

			s.packetsReceived++

			// Log first packet received
			if s.packetsReceived == 1 {
				s.logger.Info(">>> First TZSP packet received!",
					"source", remoteAddr.String(),
					"size", n)
			}

			// Process packet
			if err := s.processPacket(buf[:n], remoteAddr.String()); err != nil {
				s.logger.Debug("Failed to process packet", "error", err, "source", remoteAddr.String())
			}
		}
	}
}

// Stop stops the TZSP server
func (s *Server) Stop() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// processPacket processes a received TZSP packet
func (s *Server) processPacket(data []byte, sourceAddr string) error {
	// Decode TZSP packet
	tzspPkt, err := s.tzspDecoder.Decode(data, sourceAddr)
	if err != nil {
		return fmt.Errorf("TZSP decode error: %w", err)
	}

	// Skip if no encapsulated packet
	if len(tzspPkt.EncapPacket) == 0 {
		return nil
	}

	s.packetsDecoded++

	// Use TZSP timestamp if available, otherwise use receive time
	timestamp := tzspPkt.ReceivedTime
	if ts := tzspPkt.GetTimestamp(); ts != nil {
		timestamp = *ts
	}

	// Always log incoming packet at debug level
	s.logger.Debug("TZSP packet received",
		"source", tzspPkt.SourceAddr,
		"size", len(data),
		"protocol", tzspPkt.ProtocolName(),
		"timestamp", timestamp.Format(time.RFC3339Nano),
	)

	// Write to PCAP if enabled
	if s.pcapWriter != nil {
		if err := s.pcapWriter.WritePacket(tzspPkt.EncapPacket, timestamp); err != nil {
			s.logger.Error("Failed to write PCAP", "error", err)
		} else {
			s.packetsWritten++
		}
	}

	// Decode encapsulated packet
	packetInfo, err := s.packetDecoder.Decode(tzspPkt.EncapPacket, timestamp.UnixNano())
	if err != nil {
		// Log decode errors at debug level (they're common for non-IP packets)
		s.logger.Debug("Packet decode error", "error", err)
		return nil
	}

	// Export to NetFlow if enabled
	if s.netflowExp != nil {
		if err := s.netflowExp.ProcessPacket(packetInfo); err != nil {
			s.logger.Error("Failed to export NetFlow", "error", err)
		}
	}

	// Export to QingPing if enabled
	if s.qingpingExp != nil {
		if err := s.qingpingExp.Export(packetInfo); err != nil {
			s.logger.Error("Failed to export QingPing", "error", err)
		}
	}

	// Write packet metadata to file if enabled
	if s.fileWriter != nil {
		s.fileWriter.WritePacket(packetInfo)
	}

	return nil
}

// reportStats periodically reports server statistics
func (s *Server) reportStats(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.logger.Info("=== Statistics Report ===",
				"packets_received", s.packetsReceived,
				"packets_decoded", s.packetsDecoded,
				"packets_written", s.packetsWritten,
				"decode_rate", fmt.Sprintf("%.1f%%", float64(s.packetsDecoded)/float64(s.packetsReceived)*100),
			)
		}
	}
}
