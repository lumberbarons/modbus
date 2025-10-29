// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package modbus

import (
	"context"
	"encoding/binary"
	"testing"
)

// mockPackager is a test implementation of Packager interface
type mockPackager struct {
	encodeFunc func(*ProtocolDataUnit) ([]byte, error)
	decodeFunc func([]byte) (*ProtocolDataUnit, error)
	verifyFunc func([]byte, []byte) error
}

func (m *mockPackager) Encode(pdu *ProtocolDataUnit) ([]byte, error) {
	if m.encodeFunc != nil {
		return m.encodeFunc(pdu)
	}
	// Default: just wrap PDU in a simple frame
	adu := make([]byte, len(pdu.Data)+1)
	adu[0] = pdu.FunctionCode
	copy(adu[1:], pdu.Data)
	return adu, nil
}

func (m *mockPackager) Decode(adu []byte) (*ProtocolDataUnit, error) {
	if m.decodeFunc != nil {
		return m.decodeFunc(adu)
	}
	// Default: unwrap frame to PDU
	if len(adu) < 1 {
		return nil, ErrShortFrame
	}
	return &ProtocolDataUnit{
		FunctionCode: adu[0],
		Data:         adu[1:],
	}, nil
}

func (m *mockPackager) Verify(aduRequest, aduResponse []byte) error {
	if m.verifyFunc != nil {
		return m.verifyFunc(aduRequest, aduResponse)
	}
	return nil
}

// mockTransporter is a test implementation of Transporter interface
type mockTransporter struct {
	sendFunc func(context.Context, []byte) ([]byte, error)
}

func (m *mockTransporter) Send(ctx context.Context, aduRequest []byte) ([]byte, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, aduRequest)
	}
	return aduRequest, nil
}

// TestReadCoils tests the ReadCoils function
func TestReadCoils(t *testing.T) {
	tests := []struct {
		name      string
		address   uint16
		quantity  uint16
		response  []byte
		wantErr   bool
		wantData  []byte
		errType   error
	}{
		{
			name:     "valid read 8 coils",
			address:  0,
			quantity: 8,
			response: []byte{0x01, 0x01, 0xCD}, // FC, byte count, data
			wantData: []byte{0xCD},
		},
		{
			name:     "valid read 19 coils",
			address:  100,
			quantity: 19,
			response: []byte{0x01, 0x03, 0xCD, 0x6B, 0x05}, // FC, byte count, 3 bytes data
			wantData: []byte{0xCD, 0x6B, 0x05},
		},
		{
			name:     "quantity too small",
			address:  0,
			quantity: 0,
			wantErr:  true,
			errType:  ErrInvalidQuantity,
		},
		{
			name:     "quantity too large",
			address:  0,
			quantity: 2001,
			wantErr:  true,
			errType:  ErrInvalidQuantity,
		},
		{
			name:     "quantity minimum valid",
			address:  0,
			quantity: 1,
			response: []byte{0x01, 0x01, 0x01},
			wantData: []byte{0x01},
		},
		{
			name:     "quantity maximum valid",
			address:  0,
			quantity: 2000,
			response: func() []byte {
				resp := make([]byte, 252)
				resp[0] = 0x01
				resp[1] = 250
				return resp
			}(),
			wantData: func() []byte {
				return make([]byte, 250)
			}(),
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

			result, err := client.ReadCoils(context.Background(), tt.address, tt.quantity)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.wantData) {
				t.Errorf("result length = %d, want %d", len(result), len(tt.wantData))
			}
		})
	}
}

