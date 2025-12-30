package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pavelkim/tzsp_server/internal/config"
	"github.com/pavelkim/tzsp_server/internal/logger"
	"github.com/pavelkim/tzsp_server/internal/netflow"
	"github.com/pavelkim/tzsp_server/internal/output"
	"github.com/pavelkim/tzsp_server/internal/pcap"
	"github.com/pavelkim/tzsp_server/internal/qingping"
	"github.com/pavelkim/tzsp_server/internal/server"
	"github.com/pavelkim/tzsp_server/internal/version"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("tzsp_server version %s\n", version.GetVersion())
		os.Exit(0)
	}

	// Load configuration (uses defaults if file doesn't exist)
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger

	       logCfg := &logger.Config{
		       File: logger.FileConfig{
			       Enabled: cfg.Logging.File.Enabled,
			       Level:   cfg.Logging.File.Level,
			       Format:  cfg.Logging.File.Format,
			       Path:    cfg.Logging.File.Path,
		       },
		       Console: logger.ConsoleConfig{
			       Enabled: cfg.Logging.Console.Enabled,
			       Level:   cfg.Logging.Console.Level,
			       Format:  cfg.Logging.Console.Format,
		       },
	       }
	       log, err := logger.NewLogger(logCfg)
	       if err != nil {
		       fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		       os.Exit(1)
	       }

	       log.Info("========================================")
	       log.Info("Starting TZSP Server", "version", version.GetVersion())
	       log.Info("========================================")
	       log.Info("Configuration loaded", "file", *configPath)
	       log.Info("Server settings",
		       "listen_addr", cfg.Server.ListenAddr,
		       "buffer_size", cfg.Server.BufferSize)

	       // Print enabled logging destinations
	       if logCfg.Console.Enabled {
		       log.Info("Logging destination: CONSOLE",
			       "level", logCfg.Console.Level,
			       "format", logCfg.Console.Format)
	       }
	       if logCfg.File.Enabled && logCfg.File.Path != "" {
		       log.Info("Logging destination: FILE",
			       "level", logCfg.File.Level,
			       "format", logCfg.File.Format,
			       "path", logCfg.File.Path)
	       }

	// Initialize file output for packet metadata if enabled
	var fileWriter *output.FileWriter
	if cfg.Output.File.Enabled {
		log.Info("Initializing file output for packet metadata...")
		fileWriter, err = output.NewFileWriter(
			cfg.Output.File.Enabled,
			cfg.Output.File.OutputFile,
			cfg.Output.File.Format,
		)
		if err != nil {
			log.Error("Failed to initialize file output", "error", err)
			os.Exit(1)
		}
		defer fileWriter.Close()
		log.Info("[OK] File output initialized",
			"file", cfg.Output.File.OutputFile,
			"format", cfg.Output.File.Format)
	} else {
		log.Info("File output for packet metadata disabled")
	}

	// Initialize PCAP writer if enabled
	var pcapWriter *pcap.Writer
	if cfg.Output.PCAP.Enabled {
		log.Info("Initializing PCAP writer...")
		pcapWriter, err = pcap.NewWriter(
			cfg.Output.PCAP.OutputFile,
			cfg.Output.PCAP.MaxSizeMB,
			cfg.Output.PCAP.MaxBackups,
		)
		if err != nil {
			log.Error("Failed to initialize PCAP writer", "error", err)
			os.Exit(1)
		}
		defer pcapWriter.Close()
		log.Info("[OK] PCAP writer initialized",
			"file", cfg.Output.PCAP.OutputFile,
			"max_size_mb", cfg.Output.PCAP.MaxSizeMB,
			"max_backups", cfg.Output.PCAP.MaxBackups)
	} else {
		log.Info("PCAP writer disabled")
	}

	// Initialize NetFlow exporter if enabled
	var netflowExp *netflow.Exporter
	if cfg.Output.NetFlow.Enabled {
		log.Info("Initializing NetFlow exporter...")
		netflowExp, err = netflow.NewExporter(
			cfg.Output.NetFlow.CollectorAddr,
			cfg.Output.NetFlow.Version,
			cfg.Output.NetFlow.FlowTimeout,
			cfg.Output.NetFlow.ActiveTimeout,
		)
		if err != nil {
			log.Error("Failed to initialize NetFlow exporter", "error", err)
			os.Exit(1)
		}
		defer netflowExp.Close()
		log.Info("[OK] NetFlow exporter initialized",
			"collector", cfg.Output.NetFlow.CollectorAddr,
			"version", cfg.Output.NetFlow.Version,
			"flow_timeout", cfg.Output.NetFlow.FlowTimeout,
			"active_timeout", cfg.Output.NetFlow.ActiveTimeout)
	} else {
		log.Info("NetFlow exporter disabled")
	}

	// Initialize QingPing exporter if enabled
	var qingpingExp *qingping.Exporter
	if cfg.Output.QingPing.Enabled {
		log.Info("Initializing QingPing exporter...")
		qingpingExp, err = qingping.NewExporter(qingping.Config{
			Enabled: cfg.Output.QingPing.Enabled,
			Filter: qingping.Filter{
				SrcIP:    cfg.Output.QingPing.Filter.SrcIP,
				DstIP:    cfg.Output.QingPing.Filter.DstIP,
				DstPort:  cfg.Output.QingPing.Filter.DstPort,
				Protocol: cfg.Output.QingPing.Filter.Protocol,
			},
			StrictJSON:       cfg.Output.QingPing.StrictJSON,
			UpstreamURL:      cfg.Output.QingPing.UpstreamURL,
			IgnoreSSL:        cfg.Output.QingPing.IgnoreSSL,
			IgnoreHTTPErrors: cfg.Output.QingPing.IgnoreHTTPErrors,
			Logger:           log,
		})
		if err != nil {
			log.Error("Failed to initialize QingPing exporter", "error", err)
			os.Exit(1)
		}
		defer qingpingExp.Close()
		log.Info("[OK] QingPing exporter initialized")
	} else {
		log.Info("QingPing exporter disabled")
	}

	// Create server
	log.Info("Creating TZSP server...")
	srv := server.NewServer(&server.Config{
		ListenAddr:  cfg.Server.ListenAddr,
		BufferSize:  cfg.Server.BufferSize,
		FileWriter:  fileWriter,
		PcapWriter:  pcapWriter,
		NetFlowExp:  netflowExp,
		QingPingExp: qingpingExp,
		Logger:      log,
	})
	log.Info("[OK] Server created successfully")

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := srv.Start(ctx); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		log.Info("========================================")
		log.Info("Received shutdown signal (Ctrl+C)")
		log.Info("Shutting down gracefully...")
		cancel()
		srv.Stop()
		log.Info("[OK] Server stopped")
	case err := <-errChan:
		log.Error("========================================")
		log.Error("Server encountered an error", "error", err)
		log.Error("Shutting down...")
		cancel()
		srv.Stop()
		log.Error("[ERROR] Server stopped with errors")
		os.Exit(1)
	}

	log.Info("========================================")
	log.Info("TZSP Server terminated")
	log.Info("========================================")
}
