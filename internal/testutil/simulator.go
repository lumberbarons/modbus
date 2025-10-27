// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package testutil

import (
	"testing"

	"github.com/lumberbarons/modbus/internal/simulator"
)

// RTUSimulator wraps an RTU server for testing.
type RTUSimulator struct {
	server *simulator.RTUServer
	t      *testing.T
}

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
