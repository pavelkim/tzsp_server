package output

import (
	"os"

	"github.com/pavelkim/tzsp_server/internal/decoder"
	"github.com/sirupsen/logrus"
)

// FileWriter handles file output for packet metadata
type FileWriter struct {
	logger  *logrus.Logger
	enabled bool
}

// NewFileWriter creates a new file output writer for packet metadata
func NewFileWriter(enabled bool, outputFile, format string) (*FileWriter, error) {
	if !enabled || outputFile == "" {
		return &FileWriter{enabled: false}, nil
	}

	log := logrus.New()

	// Set format
	if format == "json" {
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// Open file
	file, err := os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	log.SetOutput(file)
	log.SetLevel(logrus.InfoLevel)

	return &FileWriter{
		logger:  log,
		enabled: true,
	}, nil
}

// WritePacket writes packet metadata to file
func (w *FileWriter) WritePacket(info *decoder.PacketInfo) {
	if !w.enabled {
		return
	}

	fields := logrus.Fields{
		"protocol":    info.Protocol,
		"src_ip":      info.SrcIP,
		"dst_ip":      info.DstIP,
		"src_port":    info.SrcPort,
		"dst_port":    info.DstPort,
		"src_mac":     info.SrcMAC,
		"dst_mac":     info.DstMAC,
		"length":      info.Length,
		"payload_len": info.PayloadLen,
	}

	if info.TCPFlags != "" {
		fields["tcp_flags"] = info.TCPFlags
	}

	w.logger.WithFields(fields).Info("packet")
}

// Close closes the file writer
func (w *FileWriter) Close() error {
	// File will be closed when the logger is garbage collected
	return nil
}
