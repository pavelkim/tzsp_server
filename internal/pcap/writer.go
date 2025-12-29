package pcap

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

// Writer handles PCAP file output
type Writer struct {
	filename     string
	maxSizeMB    int
	maxBackups   int
	file         *os.File
	writer       *pcapgo.Writer
	mu           sync.Mutex
	bytesWritten int64
}

// NewWriter creates a new PCAP writer
func NewWriter(filename string, maxSizeMB, maxBackups int) (*Writer, error) {
	w := &Writer{
		filename:   filename,
		maxSizeMB:  maxSizeMB,
		maxBackups: maxBackups,
	}

	if err := w.rotate(); err != nil {
		return nil, err
	}

	return w, nil
}

// WritePacket writes a packet to the PCAP file
func (w *Writer) WritePacket(data []byte, timestamp time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if rotation is needed
	if w.maxSizeMB > 0 && w.bytesWritten > int64(w.maxSizeMB)*1024*1024 {
		if err := w.rotate(); err != nil {
			return fmt.Errorf("failed to rotate file: %w", err)
		}
	}

	// Create capture info
	ci := gopacket.CaptureInfo{
		Timestamp:     timestamp,
		CaptureLength: len(data),
		Length:        len(data),
	}

	// Write packet
	if err := w.writer.WritePacket(ci, data); err != nil {
		return fmt.Errorf("failed to write packet: %w", err)
	}

	w.bytesWritten += int64(len(data))
	return nil
}

// Close closes the PCAP file
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// rotate rotates the PCAP file
func (w *Writer) rotate() error {
	// Close existing file
	if w.file != nil {
		w.file.Close()
	}

	// Rotate old files
	if w.maxBackups > 0 {
		for i := w.maxBackups - 1; i >= 0; i-- {
			oldName := w.getBackupName(i)
			newName := w.getBackupName(i + 1)

			if _, err := os.Stat(oldName); err == nil {
				if i == w.maxBackups-1 {
					os.Remove(oldName) // Remove oldest
				} else {
					os.Rename(oldName, newName)
				}
			}
		}

		// Move current to backup
		if _, err := os.Stat(w.filename); err == nil {
			os.Rename(w.filename, w.getBackupName(0))
		}
	}

	// Create new file
	f, err := os.Create(w.filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	// Create PCAP writer with Ethernet link type
	writer := pcapgo.NewWriter(f)
	if err := writer.WriteFileHeader(65536, layers.LinkTypeEthernet); err != nil {
		f.Close()
		return fmt.Errorf("failed to write PCAP header: %w", err)
	}

	w.file = f
	w.writer = writer
	w.bytesWritten = 0

	return nil
}

// getBackupName returns the backup filename for the given index
func (w *Writer) getBackupName(index int) string {
	if index == 0 {
		return w.filename + ".1"
	}
	return fmt.Sprintf("%s.%d", w.filename, index+1)
}
