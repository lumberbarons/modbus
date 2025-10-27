# Integration Tests for Modbus Library

## Overview

The integration tests are **fully automated** and require no external tools or manual setup. They use the built-in Modbus simulator located in `internal/simulator/`.

## Running Tests

Simply run the tests directly:

```bash
# Run all integration tests
$ go test -v ./integration

# Run specific protocol tests
$ go test -v -run TCP ./integration
$ go test -v -run RTU ./integration
$ go test -v -run ASCII ./integration
```

## How It Works

Each test automatically:
1. Starts a Modbus simulator server (RTU, ASCII, or TCP)
2. Creates a client connection
3. Runs comprehensive function code tests
4. Cleans up and stops the simulator

No external dependencies like diagslave or socat are needed!

## Simulator CLI

You can also run the simulator standalone for manual testing:

```bash
# TCP mode
$ go run cmd/simulator/main.go -mode tcp -addr localhost:5020

# RTU mode
$ go run cmd/simulator/main.go -mode rtu -slave-id 17 -baud 19200

# ASCII mode
$ go run cmd/simulator/main.go -mode ascii -slave-id 17 -baud 19200
```

See `cmd/simulator/main.go` for all available options.
