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

// LoggingConfig contains application logging settings
type LoggingConfig struct {
	Level         string `yaml:"level"`
	Format        string `yaml:"format"`
	ConsoleOutput bool   `yaml:"console_output"`
	ConsoleLevel  string `yaml:"console_level"`
	ConsoleFormat string `yaml:"console_format"`
}

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if cfg.Server.BufferSize == 0 {
		cfg.Server.BufferSize = 65536
	}
	if cfg.Output.NetFlow.FlowTimeout == 0 {
		cfg.Output.NetFlow.FlowTimeout = 60
	}
	if cfg.Output.NetFlow.ActiveTimeout == 0 {
		cfg.Output.NetFlow.ActiveTimeout = 120
	}

	return &cfg, nil
}
