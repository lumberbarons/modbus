// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"

	"github.com/lumberbarons/modbus/internal/simulator"
)

func main() {
	app := &cli.App{
		Name:  "modbus-simulator",
		Usage: "Modbus protocol simulator for testing",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "mode",
				Aliases: []string{"m"},
				Usage:   "Modbus mode: tcp, rtu, or ascii",
				Value:   "tcp",
			},
			&cli.StringFlag{
				Name:    "addr",
				Aliases: []string{"a"},
				Usage:   "TCP address (tcp mode only, format: host:port)",
				Value:   "localhost:5020",
			},
			&cli.IntFlag{
				Name:    "slave-id",
				Aliases: []string{"s"},
				Usage:   "Slave ID for serial modes (1-247)",
				Value:   1,
			},
			&cli.IntFlag{
				Name:  "baud",
				Usage: "Baud rate for serial modes",
				Value: 19200,
			},
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "JSON config file for initial data values",
			},
		},
		Action: runSimulator,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func runSimulator(c *cli.Context) error {
	mode := c.String("mode")
	slaveID := c.Int("slave-id")
	baudRate := c.Int("baud")
	tcpAddress := c.String("addr")
	configFile := c.String("config")

	// Validate slave ID
	if slaveID < 1 || slaveID > 247 {
		return fmt.Errorf("invalid slave ID %d: must be between 1 and 247", slaveID)
	}

	// Load configuration
	var config *simulator.DataStoreConfig
	if configFile != "" {
		var err error
		config, err = loadConfig(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		log.Printf("loaded initial data from %s", configFile)
	}

	// Create data store
	ds := simulator.NewDataStore(config)

	// Create and start server based on mode
	var server interface {
		Start() error
		Stop() error
	}
	var connectionInfo string

	switch mode {
	case "rtu":
		rtuServer, err := simulator.NewRTUServer(ds, &simulator.RTUServerConfig{
			SlaveID:  byte(slaveID),
			BaudRate: baudRate,
		})
		if err != nil {
			return fmt.Errorf("failed to create RTU server: %w", err)
		}
		server = rtuServer
		connectionInfo = fmt.Sprintf("Client device path: %s", rtuServer.ClientDevicePath())

	case "ascii":
		asciiServer, err := simulator.NewASCIIServer(ds, &simulator.ASCIIServerConfig{
			SlaveID:  byte(slaveID),
			BaudRate: baudRate,
		})
		if err != nil {
			return fmt.Errorf("failed to create ASCII server: %w", err)
		}
		server = asciiServer
		connectionInfo = fmt.Sprintf("Client device path: %s", asciiServer.ClientDevicePath())

	case "tcp":
		tcpServer, err := simulator.NewTCPServer(ds, &simulator.TCPServerConfig{
			Address: tcpAddress,
		})
		if err != nil {
			return fmt.Errorf("failed to create TCP server: %w", err)
		}
		server = tcpServer
		connectionInfo = fmt.Sprintf("TCP address: %s", tcpServer.Address())

	default:
		return fmt.Errorf("invalid mode %q: must be tcp, rtu, or ascii", mode)
	}

	// Start the server
	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Print connection info
	fmt.Printf("Modbus %s simulator running\n", mode)
	fmt.Printf("%s\n", connectionInfo)
	if mode == "rtu" || mode == "ascii" {
		fmt.Printf("Slave ID: %d\n", slaveID)
		fmt.Printf("Baud rate: %d\n", baudRate)
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

	return nil
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
