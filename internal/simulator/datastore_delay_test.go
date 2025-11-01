// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package simulator

import (
	"testing"
	"time"
)

func TestDelayConfig_Lookup(t *testing.T) {
	config := &DataStoreConfig{
		Delays: &DelayConfigSet{
			Global: map[RegisterType]DelayConfig{
				RegisterTypeHoldingReg: {
					Delay:  "50ms",
					Jitter: 10,
				},
			},
			HoldingRegs: map[uint16]DelayConfig{
				100: {
					Delay:  "200ms",
					Jitter: 20,
				},
				200: {
					TimeoutProbability: 1.0,
				},
			},
		},
	}

	ds := NewDataStore(config)

	tests := []struct {
		name            string
		regType         RegisterType
		address         uint16
		expectNil       bool
		expectedDelay   string
		expectedJitter  int
		expectedTimeout float64
	}{
		{
			name:           "address-specific override",
			regType:        RegisterTypeHoldingReg,
			address:        100,
			expectNil:      false,
			expectedDelay:  "200ms",
			expectedJitter: 20,
		},
		{
			name:            "timeout probability",
			regType:         RegisterTypeHoldingReg,
			address:         200,
			expectNil:       false,
			expectedTimeout: 1.0,
		},
		{
			name:           "global default",
			regType:        RegisterTypeHoldingReg,
			address:        999,
			expectNil:      false,
			expectedDelay:  "50ms",
			expectedJitter: 10,
		},
		{
			name:      "no config",
			regType:   RegisterTypeCoil,
			address:   0,
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ds.GetDelayConfig(tt.regType, tt.address)
			if tt.expectNil {
				if cfg != nil {
					t.Errorf("expected nil config, got %+v", cfg)
				}
				return
			}
			if cfg == nil {
				t.Fatal("expected non-nil config")
			}
			if tt.expectedDelay != "" && cfg.Delay != tt.expectedDelay {
				t.Errorf("expected delay %s, got %s", tt.expectedDelay, cfg.Delay)
			}
			if tt.expectedJitter != 0 && cfg.Jitter != tt.expectedJitter {
				t.Errorf("expected jitter %d, got %d", tt.expectedJitter, cfg.Jitter)
			}
			if tt.expectedTimeout != 0 && cfg.TimeoutProbability != tt.expectedTimeout {
				t.Errorf("expected timeout probability %f, got %f", tt.expectedTimeout, cfg.TimeoutProbability)
			}
		})
	}
}

func TestApplyDelay_NoConfig(t *testing.T) {
	ds := NewDataStore(nil)

	start := time.Now()
	shouldProceed := ds.ApplyDelay(RegisterTypeHoldingReg, 100)
	elapsed := time.Since(start)

	if !shouldProceed {
		t.Error("expected to proceed when no config")
	}
	if elapsed > 10*time.Millisecond {
		t.Errorf("expected no delay, but took %v", elapsed)
	}
}

func TestApplyDelay_FixedDelay(t *testing.T) {
	config := &DataStoreConfig{
		Delays: &DelayConfigSet{
			HoldingRegs: map[uint16]DelayConfig{
				100: {
					Delay:  "100ms",
					Jitter: 0,
				},
			},
		},
	}

	ds := NewDataStore(config)

	start := time.Now()
	shouldProceed := ds.ApplyDelay(RegisterTypeHoldingReg, 100)
	elapsed := time.Since(start)

	if !shouldProceed {
		t.Error("expected to proceed with fixed delay")
	}

	// Check delay is approximately correct (within 20ms tolerance)
	expectedDelay := 100 * time.Millisecond
	if elapsed < expectedDelay-20*time.Millisecond || elapsed > expectedDelay+20*time.Millisecond {
		t.Errorf("expected delay around %v, got %v", expectedDelay, elapsed)
	}
}

func TestApplyDelay_WithJitter(t *testing.T) {
	config := &DataStoreConfig{
		Delays: &DelayConfigSet{
			HoldingRegs: map[uint16]DelayConfig{
				100: {
					Delay:  "100ms",
					Jitter: 50, // ±50%
				},
			},
		},
	}

	ds := NewDataStore(config)

	// Run multiple times to test jitter range
	minDelay := time.Duration(1<<63 - 1) // max duration
	maxDelay := time.Duration(0)

	for i := 0; i < 20; i++ {
		start := time.Now()
		shouldProceed := ds.ApplyDelay(RegisterTypeHoldingReg, 100)
		elapsed := time.Since(start)

		if !shouldProceed {
			t.Error("expected to proceed with jitter")
		}

		if elapsed < minDelay {
			minDelay = elapsed
		}
		if elapsed > maxDelay {
			maxDelay = elapsed
		}
	}

	// With 50% jitter, delay should be between 50ms and 150ms
	expectedMin := 50 * time.Millisecond
	expectedMax := 150 * time.Millisecond

	if minDelay < expectedMin-20*time.Millisecond {
		t.Errorf("min delay %v below expected %v", minDelay, expectedMin)
	}
	if maxDelay > expectedMax+20*time.Millisecond {
		t.Errorf("max delay %v above expected %v", maxDelay, expectedMax)
	}
}

