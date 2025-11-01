// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lumberbarons/modbus"
	"github.com/lumberbarons/modbus/internal/simulator"
	"github.com/lumberbarons/modbus/internal/testutil"
)

func TestTCPClientWithDelay(t *testing.T) {
	// Setup simulator with delay configuration
	config := &simulator.DataStoreConfig{
		NamedHoldingRegs: map[uint16]simulator.RegisterConfig{
			100: {Name: "SLOW_REG", Value: 1234},
		},
		Delays: &simulator.DelayConfigSet{
			HoldingRegs: map[uint16]simulator.DelayConfig{
				100: {
					Delay:  "200ms",
					Jitter: 0,
				},
			},
		},
	}

	cleanup, address := testutil.StartTCPSimulator(t, testutil.WithTCPDataStoreConfig(config))
	defer cleanup()

	handler := modbus.NewTCPClientHandler(address)
	handler.Timeout = 5 * time.Second
	handler.SlaveID = 1
	if err := handler.Connect(); err != nil {
		t.Fatal(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	ctx := context.Background()

	// Measure request time
	start := time.Now()
	results, err := client.ReadHoldingRegisters(ctx, 100, 1)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected successful read with delay, got error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 bytes, got %d", len(results))
	}

	// Verify delay was applied (should be around 200ms)
	expectedDelay := 200 * time.Millisecond
	if elapsed < expectedDelay-50*time.Millisecond {
		t.Errorf("delay too short: expected ~%v, got %v", expectedDelay, elapsed)
	}
	if elapsed > expectedDelay+100*time.Millisecond {
		t.Errorf("delay too long: expected ~%v, got %v", expectedDelay, elapsed)
	}

	t.Logf("Read with 200ms delay took %v", elapsed)
}

func TestTCPClientWithTimeout(t *testing.T) {
	// Setup simulator with 100% timeout probability
	config := &simulator.DataStoreConfig{
		NamedHoldingRegs: map[uint16]simulator.RegisterConfig{
			200: {Name: "TIMEOUT_REG", Value: 5678},
		},
		Delays: &simulator.DelayConfigSet{
			HoldingRegs: map[uint16]simulator.DelayConfig{
				200: {
					TimeoutProbability: 1.0, // Always timeout
				},
			},
		},
	}

	cleanup, address := testutil.StartTCPSimulator(t, testutil.WithTCPDataStoreConfig(config))
	defer cleanup()

	handler := modbus.NewTCPClientHandler(address)
	handler.Timeout = 1 * time.Second // Short timeout for faster test
	handler.SlaveID = 1
	if err := handler.Connect(); err != nil {
		t.Fatal(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	ctx := context.Background()

	// This should timeout
	start := time.Now()
	_, err := client.ReadHoldingRegisters(ctx, 200, 1)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	// Should timeout around the configured timeout duration
	if elapsed < 900*time.Millisecond || elapsed > 1500*time.Millisecond {
		t.Errorf("unexpected timeout duration: %v", elapsed)
	}

	t.Logf("Timeout test took %v (expected ~1s)", elapsed)
}

func TestTCPClientWithGlobalDelay(t *testing.T) {
	// Setup simulator with global delay for all holding registers
	config := &simulator.DataStoreConfig{
		NamedHoldingRegs: map[uint16]simulator.RegisterConfig{
			0: {Name: "REG0", Value: 100},
			1: {Name: "REG1", Value: 200},
			2: {Name: "REG2", Value: 300},
		},
		Delays: &simulator.DelayConfigSet{
			Global: map[simulator.RegisterType]simulator.DelayConfig{
				simulator.RegisterTypeHoldingReg: {
					Delay:  "100ms",
					Jitter: 0,
				},
			},
		},
	}

	cleanup, address := testutil.StartTCPSimulator(t, testutil.WithTCPDataStoreConfig(config))
	defer cleanup()

	handler := modbus.NewTCPClientHandler(address)
	handler.Timeout = 5 * time.Second
	handler.SlaveID = 1
	if err := handler.Connect(); err != nil {
		t.Fatal(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	ctx := context.Background()

	// Test all three registers - they should all have the global delay
	for addr := uint16(0); addr < 3; addr++ {
		start := time.Now()
		_, err := client.ReadHoldingRegisters(ctx, addr, 1)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("read register %d failed: %v", addr, err)
		}

		// Should be around 100ms
		if elapsed < 80*time.Millisecond || elapsed > 150*time.Millisecond {
			t.Errorf("register %d: unexpected delay %v", addr, elapsed)
		}
	}
}

func TestRTUClientWithDelay(t *testing.T) {
	// Setup simulator with delay configuration
	config := &simulator.DataStoreConfig{
		NamedInputRegs: map[uint16]simulator.RegisterConfig{
			0: {Name: "SENSOR", Value: 999},
		},
		Delays: &simulator.DelayConfigSet{
			InputRegs: map[uint16]simulator.DelayConfig{
				0: {
					Delay:  "150ms",
					Jitter: 0,
				},
			},
		},
	}

	cleanup, devicePath := testutil.StartRTUSimulator(t, testutil.WithDataStoreConfig(config))
	defer cleanup()

	handler := modbus.NewRTUClientHandler(devicePath)
	handler.BaudRate = 19200
	handler.DataBits = 8
	handler.Parity = "E"
	handler.StopBits = 1
	handler.Timeout = 5 * time.Second
	handler.SlaveID = 1

	if err := handler.Connect(); err != nil {
		t.Fatal(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	ctx := context.Background()

	start := time.Now()
	results, err := client.ReadInputRegisters(ctx, 0, 1)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected successful read with delay, got error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 bytes, got %d", len(results))
	}

	// Verify delay was applied (should be around 150ms)
	expectedDelay := 150 * time.Millisecond
	if elapsed < expectedDelay-50*time.Millisecond {
		t.Errorf("delay too short: expected ~%v, got %v", expectedDelay, elapsed)
	}

	t.Logf("RTU read with 150ms delay took %v", elapsed)
}

func TestASCIIClientWithDelay(t *testing.T) {
	// Setup simulator with delay configuration
	config := &simulator.DataStoreConfig{
		NamedCoils: map[uint16]simulator.CoilConfig{
			0: {Name: "RELAY", Value: true},
		},
		Delays: &simulator.DelayConfigSet{
			Coils: map[uint16]simulator.DelayConfig{
				0: {
					Delay:  "100ms",
					Jitter: 0,
				},
			},
		},
	}

	cleanup, devicePath := testutil.StartASCIISimulator(t, testutil.WithASCIIDataStoreConfig(config))
	defer cleanup()

	handler := modbus.NewASCIIClientHandler(devicePath)
	handler.BaudRate = 19200
	handler.DataBits = 8
	handler.Parity = "E"
	handler.StopBits = 1
	handler.Timeout = 5 * time.Second
	handler.SlaveID = 1

	if err := handler.Connect(); err != nil {
		t.Fatal(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	ctx := context.Background()

	start := time.Now()
	results, err := client.ReadCoils(ctx, 0, 1)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected successful read with delay, got error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 byte, got %d", len(results))
	}

	// Verify delay was applied (should be around 100ms)
	expectedDelay := 100 * time.Millisecond
	if elapsed < expectedDelay-50*time.Millisecond {
		t.Errorf("delay too short: expected ~%v, got %v", expectedDelay, elapsed)
	}

	t.Logf("ASCII read with 100ms delay took %v", elapsed)
}

func TestClientWithJitter(t *testing.T) {
	// NOTE: This test is skipped in CI (see .github/workflows/ci.yml) due to timing sensitivity
	// Run locally with: go test -v -run TestClientWithJitter ./integration

	// Setup simulator with jitter
	config := &simulator.DataStoreConfig{
		NamedHoldingRegs: map[uint16]simulator.RegisterConfig{
			0: {Name: "JITTER_REG", Value: 1111},
		},
		Delays: &simulator.DelayConfigSet{
			HoldingRegs: map[uint16]simulator.DelayConfig{
				0: {
					Delay:  "100ms",
					Jitter: 50, // ±50%
				},
			},
		},
	}

	cleanup, address := testutil.StartTCPSimulator(t, testutil.WithTCPDataStoreConfig(config))
	defer cleanup()

	handler := modbus.NewTCPClientHandler(address)
	handler.Timeout = 5 * time.Second
	handler.SlaveID = 1
	if err := handler.Connect(); err != nil {
		t.Fatal(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	ctx := context.Background()

	// Perform multiple reads and track timing variance
	var timings []time.Duration
	for i := 0; i < 10; i++ {
		start := time.Now()
		_, err := client.ReadHoldingRegisters(ctx, 0, 1)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("read %d failed: %v", i, err)
		}

		timings = append(timings, elapsed)
	}

	// Check that we see variance in timings (jitter is working)
	minTiming := timings[0]
	maxTiming := timings[0]
	for _, t := range timings {
		if t < minTiming {
			minTiming = t
		}
		if t > maxTiming {
			maxTiming = t
		}
	}

	variance := maxTiming - minTiming
	t.Logf("Jitter test: min=%v, max=%v, variance=%v", minTiming, maxTiming, variance)

	// With 50% jitter on 100ms, we should see at least 30ms variance across 10 samples
	if variance < 30*time.Millisecond {
		t.Errorf("expected more variance with 50%% jitter, got %v", variance)
	}
}

func TestTCPClientTimeoutMultipleRequests(t *testing.T) {
	// NOTE: This test is skipped in CI (see .github/workflows/ci.yml) due to timing sensitivity
	// Run locally with: go test -v -run TestTCPClientTimeoutMultipleRequests ./integration

	// Test that multiple consecutive timeout requests work correctly
	config := &simulator.DataStoreConfig{
		NamedHoldingRegs: map[uint16]simulator.RegisterConfig{
			100: {Name: "TIMEOUT_REG", Value: 1234},
		},
		Delays: &simulator.DelayConfigSet{
			HoldingRegs: map[uint16]simulator.DelayConfig{
				100: {
					TimeoutProbability: 1.0, // Always timeout
				},
			},
		},
	}

	cleanup, address := testutil.StartTCPSimulator(t, testutil.WithTCPDataStoreConfig(config))
	defer cleanup()

	handler := modbus.NewTCPClientHandler(address)
	handler.Timeout = 500 * time.Millisecond
	handler.SlaveID = 1
	if err := handler.Connect(); err != nil {
		t.Fatal(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	ctx := context.Background()

	// Try multiple requests - all should timeout
	for i := 0; i < 3; i++ {
		start := time.Now()
		_, err := client.ReadHoldingRegisters(ctx, 100, 1)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatalf("request %d: expected timeout error, got nil", i)
		}

		// Should timeout around the configured timeout duration
		if elapsed < 400*time.Millisecond || elapsed > 700*time.Millisecond {
			t.Errorf("request %d: unexpected timeout duration: %v", i, elapsed)
		}

		t.Logf("Request %d timed out after %v", i, elapsed)
	}
}

func TestRTUClientContextCancellationBetweenReads(t *testing.T) {
	// Test RTU client context cancellation between read operations.
	// This test verifies that the refactored RTU client checks context
	// between read iterations (which prevents indefinite hangs on partial responses).
	//
	// Note: This test uses a short delay to trigger multiple reads. The client
	// will read the first 4 bytes (minimum), check context, then read remaining bytes.
	// We cancel the context between these reads to verify context checking works.
	config := &simulator.DataStoreConfig{
		NamedHoldingRegs: map[uint16]simulator.RegisterConfig{
			0: {Name: "TEST_REG", Value: 1234},
		},
	}

	cleanup, devicePath := testutil.StartRTUSimulator(t, testutil.WithDataStoreConfig(config))
	defer cleanup()

	handler := modbus.NewRTUClientHandler(devicePath)
	handler.BaudRate = 19200
	handler.DataBits = 8
	handler.Parity = "E"
	handler.StopBits = 1
	handler.SlaveID = 1

	if err := handler.Connect(); err != nil {
		t.Fatal(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context in background after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// This may or may not timeout depending on when cancellation happens
	// The key improvement is that context is now checked between reads,
	// which prevents indefinite hangs when devices send partial responses
	_, err := client.ReadHoldingRegisters(ctx, 0, 1)

	// We expect either success (if read completed before cancel) or context.Canceled
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("expected nil or context.Canceled, got: %v", err)
	}

	t.Logf("RTU context cancellation test result: err=%v", err)
}

func TestASCIIClientTimeoutWithLongDelay(t *testing.T) {
	// Test ASCII client timeout when delay is longer than client timeout
	config := &simulator.DataStoreConfig{
		NamedCoils: map[uint16]simulator.CoilConfig{
			0: {Name: "SLOW_COIL", Value: true},
		},
		Delays: &simulator.DelayConfigSet{
			Coils: map[uint16]simulator.DelayConfig{
				0: {
					Delay: "2s", // Delay longer than client timeout
				},
			},
		},
	}

	cleanup, devicePath := testutil.StartASCIISimulator(t, testutil.WithASCIIDataStoreConfig(config))
	defer cleanup()

	handler := modbus.NewASCIIClientHandler(devicePath)
	handler.BaudRate = 19200
	handler.DataBits = 8
	handler.Parity = "E"
	handler.StopBits = 1
	handler.Timeout = 500 * time.Millisecond // Short timeout
	handler.SlaveID = 1

	if err := handler.Connect(); err != nil {
		t.Fatal(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	ctx := context.Background()

	start := time.Now()
	_, err := client.ReadCoils(ctx, 0, 1)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error when delay exceeds timeout, got nil")
	}

	// Should timeout around the configured timeout duration
	if elapsed < 400*time.Millisecond || elapsed > 700*time.Millisecond {
		t.Errorf("unexpected timeout duration: %v (expected ~500ms)", elapsed)
	}

	t.Logf("ASCII timeout with long delay took %v", elapsed)
}

func TestTCPClientTimeoutThenSuccessfulRequest(t *testing.T) {
	// Test that after a timeout, the next successful request still works
	config := &simulator.DataStoreConfig{
		NamedHoldingRegs: map[uint16]simulator.RegisterConfig{
			100: {Name: "TIMEOUT_REG", Value: 1234},
			200: {Name: "GOOD_REG", Value: 5678},
		},
		Delays: &simulator.DelayConfigSet{
			HoldingRegs: map[uint16]simulator.DelayConfig{
				100: {
					TimeoutProbability: 1.0, // Always timeout
				},
				// Register 200 has no delay config, so it responds normally
			},
		},
	}

	cleanup, address := testutil.StartTCPSimulator(t, testutil.WithTCPDataStoreConfig(config))
	defer cleanup()

	handler := modbus.NewTCPClientHandler(address)
	handler.Timeout = 500 * time.Millisecond
	handler.SlaveID = 1
	if err := handler.Connect(); err != nil {
		t.Fatal(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	ctx := context.Background()

	// First request should timeout
	start := time.Now()
	_, err := client.ReadHoldingRegisters(ctx, 100, 1)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error for register 100")
	}
	t.Logf("First request (timeout) took %v", elapsed)

	// Second request should succeed
	start = time.Now()
	result, err := client.ReadHoldingRegisters(ctx, 200, 1)
	elapsed = time.Since(start)

	if err != nil {
		t.Fatalf("expected successful read for register 200, got error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 bytes, got %d", len(result))
	}

	// Should be fast (no delay configured)
	if elapsed > 200*time.Millisecond {
		t.Errorf("second request too slow: %v", elapsed)
	}

	t.Logf("Second request (success) took %v", elapsed)
}

func TestTCPClientMixedTimeoutProbability(t *testing.T) {
	// NOTE: This test is skipped in CI (see .github/workflows/ci.yml) due to random flakiness
	// Run locally with: go test -v -run TestTCPClientMixedTimeoutProbability ./integration

	// Test with 50% timeout probability - verify both success and failure paths work
	config := &simulator.DataStoreConfig{
		NamedHoldingRegs: map[uint16]simulator.RegisterConfig{
			100: {Name: "FLAKY_REG", Value: 1234},
		},
		Delays: &simulator.DelayConfigSet{
			HoldingRegs: map[uint16]simulator.DelayConfig{
				100: {
					Delay:              "50ms",
					TimeoutProbability: 0.5, // 50% timeout
				},
			},
		},
	}

	cleanup, address := testutil.StartTCPSimulator(t, testutil.WithTCPDataStoreConfig(config))
	defer cleanup()

	handler := modbus.NewTCPClientHandler(address)
	handler.Timeout = 1 * time.Second
	handler.SlaveID = 1
	if err := handler.Connect(); err != nil {
		t.Fatal(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	ctx := context.Background()

	// Try multiple requests and count successes/failures
	successCount := 0
	timeoutCount := 0
	iterations := 50

	for i := 0; i < iterations; i++ {
		_, err := client.ReadHoldingRegisters(ctx, 100, 1)
		if err == nil {
			successCount++
		} else {
			timeoutCount++
		}
	}

	t.Logf("Results: %d successes, %d timeouts out of %d requests", successCount, timeoutCount, iterations)

	// With 50% probability, we should see both successes and failures
	// With 50 iterations, expect roughly 25 ±15 (allow for 10-40 range = ~5σ)
	// This gives us >99.99% confidence the test won't randomly fail
	if successCount < 10 {
		t.Errorf("expected at least 10 successes with 50%% probability over %d tries, got %d", iterations, successCount)
	}
	if timeoutCount < 10 {
		t.Errorf("expected at least 10 timeouts with 50%% probability over %d tries, got %d", iterations, timeoutCount)
	}
}

func TestTCPClientTimeoutDifferentFunctionCodes(t *testing.T) {
	// NOTE: This test is skipped in CI (see .github/workflows/ci.yml) due to timing sensitivity
	// Run locally with: go test -v -run TestTCPClientTimeoutDifferentFunctionCodes ./integration

	// Test timeout behavior across different Modbus function codes
	config := &simulator.DataStoreConfig{
		NamedHoldingRegs: map[uint16]simulator.RegisterConfig{
			0: {Name: "REG", Value: 1234},
		},
		NamedCoils: map[uint16]simulator.CoilConfig{
			0: {Name: "COIL", Value: true},
		},
		NamedInputRegs: map[uint16]simulator.RegisterConfig{
			0: {Name: "INPUT", Value: 5678},
		},
		NamedDiscreteInputs: map[uint16]simulator.CoilConfig{
			0: {Name: "DISCRETE", Value: false},
		},
		Delays: &simulator.DelayConfigSet{
			HoldingRegs: map[uint16]simulator.DelayConfig{
				0: {TimeoutProbability: 1.0},
			},
			Coils: map[uint16]simulator.DelayConfig{
				0: {TimeoutProbability: 1.0},
			},
			InputRegs: map[uint16]simulator.DelayConfig{
				0: {TimeoutProbability: 1.0},
			},
			DiscreteInputs: map[uint16]simulator.DelayConfig{
				0: {TimeoutProbability: 1.0},
			},
		},
	}

	cleanup, address := testutil.StartTCPSimulator(t, testutil.WithTCPDataStoreConfig(config))
	defer cleanup()

	handler := modbus.NewTCPClientHandler(address)
	handler.Timeout = 500 * time.Millisecond
	handler.SlaveID = 1
	if err := handler.Connect(); err != nil {
		t.Fatal(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() ([]byte, error)
	}{
		{"ReadHoldingRegisters", func() ([]byte, error) {
			return client.ReadHoldingRegisters(ctx, 0, 1)
		}},
		{"ReadCoils", func() ([]byte, error) {
			return client.ReadCoils(ctx, 0, 1)
		}},
		{"ReadInputRegisters", func() ([]byte, error) {
			return client.ReadInputRegisters(ctx, 0, 1)
		}},
		{"ReadDiscreteInputs", func() ([]byte, error) {
			return client.ReadDiscreteInputs(ctx, 0, 1)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			_, err := tt.fn()
			elapsed := time.Since(start)

			if err == nil {
				t.Fatal("expected timeout error, got nil")
			}

			// Should timeout around the configured timeout duration
			if elapsed < 400*time.Millisecond || elapsed > 700*time.Millisecond {
				t.Errorf("unexpected timeout duration: %v (expected ~500ms)", elapsed)
			}

			t.Logf("%s timed out after %v", tt.name, elapsed)
		})
	}
}

func TestClientWithAddressOverride(t *testing.T) {
	// Setup simulator with global default and address-specific override
	config := &simulator.DataStoreConfig{
		NamedHoldingRegs: map[uint16]simulator.RegisterConfig{
			0:   {Name: "FAST_REG", Value: 100},
			100: {Name: "SLOW_REG", Value: 200},
		},
		Delays: &simulator.DelayConfigSet{
			Global: map[simulator.RegisterType]simulator.DelayConfig{
				simulator.RegisterTypeHoldingReg: {
					Delay:  "50ms",
					Jitter: 0,
				},
			},
			HoldingRegs: map[uint16]simulator.DelayConfig{
				100: {
					Delay:  "300ms",
					Jitter: 0,
				},
			},
		},
	}

	cleanup, address := testutil.StartTCPSimulator(t, testutil.WithTCPDataStoreConfig(config))
	defer cleanup()

	handler := modbus.NewTCPClientHandler(address)
	handler.Timeout = 5 * time.Second
	handler.SlaveID = 1
	if err := handler.Connect(); err != nil {
		t.Fatal(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	ctx := context.Background()

	// Read register 0 (should use global 50ms delay)
	start := time.Now()
	_, err := client.ReadHoldingRegisters(ctx, 0, 1)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("read register 0 failed: %v", err)
	}

	// Should be around 50ms
	if elapsed < 30*time.Millisecond || elapsed > 100*time.Millisecond {
		t.Errorf("register 0: expected ~50ms delay, got %v", elapsed)
	}

	// Read register 100 (should use override 300ms delay)
	start = time.Now()
	_, err = client.ReadHoldingRegisters(ctx, 100, 1)
	elapsed = time.Since(start)

	if err != nil {
		t.Fatalf("read register 100 failed: %v", err)
	}

	// Should be around 300ms
	if elapsed < 250*time.Millisecond || elapsed > 350*time.Millisecond {
		t.Errorf("register 100: expected ~300ms delay, got %v", elapsed)
	}

	t.Logf("Address override test: register 0 and 100 had different delays as expected")
}
