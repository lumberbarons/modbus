// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package simulator

import (
	"fmt"
	"sync"
)

const (
	// Maximum address space for each data type
	maxAddress = 65536
)

// DataStore represents the in-memory storage for Modbus data.
// It maintains four separate address spaces:
// - Coils: read/write single bits (function codes 1, 5, 15)
// - Discrete Inputs: read-only single bits (function code 2)
// - Holding Registers: read/write 16-bit registers (function codes 3, 6, 16, 22, 23)
// - Input Registers: read-only 16-bit registers (function code 4)
type DataStore struct {
	mu sync.RWMutex

	coils          []bool
	discreteInputs []bool
	holdingRegs    []uint16
	inputRegs      []uint16

	// Register names for logging/debugging
	coilNames          map[uint16]string
	discreteInputNames map[uint16]string
	holdingRegNames    map[uint16]string
	inputRegNames      map[uint16]string
}

// RegisterConfig represents a named register with an initial value.
type RegisterConfig struct {
	Name  string `json:"name"`
	Value uint16 `json:"value"`
}

// CoilConfig represents a named coil with an initial value.
type CoilConfig struct {
	Name  string `json:"name"`
	Value bool   `json:"value"`
}

// DataStoreConfig allows configuring initial values for the data store.
type DataStoreConfig struct {
	// Initial values for each data type. If nil, defaults to zeros.
	// Legacy format: map[address]value
	Coils          map[uint16]bool   `json:"Coils,omitempty"`
	DiscreteInputs map[uint16]bool   `json:"DiscreteInputs,omitempty"`
	HoldingRegs    map[uint16]uint16 `json:"HoldingRegs,omitempty"`
	InputRegs      map[uint16]uint16 `json:"InputRegs,omitempty"`

	// New format: map[address]config with name
	NamedCoils          map[uint16]CoilConfig     `json:"NamedCoils,omitempty"`
	NamedDiscreteInputs map[uint16]CoilConfig     `json:"NamedDiscreteInputs,omitempty"`
	NamedHoldingRegs    map[uint16]RegisterConfig `json:"NamedHoldingRegs,omitempty"`
	NamedInputRegs      map[uint16]RegisterConfig `json:"NamedInputRegs,omitempty"`
}

// NewDataStore creates a new DataStore with optional initial configuration.
func NewDataStore(config *DataStoreConfig) *DataStore {
	ds := &DataStore{
		coils:              make([]bool, maxAddress),
		discreteInputs:     make([]bool, maxAddress),
		holdingRegs:        make([]uint16, maxAddress),
		inputRegs:          make([]uint16, maxAddress),
		coilNames:          make(map[uint16]string),
		discreteInputNames: make(map[uint16]string),
		holdingRegNames:    make(map[uint16]string),
		inputRegNames:      make(map[uint16]string),
	}

	if config != nil {
		// Legacy format (backward compatibility)
		for addr, val := range config.Coils {
			ds.coils[addr] = val
		}
		for addr, val := range config.DiscreteInputs {
			ds.discreteInputs[addr] = val
		}
		for addr, val := range config.HoldingRegs {
			ds.holdingRegs[addr] = val
		}
		for addr, val := range config.InputRegs {
			ds.inputRegs[addr] = val
		}

		// New named format
		for addr, cfg := range config.NamedCoils {
			ds.coils[addr] = cfg.Value
			if cfg.Name != "" {
				ds.coilNames[addr] = cfg.Name
			}
		}
		for addr, cfg := range config.NamedDiscreteInputs {
			ds.discreteInputs[addr] = cfg.Value
			if cfg.Name != "" {
				ds.discreteInputNames[addr] = cfg.Name
			}
		}
		for addr, cfg := range config.NamedHoldingRegs {
			ds.holdingRegs[addr] = cfg.Value
			if cfg.Name != "" {
				ds.holdingRegNames[addr] = cfg.Name
			}
		}
		for addr, cfg := range config.NamedInputRegs {
			ds.inputRegs[addr] = cfg.Value
			if cfg.Name != "" {
				ds.inputRegNames[addr] = cfg.Name
			}
		}
	}

	return ds
}