// TestReadDiscreteInputs tests the ReadDiscreteInputs function
func TestReadDiscreteInputs(t *testing.T) {
	tests := []struct {
		name     string
		quantity uint16
		wantErr  bool
		errType  error
	}{
		{
			name:     "quantity too small",
			quantity: 0,
			wantErr:  true,
			errType:  ErrInvalidQuantity,
		},
		{
			name:     "quantity too large",
			quantity: 2001,
			wantErr:  true,
			errType:  ErrInvalidQuantity,
		},
		{
			name:     "quantity minimum valid",
			quantity: 1,
			wantErr:  false,
		},
		{
			name:     "quantity maximum valid",
			quantity: 2000,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					// Return valid response with byte count matching
					byteCount := (tt.quantity + 7) / 8
					resp := make([]byte, byteCount+2)
					resp[0] = 0x02 // function code
					resp[1] = byte(byteCount)
					return resp, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			_, err := client.ReadDiscreteInputs(context.Background(), 0, tt.quantity)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestReadHoldingRegisters tests the ReadHoldingRegisters function
func TestReadHoldingRegisters(t *testing.T) {
	tests := []struct {
		name     string
		address  uint16
		quantity uint16
		wantErr  bool
		errType  error
	}{
		{
			name:     "quantity too small",
			address:  0,
			quantity: 0,
			wantErr:  true,
			errType:  ErrInvalidQuantity,
		},
		{
			name:     "quantity too large",
			address:  0,
			quantity: 126,
			wantErr:  true,
			errType:  ErrInvalidQuantity,
		},
		{
			name:     "quantity minimum valid",
			address:  0,
			quantity: 1,
			wantErr:  false,
		},
		{
			name:     "quantity maximum valid",
			address:  0,
			quantity: 125,
			wantErr:  false,
		},
		{
			name:     "valid read",
			address:  100,
			quantity: 3,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					// Return valid response
					byteCount := tt.quantity * 2
					resp := make([]byte, byteCount+2)
					resp[0] = 0x03 // function code
					resp[1] = byte(byteCount)
					return resp, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			_, err := client.ReadHoldingRegisters(context.Background(), tt.address, tt.quantity)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestReadInputRegisters tests the ReadInputRegisters function
func TestReadInputRegisters(t *testing.T) {
	tests := []struct {
		name     string
		quantity uint16
		wantErr  bool
	}{
		{
			name:     "quantity zero",
			quantity: 0,
			wantErr:  true,
		},
		{
			name:     "quantity too large",
			quantity: 126,
			wantErr:  true,
		},
		{
			name:     "quantity valid min",
			quantity: 1,
			wantErr:  false,
		},
		{
			name:     "quantity valid max",
			quantity: 125,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					byteCount := tt.quantity * 2
					resp := make([]byte, byteCount+2)
					resp[0] = 0x04
					resp[1] = byte(byteCount)
					return resp, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			_, err := client.ReadInputRegisters(context.Background(), 0, tt.quantity)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestWriteSingleCoil tests the WriteSingleCoil function
func TestWriteSingleCoil(t *testing.T) {
	tests := []struct {
		name     string
		address  uint16
		value    uint16
		response []byte
		wantErr  bool
		errType  error
	}{
		{
			name:     "valid write ON",
			address:  100,
			value:    0xFF00,
			response: []byte{0x05, 0x00, 0x64, 0xFF, 0x00}, // FC, addr, value
			wantErr:  false,
		},
		{
			name:     "valid write OFF",
			address:  100,
			value:    0x0000,
			response: []byte{0x05, 0x00, 0x64, 0x00, 0x00},
			wantErr:  false,
		},
		{
			name:    "invalid value",
			address: 100,
			value:   0x0001,
			wantErr: true,
			errType: ErrInvalidData,
		},
		{
			name:    "invalid value FF",
			address: 100,
			value:   0x00FF,
			wantErr: true,
			errType: ErrInvalidData,
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

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestWriteSingleRegister tests the WriteSingleRegister function
func TestWriteSingleRegister(t *testing.T) {
	tests := []struct {
		name     string
		address  uint16
		value    uint16
		response []byte
		wantErr  bool
	}{
		{
			name:     "valid write",
			address:  100,
			value:    0x1234,
			response: []byte{0x06, 0x00, 0x64, 0x12, 0x34}, // FC, addr, value
			wantErr:  false,
		},
		{
			name:     "write zero",
			address:  0,
			value:    0,
			response: []byte{0x06, 0x00, 0x00, 0x00, 0x00},
			wantErr:  false,
		},
		{
			name:     "write max value",
			address:  0xFFFF,
			value:    0xFFFF,
			response: []byte{0x06, 0xFF, 0xFF, 0xFF, 0xFF},
			wantErr:  false,
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

			result, err := client.WriteSingleRegister(context.Background(), tt.address, tt.value)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantErr && len(result) != 2 {
				t.Errorf("result length = %d, want 2", len(result))
			}
		})
	}
}

// TestWriteMultipleCoils tests the WriteMultipleCoils function
func TestWriteMultipleCoils(t *testing.T) {
	tests := []struct {
		name     string
		quantity uint16
		value    []byte
		wantErr  bool
	}{
		{
			name:     "quantity too small",
			quantity: 0,
			value:    []byte{0x01},
			wantErr:  true,
		},
		{
			name:     "quantity too large",
			quantity: 1969,
			value:    make([]byte, 246),
			wantErr:  true,
		},
		{
			name:     "quantity valid min",
			quantity: 1,
			value:    []byte{0x01},
			wantErr:  false,
		},
		{
			name:     "quantity valid max",
			quantity: 1968,
			value:    make([]byte, 246),
			wantErr:  false,
		},
		{
			name:     "valid write 10 coils",
			quantity: 10,
			value:    []byte{0xCD, 0x01},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					// Return valid response echoing address and quantity
					resp := make([]byte, 5)
					resp[0] = 0x0F // function code
					binary.BigEndian.PutUint16(resp[1:], 0)
					binary.BigEndian.PutUint16(resp[3:], tt.quantity)
					return resp, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			_, err := client.WriteMultipleCoils(context.Background(), 0, tt.quantity, tt.value)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestWriteMultipleRegisters tests the WriteMultipleRegisters function
func TestWriteMultipleRegisters(t *testing.T) {
	tests := []struct {
		name     string
		quantity uint16
		value    []byte
		wantErr  bool
	}{
		{
			name:     "quantity too small",
			quantity: 0,
			value:    []byte{},
			wantErr:  true,
		},
		{
			name:     "quantity too large",
			quantity: 124,
			value:    make([]byte, 248),
			wantErr:  true,
		},
		{
			name:     "quantity valid min",
			quantity: 1,
			value:    []byte{0x00, 0x0A},
			wantErr:  false,
		},
		{
			name:     "quantity valid max",
			quantity: 123,
			value:    make([]byte, 246),
			wantErr:  false,
		},
		{
			name:     "valid write 2 registers",
			quantity: 2,
			value:    []byte{0x00, 0x0A, 0x01, 0x02},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					resp := make([]byte, 5)
					resp[0] = 0x10
					binary.BigEndian.PutUint16(resp[1:], 0)
					binary.BigEndian.PutUint16(resp[3:], tt.quantity)
					return resp, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			_, err := client.WriteMultipleRegisters(context.Background(), 0, tt.quantity, tt.value)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestMaskWriteRegister tests the MaskWriteRegister function
func TestMaskWriteRegister(t *testing.T) {
	tests := []struct {
		name     string
		address  uint16
		andMask  uint16
		orMask   uint16
		response []byte
		wantErr  bool
	}{
		{
			name:     "valid mask write",
			address:  100,
			andMask:  0xF2F2,
			orMask:   0x2525,
			response: []byte{0x16, 0x00, 0x64, 0xF2, 0xF2, 0x25, 0x25},
			wantErr:  false,
		},
		{
			name:     "all zeros",
			address:  0,
			andMask:  0,
			orMask:   0,
			response: []byte{0x16, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			wantErr:  false,
		},
		{
			name:     "all ones",
			address:  0xFFFF,
			andMask:  0xFFFF,
			orMask:   0xFFFF,
			response: []byte{0x16, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			wantErr:  false,
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

			result, err := client.MaskWriteRegister(context.Background(), tt.address, tt.andMask, tt.orMask)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantErr && len(result) != 4 {
				t.Errorf("result length = %d, want 4", len(result))
			}
		})
	}
}

// TestReadWriteMultipleRegisters tests the ReadWriteMultipleRegisters function
func TestReadWriteMultipleRegisters(t *testing.T) {
	tests := []struct {
		name          string
		readQuantity  uint16
		writeQuantity uint16
		value         []byte
		wantErr       bool
		errType       error
	}{
		{
			name:          "read quantity too small",
			readQuantity:  0,
			writeQuantity: 1,
			value:         []byte{0x00, 0x0A},
			wantErr:       true,
			errType:       ErrInvalidQuantity,
		},
		{
			name:          "read quantity too large",
			readQuantity:  126,
			writeQuantity: 1,
			value:         []byte{0x00, 0x0A},
			wantErr:       true,
			errType:       ErrInvalidQuantity,
		},
		{
			name:          "write quantity too small",
			readQuantity:  1,
			writeQuantity: 0,
			value:         []byte{},
			wantErr:       true,
			errType:       ErrInvalidQuantity,
		},
		{
			name:          "write quantity too large",
			readQuantity:  1,
			writeQuantity: 122,
			value:         make([]byte, 244),
			wantErr:       true,
			errType:       ErrInvalidQuantity,
		},
		{
			name:          "valid operation min",
			readQuantity:  1,
			writeQuantity: 1,
			value:         []byte{0x00, 0x0A},
			wantErr:       false,
		},
		{
			name:          "valid operation max",
			readQuantity:  125,
			writeQuantity: 121,
			value:         make([]byte, 242),
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &mockTransporter{
				sendFunc: func(ctx context.Context, req []byte) ([]byte, error) {
					byteCount := tt.readQuantity * 2
					resp := make([]byte, byteCount+2)
					resp[0] = 0x17
					resp[1] = byte(byteCount)
					return resp, nil
				},
			}
			mockP := &mockPackager{}
			client := NewClient2(mockP, mockT)

			_, err := client.ReadWriteMultipleRegisters(context.Background(), 0, tt.readQuantity, 100, tt.writeQuantity, tt.value)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestReadFIFOQueue tests the ReadFIFOQueue function
func TestReadFIFOQueue(t *testing.T) {
	tests := []struct {
		name     string
		address  uint16
		response []byte
		wantErr  bool
		wantLen  int
	}{
		{
			name:     "valid FIFO read",
			address:  100,
			// Response.Data includes: byte count (2) + FIFO count (2) + data (4) = 8 bytes total
			// Byte count field value should be len(response.Data) - 1 = 7
			response: []byte{0x18, 0x00, 0x07, 0x00, 0x02, 0x01, 0x02, 0x03, 0x04},
			wantErr:  false,
			wantLen:  4,
		},
		{
			name:     "empty FIFO",
			address:  100,
			// Response.Data includes: byte count (2) + FIFO count (2) = 4 bytes total
			// Byte count field value should be 3
			response: []byte{0x18, 0x00, 0x03, 0x00, 0x00},
			wantErr:  false,
			wantLen:  0,
		},
		{
			name:     "FIFO count max valid",
			address:  100,
			response: func() []byte {
				// Response.Data = byte count (2) + FIFO count (2) + data (62) = 66 bytes
				// Byte count field value should be 65
				resp := make([]byte, 67) // 1 FC + 66 data bytes
				resp[0] = 0x18
				binary.BigEndian.PutUint16(resp[1:], 65) // byte count = 65
				binary.BigEndian.PutUint16(resp[3:], 31) // FIFO count = 31
				return resp
			}(),
			wantErr: false,
			wantLen: 62,
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

			result, err := client.ReadFIFOQueue(context.Background(), tt.address)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantErr && len(result) != tt.wantLen {
				t.Errorf("result length = %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}

// TestDataBlock tests the dataBlock helper function
func TestDataBlock(t *testing.T) {
	tests := []struct {
		name   string
		values []uint16
		want   []byte
	}{
		{
			name:   "single value",
			values: []uint16{0x1234},
			want:   []byte{0x12, 0x34},
		},
		{
			name:   "multiple values",
			values: []uint16{0x1234, 0x5678, 0xABCD},
			want:   []byte{0x12, 0x34, 0x56, 0x78, 0xAB, 0xCD},
		},
		{
			name:   "zero value",
			values: []uint16{0x0000},
			want:   []byte{0x00, 0x00},
		},
		{
			name:   "max value",
			values: []uint16{0xFFFF},
			want:   []byte{0xFF, 0xFF},
		},
		{
			name:   "empty",
			values: []uint16{},
			want:   []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dataBlock(tt.values...)
			if len(got) != len(tt.want) {
				t.Errorf("dataBlock() length = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("dataBlock()[%d] = 0x%02X, want 0x%02X", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestDataBlockSuffix tests the dataBlockSuffix helper function
func TestDataBlockSuffix(t *testing.T) {
	tests := []struct {
		name   string
		suffix []byte
		values []uint16
		want   []byte
	}{
		{
			name:   "single value with suffix",
			suffix: []byte{0xAA, 0xBB},
			values: []uint16{0x1234},
			want:   []byte{0x12, 0x34, 0x02, 0xAA, 0xBB}, // value, length byte, suffix
		},
		{
			name:   "multiple values with suffix",
			suffix: []byte{0xAA, 0xBB, 0xCC},
			values: []uint16{0x1234, 0x5678},
			want:   []byte{0x12, 0x34, 0x56, 0x78, 0x03, 0xAA, 0xBB, 0xCC},
		},
		{
			name:   "empty suffix",
			suffix: []byte{},
			values: []uint16{0x1234},
			want:   []byte{0x12, 0x34, 0x00},
		},
		{
			name:   "no values with suffix",
			suffix: []byte{0xAA},
			values: []uint16{},
			want:   []byte{0x01, 0xAA},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dataBlockSuffix(tt.suffix, tt.values...)
			if len(got) != len(tt.want) {
				t.Errorf("dataBlockSuffix() length = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("dataBlockSuffix()[%d] = 0x%02X, want 0x%02X", i, got[i], tt.want[i])
				}
			}
		})
	}
}
