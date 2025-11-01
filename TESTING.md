# Testing Guide

## Running Tests

### All Tests (Local Development)
```bash
make test              # Run all tests (unit + integration)
make test-unit         # Run unit tests only
make test-integration  # Run integration tests only
```

### CI Tests
The CI pipeline skips certain timing-sensitive tests that are flaky in automated environments:

```bash
# This is what CI runs:
go test -v -race -skip 'Jitter|MixedTimeout|TimeoutMultiple|TimeoutDifferent' ./integration
```

## Flaky Tests (Excluded from CI)

The following tests are **excluded from CI** but remain available for local testing. They test timing-sensitive features like jitter, probabilistic timeouts, and multiple sequential timeout scenarios which can be unreliable in CI environments due to CPU scheduling variations.

### 1. TestClientWithJitter
Tests timing variance with jitter configuration (±50%).

**Run locally:**
```bash
go test -v -run TestClientWithJitter ./integration
```

### 2. TestTCPClientMixedTimeoutProbability  
Tests probabilistic timeout simulation with 50% timeout probability.

**Run locally:**
```bash
go test -v -run TestTCPClientMixedTimeoutProbability ./integration
```

### 3. TestTCPClientTimeoutMultipleRequests
Tests multiple consecutive timeout requests (100% timeout probability).

**Run locally:**
```bash
go test -v -run TestTCPClientTimeoutMultipleRequests ./integration
```

### 4. TestTCPClientTimeoutDifferentFunctionCodes
Tests timeout behavior across different Modbus function codes (100% timeout probability).

**Run locally:**
```bash
go test -v -run TestTCPClientTimeoutDifferentFunctionCodes ./integration
```

### Run All Flaky Tests
```bash
go test -v -run 'Jitter|MixedTimeout|TimeoutMultiple|TimeoutDifferent' ./integration
```

## Why These Tests Are Flaky

These tests simulate timing-sensitive behavior (delays, jitter, timeouts) which depends on precise CPU timing. In CI environments:
- **Variable CPU scheduling** - VMs may be oversubscribed or throttled
- **Limited resources** - Multiple jobs running on the same host
- **Network timing variations** - Even localhost connections can have unexpected latency
- **Random number generation** - Probabilistic tests can occasionally hit edge cases

While these tests are valuable for local development and manual verification of timing features, they're not reliable enough for automated CI pipelines.

## Test Coverage

CI tests cover all core functionality:
- ✅ All Modbus function codes (Read/Write Coils, Registers, etc.)
- ✅ All protocols (TCP, RTU, ASCII)
- ✅ Error handling and validation
- ✅ Basic delay simulation (non-probabilistic)
- ✅ Basic timeout simulation (deterministic cases)
- ❌ Jitter and probabilistic timeout tests (flaky, local only)
