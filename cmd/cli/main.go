package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/lumberbarons/modbus"
)

func main() {
	app := &cli.App{
		Name:  "modbus-cli",
		Usage: "Command-line tool for Modbus communication",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "protocol",
				Aliases:  []string{"p"},
				Usage:    "Protocol type: tcp, rtu, or ascii",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "address",
				Aliases:  []string{"a"},
				Usage:    "Connection address (TCP: host:port, RTU/ASCII: /dev/ttyUSB0)",
				Required: true,
			},
			&cli.IntFlag{
				Name:    "slave-id",
				Aliases: []string{"s"},
				Usage:   "Modbus slave/unit ID",
				Value:   1,
			},
			&cli.DurationFlag{
				Name:    "timeout",
				Aliases: []string{"t"},
				Usage:   "Timeout duration",
				Value:   5 * time.Second,
			},
			// Serial-specific options
			&cli.IntFlag{
				Name:  "baud",
				Usage: "Baud rate (RTU/ASCII only)",
				Value: 115200,
			},
			&cli.IntFlag{
				Name:  "data-bits",
				Usage: "Data bits (RTU/ASCII only)",
				Value: 8,
			},
			&cli.IntFlag{
				Name:  "stop-bits",
				Usage: "Stop bits (RTU/ASCII only)",
				Value: 1,
			},
			&cli.StringFlag{
				Name:  "parity",
				Usage: "Parity: none, odd, even (RTU/ASCII only)",
				Value: "none",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "read-coils",
				Usage: "Read coils (function code 1)",
				Flags: []cli.Flag{
					&cli.UintFlag{
						Name:     "start",
						Usage:    "Starting address",
						Required: true,
					},
					&cli.UintFlag{
						Name:     "count",
						Usage:    "Number of coils to read (1-2000)",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "format",
						Usage: "Output format: binary, decimal",
						Value: "binary",
					},
				},
				Action: readCoilsAction,
			},
			{
				Name:  "read-discrete-inputs",
				Usage: "Read discrete inputs (function code 2)",
				Flags: []cli.Flag{
					&cli.UintFlag{
						Name:     "start",
						Usage:    "Starting address",
						Required: true,
					},
					&cli.UintFlag{
						Name:     "count",
						Usage:    "Number of discrete inputs to read (1-2000)",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "format",
						Usage: "Output format: binary, decimal",
						Value: "binary",
					},
				},
				Action: readDiscreteInputsAction,
			},
			{
				Name:  "read-holding-registers",
				Usage: "Read holding registers (function code 3)",
				Flags: []cli.Flag{
					&cli.UintFlag{
						Name:     "start",
						Usage:    "Starting address",
						Required: true,
					},
					&cli.UintFlag{
						Name:     "count",
						Usage:    "Number of registers to read (1-125)",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "format",
						Usage: "Output format: hex, decimal",
						Value: "hex",
					},
				},
				Action: readHoldingRegistersAction,
			},
			{
				Name:  "read-input-registers",
				Usage: "Read input registers (function code 4)",
				Flags: []cli.Flag{
					&cli.UintFlag{
						Name:     "start",
						Usage:    "Starting address",
						Required: true,
					},
					&cli.UintFlag{
						Name:     "count",
						Usage:    "Number of registers to read (1-125)",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "format",
						Usage: "Output format: hex, decimal",
						Value: "hex",
					},
				},
				Action: readInputRegistersAction,
			},
			{
				Name:  "read-fifo",
				Usage: "Read FIFO queue (function code 24)",
				Flags: []cli.Flag{
					&cli.UintFlag{
						Name:     "address",
						Usage:    "FIFO pointer address",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "format",
						Usage: "Output format: hex, decimal",
						Value: "hex",
					},
				},
				Action: readFIFOAction,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// createClient creates a Modbus client based on the global flags
func createClient(c *cli.Context) (modbus.Client, error) {
	protocol := c.String("protocol")
	address := c.String("address")
	slaveID := byte(c.Int("slave-id"))
	timeout := c.Duration("timeout")

	switch protocol {
	case "tcp":
		handler := modbus.NewTCPClientHandler(address)
		handler.Timeout = timeout
		handler.SlaveID = slaveID
		return modbus.NewClient(handler), nil

	case "rtu":
		handler := modbus.NewRTUClientHandler(address)
		handler.BaudRate = c.Int("baud")
		handler.DataBits = c.Int("data-bits")
		handler.StopBits = parseStopBits(c.Int("stop-bits"))
		handler.Parity = parseParity(c.String("parity"))
		handler.Timeout = timeout
		handler.SlaveID = slaveID
		return modbus.NewClient(handler), nil

	case "ascii":
		handler := modbus.NewASCIIClientHandler(address)
		handler.BaudRate = c.Int("baud")
		handler.DataBits = c.Int("data-bits")
		handler.StopBits = parseStopBits(c.Int("stop-bits"))
		handler.Parity = parseParity(c.String("parity"))
		handler.Timeout = timeout
		handler.SlaveID = slaveID
		return modbus.NewClient(handler), nil

	default:
		return nil, fmt.Errorf("unsupported protocol: %s (must be tcp, rtu, or ascii)", protocol)
	}
}

func parseStopBits(bits int) modbus.StopBits {
	switch bits {
	case 1:
		return modbus.OneStopBit
	case 2:
		return modbus.TwoStopBits
	default:
		return modbus.OneStopBit
	}
}

func parseParity(parity string) modbus.Parity {
	switch parity {
	case "none":
		return modbus.NoParity
	case "odd":
		return modbus.OddParity
	case "even":
		return modbus.EvenParity
	default:
		return modbus.EvenParity
	}
}

// createContextWithSignalHandler creates a context that is cancelled on SIGINT/SIGTERM
func createContextWithSignalHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Received interrupt signal, cancelling operation...")
		cancel()
	}()

	return ctx, cancel
}