// ReadCoils reads quantity coils starting at address.
func (ds *DataStore) ReadCoils(address, quantity uint16) ([]bool, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	if err := ds.validateRange(address, quantity); err != nil {
		return nil, err
	}

	result := make([]bool, quantity)
	for i := uint16(0); i < quantity; i++ {
		result[i] = ds.coils[address+i]
	}
	return result, nil
}

// ReadDiscreteInputs reads quantity discrete inputs starting at address.
func (ds *DataStore) ReadDiscreteInputs(address, quantity uint16) ([]bool, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	if err := ds.validateRange(address, quantity); err != nil {
		return nil, err
	}

	result := make([]bool, quantity)
	for i := uint16(0); i < quantity; i++ {
		result[i] = ds.discreteInputs[address+i]
	}
	return result, nil
}

// ReadHoldingRegisters reads quantity holding registers starting at address.
func (ds *DataStore) ReadHoldingRegisters(address, quantity uint16) ([]uint16, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	if err := ds.validateRange(address, quantity); err != nil {
		return nil, err
	}

	result := make([]uint16, quantity)
	for i := uint16(0); i < quantity; i++ {
		result[i] = ds.holdingRegs[address+i]
	}
	return result, nil
}

// ReadInputRegisters reads quantity input registers starting at address.
func (ds *DataStore) ReadInputRegisters(address, quantity uint16) ([]uint16, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	if err := ds.validateRange(address, quantity); err != nil {
		return nil, err
	}

	result := make([]uint16, quantity)
	for i := uint16(0); i < quantity; i++ {
		result[i] = ds.inputRegs[address+i]
	}
	return result, nil
}

// WriteSingleCoil writes a single coil at address.
func (ds *DataStore) WriteSingleCoil(address uint16, value bool) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.coils[address] = value
	return nil
}

// WriteMultipleCoils writes multiple coils starting at address.
func (ds *DataStore) WriteMultipleCoils(address uint16, values []bool) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	quantity := uint16(len(values))
	if err := ds.validateRange(address, quantity); err != nil {
		return err
	}

	for i := uint16(0); i < quantity; i++ {
		ds.coils[address+i] = values[i]
	}
	return nil
}

// WriteSingleRegister writes a single holding register at address.
func (ds *DataStore) WriteSingleRegister(address, value uint16) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.holdingRegs[address] = value
	return nil
}

// WriteMultipleRegisters writes multiple holding registers starting at address.
func (ds *DataStore) WriteMultipleRegisters(address uint16, values []uint16) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	quantity := uint16(len(values))
	if err := ds.validateRange(address, quantity); err != nil {
		return err
	}

	for i := uint16(0); i < quantity; i++ {
		ds.holdingRegs[address+i] = values[i]
	}
	return nil
}

// MaskWriteRegister performs an AND/OR mask write on a holding register.
func (ds *DataStore) MaskWriteRegister(address, andMask, orMask uint16) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	// result = (current AND andMask) OR (orMask AND (NOT andMask))
	current := ds.holdingRegs[address]
	result := (current & andMask) | (orMask & (^andMask))
	ds.holdingRegs[address] = result
	return nil
}

// validateRange checks if address + quantity is within bounds.
func (ds *DataStore) validateRange(address, quantity uint16) error {
	if quantity == 0 {
		return fmt.Errorf("quantity must be greater than 0")
	}
	if uint32(address)+uint32(quantity) > maxAddress {
		return fmt.Errorf("address range %d-%d exceeds maximum", address, uint32(address)+uint32(quantity)-1)
	}
	return nil
}

// GetCoilName returns the name of a coil at the given address, if configured.
func (ds *DataStore) GetCoilName(address uint16) string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.coilNames[address]
}

// GetDiscreteInputName returns the name of a discrete input at the given address, if configured.
func (ds *DataStore) GetDiscreteInputName(address uint16) string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.discreteInputNames[address]
}

// GetHoldingRegName returns the name of a holding register at the given address, if configured.
func (ds *DataStore) GetHoldingRegName(address uint16) string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.holdingRegNames[address]
}

// GetInputRegName returns the name of an input register at the given address, if configured.
func (ds *DataStore) GetInputRegName(address uint16) string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.inputRegNames[address]
}
