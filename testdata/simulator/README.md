# Simulator Configuration Files

This directory contains example configuration files for the Modbus simulator.

## Configuration Format

Configuration files are JSON-formatted and support the following sections:

### Register Initial Values

Define initial values for the four Modbus register types:

```json
{
  "NamedHoldingRegs": {
    "100": {
      "name": "BATTERY_VOLTAGE",
      "value": 245
    }
  },
  "NamedInputRegs": {
    "0": {
      "name": "TEMPERATURE",
      "value": 250
    }
  },
  "NamedCoils": {
    "0": {
      "name": "ENABLE_CHARGING",
      "value": true
    }
  },
  "NamedDiscreteInputs": {
    "0": {
      "name": "FAULT_STATUS",
      "value": false
    }
  }
}
```

### Delay and Timeout Simulation

The `delays` section allows you to simulate network delays and timeouts for testing fault tolerance:

```json
{
  "delays": {
    "global": {
      "holdingRegs": {
        "delay": "50ms",
        "jitter": 10
      }
    },
    "holdingRegs": {
      "100": {
        "delay": "500ms",
        "jitter": 20,
        "timeoutProbability": 0.3
      }
    }
  }
}
```

#### Delay Configuration Fields

- **`delay`** (string): Base delay duration before responding (e.g., "100ms", "1s", "500ms")
  - Uses Go's time.Duration format
  - Examples: "50ms", "1s", "2.5s"

- **`jitter`** (integer, 0-100): Percentage of random variance to add to the delay
  - `0` = no jitter (fixed delay)
  - `10` = ±10% random variance
  - `50` = ±50% random variance
  - Example: With `delay: "100ms"` and `jitter: 20`, actual delay will be between 80ms and 120ms

- **`timeoutProbability`** (float, 0.0-1.0): Probability of not responding at all (**TCP mode only**)
  - `0.0` = never timeout (always respond)
  - `0.3` = 30% chance of timeout
  - `1.0` = always timeout (never respond)
  - When a timeout occurs, no response is sent and the client will timeout waiting
  - **Note**: Timeout simulation only works in TCP mode. RTU and ASCII modes ignore this setting because the underlying pseudo-terminal (PTY) infrastructure used for testing doesn't support timeout behavior.

#### Delay Configuration Hierarchy

The simulator supports both global defaults and per-address overrides:

1. **Global defaults** - Applied to all registers of a given type unless overridden:
   ```json
   "delays": {
     "global": {
       "holdingRegs": {"delay": "50ms", "jitter": 10},
       "inputRegs": {"delay": "100ms"},
       "coils": {"delay": "25ms"},
       "discreteInputs": {"delay": "25ms"}
     }
   }
   ```

2. **Per-address overrides** - Override global defaults for specific addresses:
   ```json
   "delays": {
     "holdingRegs": {
       "100": {"delay": "500ms", "jitter": 20},
       "200": {"timeoutProbability": 0.5}
     },
     "inputRegs": {
       "0": {"delay": "2s"}
     }
   }
   ```

The simulator looks up delays in this order:
1. Check for address-specific override for the register type
2. Fall back to global default for the register type
3. If neither exists, no delay is applied

## Example Configurations

### solar-charger.json

A realistic solar charge controller configuration with typical register mappings for voltage, current, and power readings.

### delays-example.json

Demonstrates various delay and timeout scenarios:
- Global 50ms delay for all holding registers (±10% jitter)
- Specific registers with different delay characteristics:
  - Register 100: Fast response (no delay override)
  - Register 200: Slow response (500ms ±20% jitter)
  - Register 300: Flaky connection (30% timeout probability)
  - Register 500: Variable latency (200ms ±50% jitter)
  - Coil 0: Very slow (1s ±10% jitter)

## Usage

Run the simulator with a configuration file:

```bash
# TCP mode with delays
./bin/modbus-simulator -mode tcp -addr localhost:5020 -config testdata/simulator/delays-example.json

# RTU mode with solar charger config
./bin/modbus-simulator -mode rtu -slave-id 1 -config testdata/simulator/solar-charger.json

# ASCII mode
./bin/modbus-simulator -mode ascii -slave-id 1 -config testdata/simulator/delays-example.json
```

## Testing Fault Tolerance

The delay and timeout features are useful for testing:
- **Client timeout handling**: Use `timeoutProbability` to verify clients handle timeouts gracefully
- **Variable latency**: Use `jitter` to test behavior under network jitter
- **Slow devices**: Use `delay` to simulate slow-responding devices
- **Network conditions**: Combine delays and timeouts to simulate poor network quality