// readCoilsAction handles the read-coils command
func readCoilsAction(c *cli.Context) error {
	client, err := createClient(c)
	if err != nil {
		return err
	}

	ctx, cancel := createContextWithSignalHandler()
	defer cancel()

	start := uint16(c.Uint("start"))
	count := uint16(c.Uint("count"))
	format := c.String("format")

	if count < 1 || count > 2000 {
		return fmt.Errorf("count must be between 1 and 2000")
	}

	results, err := client.ReadCoils(ctx, start, count)
	if err != nil {
		return fmt.Errorf("failed to read coils: %w", err)
	}

	printBitResults(start, count, results, format)
	return nil
}

// readDiscreteInputsAction handles the read-discrete-inputs command
func readDiscreteInputsAction(c *cli.Context) error {
	client, err := createClient(c)
	if err != nil {
		return err
	}

	ctx, cancel := createContextWithSignalHandler()
	defer cancel()

	start := uint16(c.Uint("start"))
	count := uint16(c.Uint("count"))
	format := c.String("format")

	if count < 1 || count > 2000 {
		return fmt.Errorf("count must be between 1 and 2000")
	}

	results, err := client.ReadDiscreteInputs(ctx, start, count)
	if err != nil {
		return fmt.Errorf("failed to read discrete inputs: %w", err)
	}

	printBitResults(start, count, results, format)
	return nil
}

// readHoldingRegistersAction handles the read-holding-registers command
func readHoldingRegistersAction(c *cli.Context) error {
	client, err := createClient(c)
	if err != nil {
		return err
	}

	ctx, cancel := createContextWithSignalHandler()
	defer cancel()

	start := uint16(c.Uint("start"))
	count := uint16(c.Uint("count"))
	format := c.String("format")

	if count < 1 || count > 125 {
		return fmt.Errorf("count must be between 1 and 125")
	}

	results, err := client.ReadHoldingRegisters(ctx, start, count)
	if err != nil {
		return fmt.Errorf("failed to read holding registers: %w", err)
	}

	printRegisterResults(start, count, results, format)
	return nil
}

// readInputRegistersAction handles the read-input-registers command
func readInputRegistersAction(c *cli.Context) error {
	client, err := createClient(c)
	if err != nil {
		return err
	}

	ctx, cancel := createContextWithSignalHandler()
	defer cancel()

	start := uint16(c.Uint("start"))
	count := uint16(c.Uint("count"))
	format := c.String("format")

	if count < 1 || count > 125 {
		return fmt.Errorf("count must be between 1 and 125")
	}

	results, err := client.ReadInputRegisters(ctx, start, count)
	if err != nil {
		return fmt.Errorf("failed to read input registers: %w", err)
	}

	printRegisterResults(start, count, results, format)
	return nil
}

// readFIFOAction handles the read-fifo command
func readFIFOAction(c *cli.Context) error {
	client, err := createClient(c)
	if err != nil {
		return err
	}

	ctx, cancel := createContextWithSignalHandler()
	defer cancel()

	address := uint16(c.Uint("address"))
	format := c.String("format")

	results, err := client.ReadFIFOQueue(ctx, address)
	if err != nil {
		return fmt.Errorf("failed to read FIFO queue: %w", err)
	}

	// FIFO response format: first 2 bytes are count, then the register values
	if len(results) < 2 {
		return fmt.Errorf("invalid FIFO response: too short")
	}

	count := binary.BigEndian.Uint16(results[0:2])
	fmt.Printf("FIFO Count: %d\n", count)

	if count > 0 {
		printRegisterResults(0, count, results[2:], format)
	}

	return nil
}

// printBitResults prints bit values (coils/discrete inputs)
func printBitResults(start, count uint16, data []byte, format string) {
	for i := uint16(0); i < count; i++ {
		byteIndex := i / 8
		bitIndex := i % 8

		if int(byteIndex) >= len(data) {
			break
		}

		bitValue := (data[byteIndex] >> bitIndex) & 0x01

		switch format {
		case "decimal":
			fmt.Printf("0x%04X: %d\n", start+i, bitValue)
		default: // binary
			fmt.Printf("0x%04X: %d\n", start+i, bitValue)
		}
	}
}

// printRegisterResults prints register values
func printRegisterResults(start, count uint16, data []byte, format string) {
	for i := uint16(0); i < count; i++ {
		offset := i * 2
		if int(offset+1) >= len(data) {
			break
		}

		value := binary.BigEndian.Uint16(data[offset : offset+2])

		switch format {
		case "decimal":
			fmt.Printf("0x%04X: %d\n", start+i, value)
		default: // hex
			fmt.Printf("0x%04X: 0x%04X\n", start+i, value)
		}
	}
}
