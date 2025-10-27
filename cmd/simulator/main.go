// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lumberbarons/modbus/internal/simulator"
)

func main() {
	// Parse command line flags
	mode := flag.String("mode", "rtu", "Modbus mode: rtu, ascii, or tcp")
	slaveID := flag.Int("slave-id", 1, "Slave ID for serial modes (1-247)")
	baudRate := flag.Int("baud", 19200, "Baud rate for serial modes")
	tcpAddress := flag.String("addr", "localhost:5020", "TCP address for tcp mode (host:port)")
	configFile := flag.String("config", "", "JSON config file for initial data values")
	flag.Parse()

	if *slaveID < 1 || *slaveID > 247 {
		log.Fatalf("invalid slave ID %d: must be between 1 and 247", *slaveID)
	}

	// Load configuration
	var config *simulator.DataStoreConfig
	if *configFile != "" {
		var err error
		config, err = loadConfig(*configFile)
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
		log.Printf("loaded initial data from %s", *configFile)
	}

	// Create data store
	ds := simulator.NewDataStore(config)

	// Create and start server based on mode
	var server interface {
		Start() error
		Stop() error
	}
	var connectionInfo string

	switch *mode {
	case "rtu":
		rtuServer, err := simulator.NewRTUServer(ds, &simulator.RTUServerConfig{
			SlaveID:  byte(*slaveID),
			BaudRate: *baudRate,
		})
		if err != nil {
			log.Fatalf("failed to create RTU server: %v", err)
		}
		server = rtuServer
		connectionInfo = fmt.Sprintf("Client device path: %s", rtuServer.ClientDevicePath())

	case "ascii":
		asciiServer, err := simulator.NewASCIIServer(ds, &simulator.ASCIIServerConfig{
			SlaveID:  byte(*slaveID),
			BaudRate: *baudRate,
		})
		if err != nil {
			log.Fatalf("failed to create ASCII server: %v", err)
		}
		server = asciiServer
		connectionInfo = fmt.Sprintf("Client device path: %s", asciiServer.ClientDevicePath())

	case "tcp":
		tcpServer, err := simulator.NewTCPServer(ds, &simulator.TCPServerConfig{
			Address: *tcpAddress,
		})
		if err != nil {
			log.Fatalf("failed to create TCP server: %v", err)
		}
		server = tcpServer
		connectionInfo = fmt.Sprintf("TCP address: %s", tcpServer.Address())

	default:
		log.Fatalf("invalid mode %q: must be rtu, ascii, or tcp", *mode)
	}

	// Start the server
	if err := server.Start(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}

	// Print connection info
	fmt.Printf("Modbus %s simulator running\n", *mode)
	fmt.Printf("%s\n", connectionInfo)
	if *mode == "rtu" || *mode == "ascii" {
		fmt.Printf("Slave ID: %d\n", *slaveID)
		fmt.Printf("Baud rate: %d\n", *baudRate)
	}
	fmt.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
	if err := server.Stop(); err != nil {
		log.Printf("error stopping server: %v", err)
	}
}

// loadConfig loads a DataStoreConfig from a JSON file.
func loadConfig(filename string) (*simulator.DataStoreConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var config simulator.DataStoreConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &config, nil
}
