// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package modbus

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"
)

// TestReadCoilsInvalidResponse tests error handling for invalid responses
func TestReadCoilsInvalidResponse(t *testing.T) {
	tests := []struct {
		name     string
		response []byte
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "byte count mismatch - too small",
			response: []byte{0x01, 0x03, 0xCD, 0x6B}, // says 3 bytes but only 2 present
			wantErr:  true,
			errMsg:   "does not match count",
		},
		{
			name:     "byte count mismatch - too large",
			response: []byte{0x01, 0x01, 0xCD, 0x6B}, // says 1 byte but 2 present
			wantErr:  true,
			errMsg:   "does not match count",
		},
		{
			name:     "empty response data",
			response: []byte{0x01}, // only function code
			wantErr:  true,
			errMsg:   "response data is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					return tt.response, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			_, err := client.ReadCoils(context.Background(), 0, 10)

			if !tt.wantErr {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected error containing '%s' but got nil", tt.errMsg)
				return
			}
		})
	}
}

// TestWriteSingleCoilInvalidResponse tests response validation errors
func TestWriteSingleCoilInvalidResponse(t *testing.T) {
	tests := []struct {
		name     string
		address  uint16
		value    uint16
		response []byte
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "response too short",
			address:  100,
			value:    0xFF00,
			response: []byte{0x05, 0x00, 0x64}, // missing value bytes
			wantErr:  true,
			errMsg:   "response data size",
		},
		{
			name:     "address mismatch",
			address:  100,
			value:    0xFF00,
			response: []byte{0x05, 0x00, 0x65, 0xFF, 0x00}, // address is 101 not 100
			wantErr:  true,
			errMsg:   "response address",
		},
		{
			name:     "value mismatch",
			address:  100,
			value:    0xFF00,
			response: []byte{0x05, 0x00, 0x64, 0x00, 0x00}, // value is 0x0000 not 0xFF00
			wantErr:  true,
			errMsg:   "response value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					return tt.response, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			_, err := client.WriteSingleCoil(context.Background(), tt.address, tt.value)

			if !tt.wantErr {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected error containing '%s' but got nil", tt.errMsg)
			}
		})
	}
}

