# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go implementation of the Modbus protocol, supporting three communication modes:
- **TCP**: Modbus over TCP/IP networks
- **RTU**: Modbus over serial lines using binary encoding with CRC
- **ASCII**: Modbus over serial lines using ASCII encoding with LRC

The library provides a fault-tolerant, fail-fast client implementation for all standard Modbus function codes.

## Development Commands

### Using Make (Recommended)

```bash
make                    # Build both CLI and simulator (default)
make cli                # Build modbus-cli in bin/
make simulator          # Build modbus-simulator in bin/
make test               # Run all tests (unit + integration)
make test-unit          # Run unit tests only
make test-integration   # Run integration tests only
make test-coverage      # Run unit tests with coverage report
make clean              # Remove bin/ and coverage files
make help               # Show all available targets
```

### Direct Go Commands

**Build**:
```bash
go build -v ./...                              # Build all packages
go build -o bin/modbus-cli ./cmd/cli          # Build CLI tool
go build -o bin/modbus-simulator ./cmd/simulator  # Build simulator
```

**Run unit tests**:
```bash
go test -v -race .
```

**Run tests with coverage**:
```bash
go test -v -race -coverprofile=coverage.txt -covermode=atomic .
```

**Run linter**:
```bash
golangci-lint run --timeout=5m
```

**Run integration tests** (uses built-in simulator - no external dependencies required):
```bash
go test -v ./integration
# Or run specific protocol tests:
go test -v -run TCP ./integration
go test -v -run RTU ./integration
go test -v -run ASCII ./integration
```

**Verify go.mod is tidy**:
```bash
go mod tidy
git diff --exit-code go.mod go.sum
```

## Command-Line Tools

### Modbus CLI (`cmd/cli/main.go`)

A command-line interface for performing Modbus operations. Built with `urfave/cli/v2`.

**Supported commands**:
- `read-coils` - Read Coils (FC 1)
- `read-discrete-inputs` - Read Discrete Inputs (FC 2)
- `read-holding-registers` - Read Holding Registers (FC 3)
- `read-input-registers` - Read Input Registers (FC 4)
- `read-fifo` - Read FIFO Queue (FC 24)

**Global options**:
- `-p, --protocol` - Protocol: tcp, rtu, or ascii (default: tcp)
- `-a, --address` - Address (e.g., localhost:502 or /dev/ttyUSB0)
- `-s, --slave-id` - Slave ID (default: 1)
- `-t, --timeout` - Timeout duration (default: 5s)
- `--baud` - Baud rate for serial (default: 19200)
- `--parity` - Parity: N, E, or O (default: E)
- `--stop-bits` - Stop bits: 1 or 2 (default: 1)

**Example usage**:
```bash
# Read holding registers via TCP
./bin/modbus-cli -p tcp -a localhost:502 read-holding-registers --start 0 --count 10

# Read coils via RTU with custom serial settings
./bin/modbus-cli -p rtu -a /dev/ttyUSB0 --baud 9600 --parity N read-coils --start 0 --count 8

# Read input registers with hex output
./bin/modbus-cli -p tcp -a localhost:502 read-input-registers --start 100 --count 5 --format hex
```

### Modbus Simulator (`cmd/simulator/main.go`)

A standalone Modbus protocol simulator for testing. Uses the `internal/simulator/` package.

**Features**:
- Supports TCP, RTU, and ASCII modes
- Configurable via JSON files for initial register values
- Automatic PTY (pseudo-terminal) support for serial modes
- Named registers for readability (e.g., "battery_voltage")
- Implements all standard Modbus function codes
- Configurable delays and timeouts for fault tolerance testing

**Command-line options**:
- `-mode` - Server mode: tcp, rtu, or ascii (default: tcp)
- `-addr` - TCP address (e.g., localhost:502) or serial port
- `-slave-id` - Slave ID for serial modes (default: 1)
- `-baud` - Baud rate for serial (default: 19200)
- `-parity` - Parity: N, E, or O (default: E)
- `-stop-bits` - Stop bits: 1 or 2 (default: 1)
- `-config` - Path to JSON configuration file

**Example usage**:
```bash
# Run TCP simulator on port 5020
./bin/modbus-simulator -mode tcp -addr localhost:5020

# Run RTU simulator with configuration
./bin/modbus-simulator -mode rtu -slave-id 1 -baud 19200 -config testdata/simulator/solar-charger.json

# Run ASCII simulator on a specific serial port
./bin/modbus-simulator -mode ascii -addr /dev/ttyUSB0 -slave-id 1
```