func TestApplyDelay_TimeoutProbability(t *testing.T) {
	config := &DataStoreConfig{
		Delays: &DelayConfigSet{
			HoldingRegs: map[uint16]DelayConfig{
				100: {
					TimeoutProbability: 0.5, // 50% timeout
				},
			},
		},
	}

	ds := NewDataStore(config)

	// Run many times and count timeouts
	timeoutCount := 0
	iterations := 100

	for i := 0; i < iterations; i++ {
		shouldProceed := ds.ApplyDelay(RegisterTypeHoldingReg, 100)
		if !shouldProceed {
			timeoutCount++
		}
	}

	// With 50% probability and 100 iterations, expect around 50 timeouts
	// Allow 20-80 range for statistical variance
	if timeoutCount < 20 || timeoutCount > 80 {
		t.Errorf("expected around 50 timeouts, got %d out of %d", timeoutCount, iterations)
	}
}

func TestApplyDelay_AlwaysTimeout(t *testing.T) {
	config := &DataStoreConfig{
		Delays: &DelayConfigSet{
			HoldingRegs: map[uint16]DelayConfig{
				100: {
					TimeoutProbability: 1.0, // Always timeout
				},
			},
		},
	}

	ds := NewDataStore(config)

	for i := 0; i < 10; i++ {
		shouldProceed := ds.ApplyDelay(RegisterTypeHoldingReg, 100)
		if shouldProceed {
			t.Error("expected timeout with probability 1.0")
		}
	}
}

func TestApplyDelay_NeverTimeout(t *testing.T) {
	config := &DataStoreConfig{
		Delays: &DelayConfigSet{
			HoldingRegs: map[uint16]DelayConfig{
				100: {
					Delay:              "10ms",
					TimeoutProbability: 0.0, // Never timeout
				},
			},
		},
	}

	ds := NewDataStore(config)

	for i := 0; i < 10; i++ {
		shouldProceed := ds.ApplyDelay(RegisterTypeHoldingReg, 100)
		if !shouldProceed {
			t.Error("expected no timeout with probability 0.0")
		}
	}
}

func TestApplyDelay_InvalidDuration(t *testing.T) {
	config := &DataStoreConfig{
		Delays: &DelayConfigSet{
			HoldingRegs: map[uint16]DelayConfig{
				100: {
					Delay: "invalid",
				},
			},
		},
	}

	ds := NewDataStore(config)

	start := time.Now()
	shouldProceed := ds.ApplyDelay(RegisterTypeHoldingReg, 100)
	elapsed := time.Since(start)

	if !shouldProceed {
		t.Error("expected to proceed with invalid duration")
	}
	if elapsed > 10*time.Millisecond {
		t.Errorf("expected no delay with invalid duration, but took %v", elapsed)
	}
}

func TestApplyDelay_AllRegisterTypes(t *testing.T) {
	config := &DataStoreConfig{
		Delays: &DelayConfigSet{
			Global: map[RegisterType]DelayConfig{
				RegisterTypeCoil:          {Delay: "10ms"},
				RegisterTypeDiscreteInput: {Delay: "20ms"},
				RegisterTypeHoldingReg:    {Delay: "30ms"},
				RegisterTypeInputReg:      {Delay: "40ms"},
			},
		},
	}

	ds := NewDataStore(config)

	tests := []struct {
		regType       RegisterType
		expectedDelay time.Duration
	}{
		{RegisterTypeCoil, 10 * time.Millisecond},
		{RegisterTypeDiscreteInput, 20 * time.Millisecond},
		{RegisterTypeHoldingReg, 30 * time.Millisecond},
		{RegisterTypeInputReg, 40 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(string(tt.regType), func(t *testing.T) {
			start := time.Now()
			shouldProceed := ds.ApplyDelay(tt.regType, 0)
			elapsed := time.Since(start)

			if !shouldProceed {
				t.Error("expected to proceed")
			}

			// Allow 15ms tolerance
			if elapsed < tt.expectedDelay-15*time.Millisecond || elapsed > tt.expectedDelay+15*time.Millisecond {
				t.Errorf("expected delay around %v, got %v", tt.expectedDelay, elapsed)
			}
		})
	}
}