// TestWriteSingleRegisterInvalidResponse tests response validation errors
func TestWriteSingleRegisterInvalidResponse(t *testing.T) {
	tests := []struct {
		name     string
		address  uint16
		value    uint16
		response []byte
		wantErr  bool
	}{
		{
			name:     "response too short",
			address:  100,
			value:    0x1234,
			response: []byte{0x06, 0x00, 0x64}, // missing value bytes
			wantErr:  true,
		},
		{
			name:     "address mismatch",
			address:  100,
			value:    0x1234,
			response: []byte{0x06, 0x00, 0x65, 0x12, 0x34}, // wrong address
			wantErr:  true,
		},
		{
			name:     "value mismatch",
			address:  100,
			value:    0x1234,
			response: []byte{0x06, 0x00, 0x64, 0x12, 0x35}, // wrong value
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					return tt.response, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			_, err := client.WriteSingleRegister(context.Background(), tt.address, tt.value)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestWriteMultipleCoilsInvalidResponse tests response validation
func TestWriteMultipleCoilsInvalidResponse(t *testing.T) {
	tests := []struct {
		name     string
		address  uint16
		quantity uint16
		response []byte
		wantErr  bool
	}{
		{
			name:     "response too short",
			address:  0,
			quantity: 10,
			response: []byte{0x0F, 0x00, 0x00}, // missing quantity bytes
			wantErr:  true,
		},
		{
			name:     "address mismatch",
			address:  100,
			quantity: 10,
			response: []byte{0x0F, 0x00, 0x65, 0x00, 0x0A}, // wrong address
			wantErr:  true,
		},
		{
			name:     "quantity mismatch",
			address:  100,
			quantity: 10,
			response: []byte{0x0F, 0x00, 0x64, 0x00, 0x0B}, // wrong quantity
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					return tt.response, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			_, err := client.WriteMultipleCoils(context.Background(), tt.address, tt.quantity, []byte{0xCD, 0x01})

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestWriteMultipleRegistersInvalidResponse tests response validation
func TestWriteMultipleRegistersInvalidResponse(t *testing.T) {
	tests := []struct {
		name     string
		address  uint16
		quantity uint16
		response []byte
		wantErr  bool
	}{
		{
			name:     "response too short",
			address:  0,
			quantity: 2,
			response: []byte{0x10, 0x00, 0x00}, // missing quantity bytes
			wantErr:  true,
		},
		{
			name:     "address mismatch",
			address:  100,
			quantity: 2,
			response: []byte{0x10, 0x00, 0x65, 0x00, 0x02}, // wrong address
			wantErr:  true,
		},
		{
			name:     "quantity mismatch",
			address:  100,
			quantity: 2,
			response: []byte{0x10, 0x00, 0x64, 0x00, 0x03}, // wrong quantity
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					return tt.response, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			_, err := client.WriteMultipleRegisters(context.Background(), tt.address, tt.quantity, []byte{0x00, 0x0A, 0x01, 0x02})

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestMaskWriteRegisterInvalidResponse tests response validation
func TestMaskWriteRegisterInvalidResponse(t *testing.T) {
	tests := []struct {
		name     string
		address  uint16
		andMask  uint16
		orMask   uint16
		response []byte
		wantErr  bool
	}{
		{
			name:     "response too short",
			address:  100,
			andMask:  0xF2F2,
			orMask:   0x2525,
			response: []byte{0x16, 0x00, 0x64, 0xF2, 0xF2}, // missing OR mask
			wantErr:  true,
		},
		{
			name:     "address mismatch",
			address:  100,
			andMask:  0xF2F2,
			orMask:   0x2525,
			response: []byte{0x16, 0x00, 0x65, 0xF2, 0xF2, 0x25, 0x25}, // wrong address
			wantErr:  true,
		},
		{
			name:     "AND mask mismatch",
			address:  100,
			andMask:  0xF2F2,
			orMask:   0x2525,
			response: []byte{0x16, 0x00, 0x64, 0xF2, 0xF3, 0x25, 0x25}, // wrong AND mask
			wantErr:  true,
		},
		{
			name:     "OR mask mismatch",
			address:  100,
			andMask:  0xF2F2,
			orMask:   0x2525,
			response: []byte{0x16, 0x00, 0x64, 0xF2, 0xF2, 0x25, 0x26}, // wrong OR mask
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					return tt.response, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			_, err := client.MaskWriteRegister(context.Background(), tt.address, tt.andMask, tt.orMask)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestReadWriteMultipleRegistersInvalidResponse tests response validation
func TestReadWriteMultipleRegistersInvalidResponse(t *testing.T) {
	tests := []struct {
		name     string
		response []byte
		wantErr  bool
	}{
		{
			name:     "byte count mismatch",
			response: []byte{0x17, 0x04, 0x00, 0x0A, 0x01}, // says 4 bytes but only 2 present
			wantErr:  true,
		},
		{
			name:     "empty data",
			response: []byte{0x17}, // only function code
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					return tt.response, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			_, err := client.ReadWriteMultipleRegisters(context.Background(), 0, 1, 100, 1, []byte{0x00, 0x0A})

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestReadFIFOQueueInvalidResponse tests response validation
func TestReadFIFOQueueInvalidResponse(t *testing.T) {
	tests := []struct {
		name     string
		response []byte
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "response too short",
			response: []byte{0x18, 0x00, 0x06}, // missing FIFO count and data
			wantErr:  true,
			errMsg:   "less than expected",
		},
		{
			name:     "byte count mismatch",
			response: []byte{0x18, 0x00, 0x08, 0x00, 0x02, 0x01, 0x02}, // says 8 but actual is 6
			wantErr:  true,
			errMsg:   "does not match count",
		},
		{
			name: "FIFO count too large",
			response: func() []byte {
				resp := make([]byte, 70)
				resp[0] = 0x18
				binary.BigEndian.PutUint16(resp[1:], 68)
				binary.BigEndian.PutUint16(resp[3:], 32) // FIFO count > 31
				return resp
			}(),
			wantErr: true,
			errMsg:  "greater than expected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					return tt.response, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			_, err := client.ReadFIFOQueue(context.Background(), 100)

			if !tt.wantErr {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected error containing '%s' but got nil", tt.errMsg)
			}
		})
	}
}

// TestModbusError tests the ModbusError type
func TestModbusError(t *testing.T) {
	tests := []struct {
		name          string
		functionCode  byte
		exceptionCode byte
		wantContains  string
	}{
		{
			name:          "illegal function",
			functionCode:  0x01,
			exceptionCode: ExceptionCodeIllegalFunction,
			wantContains:  "illegal function",
		},
		{
			name:          "illegal data address",
			functionCode:  0x03,
			exceptionCode: ExceptionCodeIllegalDataAddress,
			wantContains:  "illegal data address",
		},
		{
			name:          "illegal data value",
			functionCode:  0x06,
			exceptionCode: ExceptionCodeIllegalDataValue,
			wantContains:  "illegal data value",
		},
		{
			name:          "server device failure",
			functionCode:  0x10,
			exceptionCode: ExceptionCodeServerDeviceFailure,
			wantContains:  "server device failure",
		},
		{
			name:          "acknowledge",
			functionCode:  0x05,
			exceptionCode: ExceptionCodeAcknowledge,
			wantContains:  "acknowledge",
		},
		{
			name:          "server device busy",
			functionCode:  0x11,
			exceptionCode: ExceptionCodeServerDeviceBusy,
			wantContains:  "server device busy",
		},
		{
			name:          "memory parity error",
			functionCode:  0x08,
			exceptionCode: ExceptionCodeMemoryParityError,
			wantContains:  "memory parity error",
		},
		{
			name:          "gateway path unavailable",
			functionCode:  0x0A,
			exceptionCode: ExceptionCodeGatewayPathUnavailable,
			wantContains:  "gateway path unavailable",
		},
		{
			name:          "gateway target device failed to respond",
			functionCode:  0x0B,
			exceptionCode: ExceptionCodeGatewayTargetDeviceFailedToRespond,
			wantContains:  "gateway target device failed to respond",
		},
		{
			name:          "unknown exception code",
			functionCode:  0x01,
			exceptionCode: 0xFF,
			wantContains:  "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ModbusError{
				FunctionCode:  tt.functionCode,
				ExceptionCode: tt.exceptionCode,
			}

			errMsg := err.Error()
			if errMsg == "" {
				t.Errorf("Error() returned empty string")
			}

			// Check that error message contains expected text
			if tt.wantContains != "" {
				found := false
				if len(errMsg) > 0 {
					// Simple substring search
					for i := 0; i <= len(errMsg)-len(tt.wantContains); i++ {
						if errMsg[i:i+len(tt.wantContains)] == tt.wantContains {
							found = true
							break
						}
					}
				}
				if !found {
					t.Errorf("Error() = %q, want to contain %q", errMsg, tt.wantContains)
				}
			}
		})
	}
}

// TestResponseError tests the responseError helper function
func TestResponseError(t *testing.T) {
	tests := []struct {
		name          string
		response      *ProtocolDataUnit
		wantFuncCode  byte
		wantExcCode   byte
	}{
		{
			name: "exception with data",
			response: &ProtocolDataUnit{
				FunctionCode: 0x81, // 0x80 | 0x01
				Data:         []byte{0x02}, // exception code
			},
			wantFuncCode: 0x81,
			wantExcCode:  0x02,
		},
		{
			name: "exception without data",
			response: &ProtocolDataUnit{
				FunctionCode: 0x83,
				Data:         []byte{},
			},
			wantFuncCode: 0x83,
			wantExcCode:  0x00,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := responseError(tt.response)

			if err == nil {
				t.Fatal("responseError() returned nil")
			}

			modbusErr, ok := err.(*ModbusError)
			if !ok {
				t.Fatalf("responseError() returned %T, want *ModbusError", err)
			}

			if modbusErr.FunctionCode != tt.wantFuncCode {
				t.Errorf("FunctionCode = 0x%02X, want 0x%02X", modbusErr.FunctionCode, tt.wantFuncCode)
			}

			if modbusErr.ExceptionCode != tt.wantExcCode {
				t.Errorf("ExceptionCode = 0x%02X, want 0x%02X", modbusErr.ExceptionCode, tt.wantExcCode)
			}
		})
	}
}

// TestClientExceptionHandling tests that client properly returns ModbusError for exception responses
func TestClientExceptionHandling(t *testing.T) {
	tests := []struct {
		name          string
		requestFunc   byte
		responseFunc  byte
		exceptionCode byte
		wantErr       bool
	}{
		{
			name:          "read coils exception",
			requestFunc:   FuncCodeReadCoils,
			responseFunc:  0x81, // exception response
			exceptionCode: ExceptionCodeIllegalDataAddress,
			wantErr:       true,
		},
		{
			name:          "write single register exception",
			requestFunc:   FuncCodeWriteSingleRegister,
			responseFunc:  0x86, // exception response
			exceptionCode: ExceptionCodeIllegalDataValue,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					// Return exception response
					return []byte{tt.responseFunc, tt.exceptionCode}, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			var err error
			switch tt.requestFunc {
			case FuncCodeReadCoils:
				_, err = client.ReadCoils(context.Background(), 0, 10)
			case FuncCodeWriteSingleRegister:
				_, err = client.WriteSingleRegister(context.Background(), 0, 0x1234)
			}

			if !tt.wantErr {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("expected ModbusError but got nil")
			}

			// Check if it's a ModbusError
			var modbusErr *ModbusError
			if !errors.As(err, &modbusErr) {
				t.Errorf("error is not a ModbusError: %v", err)
			}
		})
	}
}

// TestPackagerErrors tests that packager errors are properly propagated
func TestPackagerErrors(t *testing.T) {
	tests := []struct {
		name        string
		encodeErr   error
		decodeErr   error
		verifyErr   error
		wantErr     bool
	}{
		{
			name:      "encode error",
			encodeErr: fmt.Errorf("encode failed"),
			wantErr:   true,
		},
		{
			name:      "decode error",
			decodeErr: fmt.Errorf("decode failed"),
			wantErr:   true,
		},
		{
			name:      "verify error",
			verifyErr: ErrProtocolError,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					return []byte{0x03, 0x04, 0x00, 0x0A, 0x00, 0x0B}, nil
				},
			}
			mockP := &mockPackager{
				encodeFunc: func(pdu *ProtocolDataUnit) ([]byte, error) {
					if tt.encodeErr != nil {
						return nil, tt.encodeErr
					}
					return []byte{pdu.FunctionCode}, nil
				},
				decodeFunc: func(adu []byte) (*ProtocolDataUnit, error) {
					if tt.decodeErr != nil {
						return nil, tt.decodeErr
					}
					return &ProtocolDataUnit{
						FunctionCode: adu[0],
						Data:         adu[1:],
					}, nil
				},
				verifyFunc: func(req, resp []byte) error {
					return tt.verifyErr
				},
			}
			client := NewClient2(mockP, mockT)

			_, err := client.ReadHoldingRegisters(context.Background(), 0, 1)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestTransporterErrors tests that transporter errors are properly propagated
func TestTransporterErrors(t *testing.T) {
	testErr := fmt.Errorf("transport failed")

	mockT := &mockTransporter{
		sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
			return nil, testErr
		},
	}
	mockP := &mockPackager{}
	client := NewClient2(mockP, mockT)

	_, err := client.ReadCoils(context.Background(), 0, 10)

	if err == nil {
		t.Fatal("expected error but got nil")
	}

	// Error should wrap the transport error
	if !errors.Is(err, testErr) {
		t.Errorf("error chain does not contain transport error")
	}
}

// TestClientContextCancellation tests context cancellation handling
func TestClientContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	mockT := &mockTransporter{
		sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
			return nil, ctx.Err()
		},
	}
	mockP := &mockPackager{}
	client := NewClient2(mockP, mockT)

	_, err := client.ReadCoils(ctx, 0, 10)

	if err == nil {
		t.Fatal("expected error but got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("error should wrap context.Canceled, got: %v", err)
	}
}
