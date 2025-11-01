// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package simulator

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"time"
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

	// Delay and timeout configuration
	delayConfig *DelayConfigSet
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

// DelayConfig defines delay and timeout behavior for register access.
type DelayConfig struct {
	// Base delay to apply before responding (e.g., "100ms", "1s")
	Delay string `json:"delay,omitempty"`
	// Jitter percentage (0-100) to add random variance to delay
	// e.g., 20 means ±20% of Delay
	Jitter int `json:"jitter,omitempty"`
	// TimeoutProbability (0.0-1.0) is the probability of not responding at all
	// e.g., 0.3 means 30% of requests will timeout
	TimeoutProbability float64 `json:"timeoutProbability,omitempty"`
}

// RegisterType identifies one of the four Modbus register types.
type RegisterType string

const (
	RegisterTypeCoil          RegisterType = "coils"
	RegisterTypeDiscreteInput RegisterType = "discreteInputs"
	RegisterTypeHoldingReg    RegisterType = "holdingRegs"
	RegisterTypeInputReg      RegisterType = "inputRegs"
)

// DelayConfigSet contains global defaults and per-address delay configurations.
type DelayConfigSet struct {
	// Global default delays per register type
	Global map[RegisterType]DelayConfig `json:"global,omitempty"`
	// Per-address delay overrides for coils
	Coils map[uint16]DelayConfig `json:"coils,omitempty"`
	// Per-address delay overrides for discrete inputs
	DiscreteInputs map[uint16]DelayConfig `json:"discreteInputs,omitempty"`
	// Per-address delay overrides for holding registers
	HoldingRegs map[uint16]DelayConfig `json:"holdingRegs,omitempty"`
	// Per-address delay overrides for input registers
	InputRegs map[uint16]DelayConfig `json:"inputRegs,omitempty"`
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

	// Delay and timeout configuration
	Delays *DelayConfigSet `json:"delays,omitempty"`
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
		// Store delay configuration
		ds.delayConfig = config.Delays
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

// GetDelayConfig returns the applicable delay configuration for a given register type and address.
// It checks for address-specific overrides first, then falls back to global defaults.
// Returns nil if no delay configuration is found.
func (ds *DataStore) GetDelayConfig(regType RegisterType, address uint16) *DelayConfig {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	if ds.delayConfig == nil {
		return nil
	}

	// Check for address-specific override first
	var addressConfig *DelayConfig
	switch regType {
	case RegisterTypeCoil:
		if cfg, ok := ds.delayConfig.Coils[address]; ok {
			addressConfig = &cfg
		}
	case RegisterTypeDiscreteInput:
		if cfg, ok := ds.delayConfig.DiscreteInputs[address]; ok {
			addressConfig = &cfg
		}
	case RegisterTypeHoldingReg:
		if cfg, ok := ds.delayConfig.HoldingRegs[address]; ok {
			addressConfig = &cfg
		}
	case RegisterTypeInputReg:
		if cfg, ok := ds.delayConfig.InputRegs[address]; ok {
			addressConfig = &cfg
		}
	}

	// If address-specific config exists, return it
	if addressConfig != nil {
		return addressConfig
	}

	// Fall back to global default for this register type
	if ds.delayConfig.Global != nil {
		if cfg, ok := ds.delayConfig.Global[regType]; ok {
			return &cfg
		}
	}

	return nil
}

// ApplyDelay applies the configured delay and checks for timeout simulation.
// Returns true if the request should proceed, false if it should timeout (no response).
func (ds *DataStore) ApplyDelay(regType RegisterType, address uint16) bool {
	return ds.ApplyDelayWithOptions(regType, address, false)
}

// ApplyDelayWithOptions applies the configured delay and optionally checks for timeout simulation.
// Returns true if the request should proceed, false if it should timeout (no response).
// If disableTimeout is true, timeout probability is ignored (useful for RTU/ASCII where timeouts don't work with PTYs).
func (ds *DataStore) ApplyDelayWithOptions(regType RegisterType, address uint16, disableTimeout bool) bool {
	cfg := ds.GetDelayConfig(regType, address)
	if cfg == nil {
		return true // No delay configured, proceed normally
	}

	// Check timeout probability first (unless disabled)
	if !disableTimeout && cfg.TimeoutProbability > 0 {
		if rand.Float64() < cfg.TimeoutProbability {
			// Simulate timeout - return false to indicate no response should be sent
			return false
		}
	}

	// Parse and apply delay if configured
	if cfg.Delay != "" {
		baseDuration, err := time.ParseDuration(cfg.Delay)
		if err != nil {
			// Invalid duration, skip delay
			return true
		}

		// Apply jitter if configured
		delay := baseDuration
		if cfg.Jitter > 0 && cfg.Jitter <= 100 {
			// Calculate jitter range: delay * (jitter / 100)
			jitterRange := float64(baseDuration) * (float64(cfg.Jitter) / 100.0)
			// Random jitter between -jitterRange and +jitterRange
			jitterAmount := (rand.Float64()*2 - 1) * jitterRange
			delay = baseDuration + time.Duration(jitterAmount)

			// Ensure delay doesn't go negative
			if delay < 0 {
				delay = 0
			}
		}

		if delay > 0 {
			time.Sleep(delay)
		}
	}

	return true // Proceed with normal response
}