**Configuration format** (`testdata/simulator/solar-charger.json`):
```json
{
  "NamedHoldingRegs": {
    "0": {"name": "pv_voltage", "value": 245},
    "1": {"name": "pv_current", "value": 82},
    "10": {"name": "battery_voltage", "value": 132}
  },
  "NamedInputRegs": {
    "0": {"name": "load_power", "value": 150}
  },
  "NamedCoils": {
    "0": {"name": "manual_control", "value": false}
  }
}
```

**Delay and Timeout Configuration**:

The simulator supports configurable delays and timeouts for testing fault tolerance. Add a `delays` section to your configuration:

```json
{
  "NamedHoldingRegs": {
    "100": {"name": "SLOW_REGISTER", "value": 1234}
  },
  "delays": {
    "global": {
      "holdingRegs": {"delay": "50ms", "jitter": 10}
    },
    "holdingRegs": {
      "100": {"delay": "500ms", "jitter": 20, "timeoutProbability": 0.3}
    }
  }
}
```

**Delay configuration fields**:
- `delay` - Base delay duration (e.g., "100ms", "1s", "500ms") - works with all protocols
- `jitter` - Percentage of random variance (0-100). E.g., 20 = ±20% random variance - works with all protocols
- `timeoutProbability` - Probability (0.0-1.0) of not responding. E.g., 0.3 = 30% timeout rate - **TCP only** (RTU/ASCII don't support timeout simulation)

**Configuration hierarchy**:
1. **Global defaults** - Applied to all registers of a type unless overridden
2. **Per-address overrides** - Override global defaults for specific addresses

Example: With the config above, reading holding register 100 will have a 500ms delay (±20% jitter) and a 30% chance of timeout, while other holding registers will have a 50ms delay (±10% jitter).

See `testdata/simulator/delays-example.json` and `testdata/simulator/README.md` for more examples.

## Architecture

The library uses a layered architecture that separates protocol concerns:

### Core Abstractions

1. **ProtocolDataUnit (PDU)**: Protocol-independent data structure containing function code and data
2. **Packager**: Encodes/decodes PDU into transport-specific frames (ADU - Application Data Unit)
3. **Transporter**: Handles physical communication (network/serial I/O)
4. **ClientHandler**: Interface combining Packager + Transporter

### Client Implementations

Each protocol has its own handler that implements both Packager and Transporter:

- **tcpclient.go**: `TCPClientHandler` - handles MBAP header, transaction IDs, TCP connection management
- **rtuclient.go**: `RTUClientHandler` - handles RTU framing with CRC-16 checksum
- **asciiclient.go**: `ASCIIClientHandler` - handles ASCII framing with LRC checksum

All three share the same `Client` interface (defined in api.go) with methods for all Modbus functions.

### Request Flow

1. Client method (e.g., `ReadHoldingRegisters`) creates a PDU
2. PDU is sent to `client.send()` (in client.go)
3. Packager encodes PDU → ADU with protocol-specific framing
4. Transporter sends ADU and receives response
5. Packager verifies and decodes response ADU → PDU
6. Client validates response and returns data

### Key Components

- **modbus.go**: Defines function codes, exception codes, core interfaces (Packager, Transporter), and ModbusError type
- **client.go**: Implements the `Client` interface with all Modbus function logic (validation, request/response handling)
- **api.go**: Defines the `Client` interface with all public methods
- **serial.go**: Common serial port functionality shared by RTU and ASCII
- **crc.go/lrc.go**: Checksum implementations for RTU (CRC-16) and ASCII (LRC)

## Implementation Details

### Connection Management

- TCP: Maintains persistent connection with idle timeout (default 60s)
- Serial (RTU/ASCII): Opens port on first use, closes after idle timeout
- All handlers implement `Connect()` and `Close()` for manual connection management
- Automatic reconnection on transporter failures

### Thread Safety

- TCP and ASCII transporters use mutexes to protect connection state
- RTU transporter uses mutexes to protect connection state
- Transaction IDs (TCP) use atomic operations for thread-safe increments

### Error Handling

- Modbus exceptions are returned as `*ModbusError` with function and exception codes
- Validation errors (quantity limits, value ranges) return standard Go errors
- Response validation checks data lengths, addresses, and checksums
- Context cancellation is checked between read operations, preventing indefinite hangs

### Serial Communication

- **RTU**:
  - Calculates frame delays based on baud rate (3.5 character times between frames)
  - Uses `Read()` in a loop with context checks between iterations (prevents indefinite hangs on partial responses)
  - Improved timeout handling compared to blocking `io.ReadFull()` approach
- **ASCII**: Reads until CRLF terminator or max buffer size with context checks in read loop
- Default serial config: 19200 baud, 8 data bits, 1 stop bit, even parity
- Timeout defaults to 5 seconds for both RTU and ASCII
- Context-based timeouts provide more reliable cancellation than serial port timeouts alone

## Testing

- **Unit tests**: Located in root directory (`*_test.go`), test individual components without external dependencies
- **Integration tests**: Located in `integration/` subdirectory, **fully automated with built-in simulator**
  - No external dependencies required (no diagslave, socat, etc.)
  - Uses `internal/simulator/` package for all protocol testing
  - TCP tests spin up a local TCP server
  - RTU/ASCII tests use PTY (pseudo-terminal) pairs for virtual serial ports
  - See `integration/README.md` for details on test architecture

## Project Structure

```
.
├── cmd/
│   ├── cli/           # Modbus CLI tool
│   └── simulator/     # Modbus simulator tool
├── internal/
│   ├── simulator/     # Simulator implementation (TCP, RTU, ASCII servers)
│   └── testutil/      # Test utilities
├── integration/       # Integration tests (fully automated)
├── testdata/
│   └── simulator/     # Example simulator configurations (e.g., solar-charger.json)
├── bin/               # Built executables (created by make)
├── *.go               # Core library files (modbus.go, client.go, tcpclient.go, etc.)
├── *_test.go          # Unit tests
├── Makefile           # Build and test targets
├── .goreleaser.yaml   # Release configuration for multi-platform builds
└── .github/workflows/ # CI/CD pipelines
```

## Dependencies

- **go.bug.st/serial** (v1.6.4) - Cross-platform serial port access
- **github.com/urfave/cli/v2** (v2.27.7) - CLI framework for modbus-cli
- **github.com/creack/pty** (v1.1.24) - PTY support for serial emulation in tests

## CI/CD

**Continuous Integration** (`.github/workflows/ci.yml`):
- Runs on every push and pull request
- Tests on Ubuntu and macOS
- Tests against Go 1.22, 1.23, 1.24, 1.25
- Runs unit tests with race detector
- Runs integration tests
- Runs golangci-lint v2.5
- Verifies `go.mod` is tidy

**Release Pipeline** (`.github/workflows/release.yml`):
- Triggered on version tags (v*)
- Uses GoReleaser v2 for multi-platform builds
- Builds for Linux and macOS (amd64 and arm64)
- Produces `modbus-cli` and `modbus-simulator` binaries
- Creates GitHub releases with archives

## Go Version

Minimum Go version: 1.22 (specified in go.mod)
CI tests against Go 1.22, 1.23, 1.24, 1.25 on Ubuntu and macOS

## Development Best Practices

- Always run golangci-lint on new code
- Run tests with race detector: `go test -v -race ./...`
- Use `make test` to run both unit and integration tests before committing
- Keep `go.mod` tidy: `go mod tidy && git diff --exit-code go.mod go.sum`

## Simulator Implementation (`internal/simulator/`)

The simulator package provides a complete Modbus server implementation for testing purposes:

### Components

- **handler.go** (~15 KB) - Core request handler implementing all Modbus function codes:
  - Validates slave IDs, addresses, and quantities
  - Returns proper Modbus exceptions for invalid requests
  - Supports all 11 standard Modbus functions

- **datastore.go** - In-memory storage for Modbus data:
  - Coils, discrete inputs, holding registers, input registers
  - Support for named registers (e.g., "battery_voltage": 132)
  - JSON configuration loading
  - Thread-safe access with mutexes

- **tcp_server.go** - TCP/IP server with MBAP header handling
- **rtu_server.go** - RTU server with CRC-16 checksums and frame timing
- **ascii_server.go** - ASCII server with LRC checksums and CRLF terminators
- **pty.go** - Pseudo-terminal creation for serial emulation in tests
- **crc.go / lrc.go** - Checksum utilities

### Usage in Tests

Integration tests use the simulator programmatically:

```go
// TCP example
server := simulator.NewTCPServer(":0", 1)
go server.Start()
defer server.Stop()

// RTU example (with PTY)
ptyPath, err := simulator.CreatePTY()
server := simulator.NewRTUServer(ptyPath, 1, 19200, "E", 1)
```

This approach eliminates the need for external Modbus simulators like diagslave.