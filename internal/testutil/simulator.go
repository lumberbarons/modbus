// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package testutil

import (
	"testing"

	"github.com/lumberbarons/modbus/internal/simulator"
)

// RTUSimulatorOption configures an RTU simulator.
type RTUSimulatorOption func(*rtuSimulatorConfig)

type rtuSimulatorConfig struct {
	slaveID  byte
	baudRate int
	config   *simulator.DataStoreConfig
}

// WithSlaveID sets the slave ID for the simulator.
func WithSlaveID(id byte) RTUSimulatorOption {
	return func(c *rtuSimulatorConfig) {
		c.slaveID = id
	}
}

// WithBaudRate sets the baud rate for the simulator.
func WithBaudRate(rate int) RTUSimulatorOption {
	return func(c *rtuSimulatorConfig) {
		c.baudRate = rate
	}
}

// WithDataStoreConfig sets initial data values for the simulator.
func WithDataStoreConfig(config *simulator.DataStoreConfig) RTUSimulatorOption {
	return func(c *rtuSimulatorConfig) {
		c.config = config
	}
}

// StartRTUSimulator creates and starts an RTU Modbus simulator for testing.
// It returns a cleanup function that should be deferred, and the device path
// that clients should use to connect.
//
// Example usage:
//
//	cleanup, devicePath := testutil.StartRTUSimulator(t,
//	    testutil.WithSlaveID(17),
//	    testutil.WithBaudRate(19200))
//	defer cleanup()
//
//	client := modbus.NewRTUClientHandler(devicePath)
//	// ... use client ...
func StartRTUSimulator(t *testing.T, opts ...RTUSimulatorOption) (cleanup func(), devicePath string) {
	t.Helper()

	// Apply options
	config := &rtuSimulatorConfig{
		slaveID:  1,
		baudRate: 19200,
	}
	for _, opt := range opts {
		opt(config)
	}

	// Create data store
	ds := simulator.NewDataStore(config.config)

	// Create RTU server
	server, err := simulator.NewRTUServer(ds, &simulator.RTUServerConfig{
		SlaveID:  config.slaveID,
		BaudRate: config.baudRate,
	})
	if err != nil {
		t.Fatalf("failed to create RTU simulator: %v", err)
	}

	// Start the server
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start RTU simulator: %v", err)
	}

	devicePath = server.ClientDevicePath()
	t.Logf("RTU simulator started on %s (slave ID: %d)", devicePath, config.slaveID)

	cleanup = func() {
		if err := server.Stop(); err != nil {
			t.Errorf("failed to stop RTU simulator: %v", err)
		}
		t.Logf("RTU simulator stopped")
	}

	return cleanup, devicePath
}

// ASCIISimulatorOption configures an ASCII simulator.
type ASCIISimulatorOption func(*asciiSimulatorConfig)

type asciiSimulatorConfig struct {
	slaveID  byte
	baudRate int
	config   *simulator.DataStoreConfig
}

// WithASCIISlaveID sets the slave ID for the ASCII simulator.
func WithASCIISlaveID(id byte) ASCIISimulatorOption {
	return func(c *asciiSimulatorConfig) {
		c.slaveID = id
	}
}

// WithASCIIBaudRate sets the baud rate for the ASCII simulator.
func WithASCIIBaudRate(rate int) ASCIISimulatorOption {
	return func(c *asciiSimulatorConfig) {
		c.baudRate = rate
	}
}

// WithASCIIDataStoreConfig sets initial data values for the ASCII simulator.
func WithASCIIDataStoreConfig(config *simulator.DataStoreConfig) ASCIISimulatorOption {
	return func(c *asciiSimulatorConfig) {
		c.config = config
	}
}

// StartASCIISimulator creates and starts an ASCII Modbus simulator for testing.
// It returns a cleanup function that should be deferred, and the device path
// that clients should use to connect.
func StartASCIISimulator(t *testing.T, opts ...ASCIISimulatorOption) (cleanup func(), devicePath string) {
	t.Helper()

	// Apply options
	config := &asciiSimulatorConfig{
		slaveID:  1,
		baudRate: 19200,
	}
	for _, opt := range opts {
		opt(config)
	}

	// Create data store
	ds := simulator.NewDataStore(config.config)

	// Create ASCII server
	server, err := simulator.NewASCIIServer(ds, &simulator.ASCIIServerConfig{
		SlaveID:  config.slaveID,
		BaudRate: config.baudRate,
	})
	if err != nil {
		t.Fatalf("failed to create ASCII simulator: %v", err)
	}

	// Start the server
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start ASCII simulator: %v", err)
	}

	devicePath = server.ClientDevicePath()
	t.Logf("ASCII simulator started on %s (slave ID: %d)", devicePath, config.slaveID)

	cleanup = func() {
		if err := server.Stop(); err != nil {
			t.Errorf("failed to stop ASCII simulator: %v", err)
		}
		t.Logf("ASCII simulator stopped")
	}

	return cleanup, devicePath
}

// TCPSimulatorOption configures a TCP simulator.
type TCPSimulatorOption func(*tcpSimulatorConfig)

type tcpSimulatorConfig struct {
	address string
	config  *simulator.DataStoreConfig
}

// WithTCPAddress sets the TCP address for the simulator.
func WithTCPAddress(address string) TCPSimulatorOption {
	return func(c *tcpSimulatorConfig) {
		c.address = address
	}
}

// WithTCPDataStoreConfig sets initial data values for the TCP simulator.
func WithTCPDataStoreConfig(config *simulator.DataStoreConfig) TCPSimulatorOption {
	return func(c *tcpSimulatorConfig) {
		c.config = config
	}
}

// StartTCPSimulator creates and starts a TCP Modbus simulator for testing.
// It returns a cleanup function that should be deferred, and the address
// that clients should use to connect.
func StartTCPSimulator(t *testing.T, opts ...TCPSimulatorOption) (cleanup func(), address string) {
	t.Helper()

	// Apply options
	config := &tcpSimulatorConfig{
		address: "localhost:0", // Use port 0 to let OS assign a free port
	}
	for _, opt := range opts {
		opt(config)
	}

	// Create data store
	ds := simulator.NewDataStore(config.config)

	// Create TCP server
	server, err := simulator.NewTCPServer(ds, &simulator.TCPServerConfig{
		Address: config.address,
	})
	if err != nil {
		t.Fatalf("failed to create TCP simulator: %v", err)
	}

	// Start the server
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start TCP simulator: %v", err)
	}

	address = server.Address()
	t.Logf("TCP simulator started on %s", address)

	cleanup = func() {
		if err := server.Stop(); err != nil {
			t.Errorf("failed to stop TCP simulator: %v", err)
		}
		t.Logf("TCP simulator stopped")
	}

	return cleanup, address
}
