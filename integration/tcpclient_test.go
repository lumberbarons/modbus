// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license.  See the LICENSE file for details.

package integration

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/lumberbarons/modbus"
	"github.com/lumberbarons/modbus/internal/testutil"
)

func TestTCPClient(t *testing.T) {
	cleanup, address := testutil.StartTCPSimulator(t)
	defer cleanup()

	client := modbus.TCPClient(address)
	ClientTestAll(t, client)
}

func TestTCPClientAdvancedUsage(t *testing.T) {
	cleanup, address := testutil.StartTCPSimulator(t)
	defer cleanup()

	handler := modbus.NewTCPClientHandler(address)
	handler.Timeout = 5 * time.Second
	handler.SlaveID = 1
	handler.Logger = log.New(os.Stdout, "tcp: ", log.LstdFlags)
	handler.Connect()
	defer handler.Close()

	client := modbus.NewClient(handler)
	results, err := client.ReadDiscreteInputs(15, 2)
	if err != nil || results == nil {
		t.Fatal(err, results)
	}
	results, err = client.WriteMultipleRegisters(1, 2, []byte{0, 3, 0, 4})
	if err != nil || results == nil {
		t.Fatal(err, results)
	}
	results, err = client.WriteMultipleCoils(5, 10, []byte{4, 3})
	if err != nil || results == nil {
		t.Fatal(err, results)
	}
}
