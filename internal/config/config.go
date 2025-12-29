package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Output  OutputConfig  `yaml:"output"`
	Logging LoggingConfig `yaml:"logging"`
}

// ServerConfig contains server-specific settings
type ServerConfig struct {
	ListenAddr string `yaml:"listen_addr"`
	BufferSize int    `yaml:"buffer_size"`
}

// OutputConfig contains all output mode settings
type OutputConfig struct {
	File     FileOutputConfig     `yaml:"file"`
	PCAP     PCAPOutputConfig     `yaml:"pcap"`
	NetFlow  NetFlowOutputConfig  `yaml:"netflow"`
	QingPing QingPingOutputConfig `yaml:"qingping"`
}

// FileOutputConfig contains file output settings for packet metadata
type FileOutputConfig struct {
	Enabled    bool   `yaml:"enabled"`
	OutputFile string `yaml:"output_file"`
	Format     string `yaml:"format"`
}

// PCAPOutputConfig contains PCAP output settings
type PCAPOutputConfig struct {
	Enabled    bool   `yaml:"enabled"`
	OutputFile string `yaml:"output_file"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
}

// NetFlowOutputConfig contains NetFlow export settings
type NetFlowOutputConfig struct {
	Enabled       bool   `yaml:"enabled"`
	CollectorAddr string `yaml:"collector_addr"`
	Version       int    `yaml:"version"`
	FlowTimeout   int    `yaml:"flow_timeout"`
	ActiveTimeout int    `yaml:"active_timeout"`
}

// QingPingFilterConfig contains packet filtering criteria
type QingPingFilterConfig struct {
	SrcIP    string `yaml:"src_ip"`
	DstIP    string `yaml:"dst_ip"`
	DstPort  uint16 `yaml:"dst_port"`
	Protocol string `yaml:"protocol"` // tcp, udp, icmp
}

// QingPingOutputConfig contains QingPing sensor data export settings
type QingPingOutputConfig struct {
	Enabled          bool                 `yaml:"enabled"`
	Filter           QingPingFilterConfig `yaml:"filter"`
	StrictJSON       bool                 `yaml:"strict_json"`
	UpstreamURL      string               `yaml:"upstream_url"`
	IgnoreSSL        bool                 `yaml:"ignore_ssl"`
	IgnoreHTTPErrors bool                 `yaml:"ignore_http_errors"`
}

// LoggingFileConfig contains file logging settings
type LoggingFileConfig struct {
	Enabled bool   `yaml:"enabled"`
	Level   string `yaml:"level"`
	Format  string `yaml:"format"`
	Path    string `yaml:"path"`
}

// LoggingConsoleConfig contains console logging settings
type LoggingConsoleConfig struct {
	Enabled bool   `yaml:"enabled"`
	Level   string `yaml:"level"`
	Format  string `yaml:"format"`
}

// LoggingConfig contains application logging settings
type LoggingConfig struct {
	File    LoggingFileConfig    `yaml:"file"`
	Console LoggingConsoleConfig `yaml:"console"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			ListenAddr: "0.0.0.0:37008",
			BufferSize: 65536,
		},
		Output: OutputConfig{},
		Logging: LoggingConfig{
			Console: LoggingConsoleConfig{
				Enabled: true,
				Level:   "info",
				Format:  "json",
			},
		},
	}
}

// Load reads and parses the configuration file
// If the file doesn't exist, returns default configuration
func Load(path string) (*Config, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Start with defaults
	cfg := DefaultConfig()

	// Override with values from file
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply additional defaults if not set
	if cfg.Server.BufferSize == 0 {
		cfg.Server.BufferSize = 65536
	}
	if cfg.Server.ListenAddr == "" {
		cfg.Server.ListenAddr = "0.0.0.0:37008"
	}
	if cfg.Output.NetFlow.FlowTimeout == 0 {
		cfg.Output.NetFlow.FlowTimeout = 60
	}
	if cfg.Output.NetFlow.ActiveTimeout == 0 {
		cfg.Output.NetFlow.ActiveTimeout = 120
	}

	return cfg, nil
}
