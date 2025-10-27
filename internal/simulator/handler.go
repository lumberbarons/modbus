// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package simulator

import (
	"encoding/binary"

	"github.com/lumberbarons/modbus"
)

// Handler processes Modbus function codes and interacts with the DataStore.
type Handler struct {
	dataStore *DataStore
}

// NewHandler creates a new Handler with the given DataStore.
func NewHandler(ds *DataStore) *Handler {
	return &Handler{dataStore: ds}
}

// HandleRequest processes a Modbus PDU request and returns a response PDU.
func (h *Handler) HandleRequest(req *modbus.ProtocolDataUnit) *modbus.ProtocolDataUnit {
	switch req.FunctionCode {
	case modbus.FuncCodeReadCoils:
		return h.handleReadCoils(req)
	case modbus.FuncCodeReadDiscreteInputs:
		return h.handleReadDiscreteInputs(req)
	case modbus.FuncCodeReadHoldingRegisters:
		return h.handleReadHoldingRegisters(req)
	case modbus.FuncCodeReadInputRegisters:
		return h.handleReadInputRegisters(req)
	case modbus.FuncCodeWriteSingleCoil:
		return h.handleWriteSingleCoil(req)
	case modbus.FuncCodeWriteSingleRegister:
		return h.handleWriteSingleRegister(req)
	case modbus.FuncCodeWriteMultipleCoils:
		return h.handleWriteMultipleCoils(req)
	case modbus.FuncCodeWriteMultipleRegisters:
		return h.handleWriteMultipleRegisters(req)
	case modbus.FuncCodeMaskWriteRegister:
		return h.handleMaskWriteRegister(req)
	case modbus.FuncCodeReadWriteMultipleRegisters:
		return h.handleReadWriteMultipleRegisters(req)
	case modbus.FuncCodeReadFIFOQueue:
		return h.handleReadFIFOQueue(req)
	default:
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalFunction)
	}
}

func (h *Handler) handleReadCoils(req *modbus.ProtocolDataUnit) *modbus.ProtocolDataUnit {
	if len(req.Data) < 4 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	address := binary.BigEndian.Uint16(req.Data[0:2])
	quantity := binary.BigEndian.Uint16(req.Data[2:4])

	if quantity < 1 || quantity > 2000 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	coils, err := h.dataStore.ReadCoils(address, quantity)
	if err != nil {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataAddress)
	}

	return &modbus.ProtocolDataUnit{
		FunctionCode: req.FunctionCode,
		Data:         boolsToBytes(coils),
	}
}

func (h *Handler) handleReadDiscreteInputs(req *modbus.ProtocolDataUnit) *modbus.ProtocolDataUnit {
	if len(req.Data) < 4 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	address := binary.BigEndian.Uint16(req.Data[0:2])
	quantity := binary.BigEndian.Uint16(req.Data[2:4])

	if quantity < 1 || quantity > 2000 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	inputs, err := h.dataStore.ReadDiscreteInputs(address, quantity)
	if err != nil {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataAddress)
	}

	return &modbus.ProtocolDataUnit{
		FunctionCode: req.FunctionCode,
		Data:         boolsToBytes(inputs),
	}
}

func (h *Handler) handleReadHoldingRegisters(req *modbus.ProtocolDataUnit) *modbus.ProtocolDataUnit {
	if len(req.Data) < 4 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	address := binary.BigEndian.Uint16(req.Data[0:2])
	quantity := binary.BigEndian.Uint16(req.Data[2:4])

	if quantity < 1 || quantity > 125 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	registers, err := h.dataStore.ReadHoldingRegisters(address, quantity)
	if err != nil {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataAddress)
	}

	return &modbus.ProtocolDataUnit{
		FunctionCode: req.FunctionCode,
		Data:         registersToBytes(registers),
	}
}

func (h *Handler) handleReadInputRegisters(req *modbus.ProtocolDataUnit) *modbus.ProtocolDataUnit {
	if len(req.Data) < 4 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	address := binary.BigEndian.Uint16(req.Data[0:2])
	quantity := binary.BigEndian.Uint16(req.Data[2:4])

	if quantity < 1 || quantity > 125 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	registers, err := h.dataStore.ReadInputRegisters(address, quantity)
	if err != nil {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataAddress)
	}

	return &modbus.ProtocolDataUnit{
		FunctionCode: req.FunctionCode,
		Data:         registersToBytes(registers),
	}
}

func (h *Handler) handleWriteSingleCoil(req *modbus.ProtocolDataUnit) *modbus.ProtocolDataUnit {
	if len(req.Data) < 4 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	address := binary.BigEndian.Uint16(req.Data[0:2])
	value := binary.BigEndian.Uint16(req.Data[2:4])

	if value != 0x0000 && value != 0xFF00 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	if err := h.dataStore.WriteSingleCoil(address, value == 0xFF00); err != nil {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataAddress)
	}

	// Echo back the request
	return &modbus.ProtocolDataUnit{
		FunctionCode: req.FunctionCode,
		Data:         req.Data,
	}
}

func (h *Handler) handleWriteSingleRegister(req *modbus.ProtocolDataUnit) *modbus.ProtocolDataUnit {
	if len(req.Data) < 4 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	address := binary.BigEndian.Uint16(req.Data[0:2])
	value := binary.BigEndian.Uint16(req.Data[2:4])

	if err := h.dataStore.WriteSingleRegister(address, value); err != nil {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataAddress)
	}

	// Echo back the request
	return &modbus.ProtocolDataUnit{
		FunctionCode: req.FunctionCode,
		Data:         req.Data,
	}
}

func (h *Handler) handleWriteMultipleCoils(req *modbus.ProtocolDataUnit) *modbus.ProtocolDataUnit {
	if len(req.Data) < 5 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	address := binary.BigEndian.Uint16(req.Data[0:2])
	quantity := binary.BigEndian.Uint16(req.Data[2:4])
	byteCount := req.Data[4]

	if quantity < 1 || quantity > 1968 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	expectedByteCount := (quantity + 7) / 8
	if uint16(byteCount) != expectedByteCount || len(req.Data) < int(5+byteCount) {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	coils := bytesToBools(req.Data[5:5+byteCount], quantity)
	if err := h.dataStore.WriteMultipleCoils(address, coils); err != nil {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataAddress)
	}

	// Response contains address and quantity
	response := make([]byte, 4)
	binary.BigEndian.PutUint16(response[0:2], address)
	binary.BigEndian.PutUint16(response[2:4], quantity)

	return &modbus.ProtocolDataUnit{
		FunctionCode: req.FunctionCode,
		Data:         response,
	}
}

func (h *Handler) handleWriteMultipleRegisters(req *modbus.ProtocolDataUnit) *modbus.ProtocolDataUnit {
	if len(req.Data) < 5 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	address := binary.BigEndian.Uint16(req.Data[0:2])
	quantity := binary.BigEndian.Uint16(req.Data[2:4])
	byteCount := req.Data[4]

	if quantity < 1 || quantity > 123 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	if byteCount != byte(quantity*2) || len(req.Data) < int(5+byteCount) {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	registers := bytesToRegisters(req.Data[5 : 5+byteCount])
	if err := h.dataStore.WriteMultipleRegisters(address, registers); err != nil {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataAddress)
	}

	// Response contains address and quantity
	response := make([]byte, 4)
	binary.BigEndian.PutUint16(response[0:2], address)
	binary.BigEndian.PutUint16(response[2:4], quantity)

	return &modbus.ProtocolDataUnit{
		FunctionCode: req.FunctionCode,
		Data:         response,
	}
}

func (h *Handler) handleMaskWriteRegister(req *modbus.ProtocolDataUnit) *modbus.ProtocolDataUnit {
	if len(req.Data) < 6 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	address := binary.BigEndian.Uint16(req.Data[0:2])
	andMask := binary.BigEndian.Uint16(req.Data[2:4])
	orMask := binary.BigEndian.Uint16(req.Data[4:6])

	if err := h.dataStore.MaskWriteRegister(address, andMask, orMask); err != nil {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataAddress)
	}

	// Echo back the request
	return &modbus.ProtocolDataUnit{
		FunctionCode: req.FunctionCode,
		Data:         req.Data,
	}
}

func (h *Handler) handleReadWriteMultipleRegisters(req *modbus.ProtocolDataUnit) *modbus.ProtocolDataUnit {
	if len(req.Data) < 9 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	readAddress := binary.BigEndian.Uint16(req.Data[0:2])
	readQuantity := binary.BigEndian.Uint16(req.Data[2:4])
	writeAddress := binary.BigEndian.Uint16(req.Data[4:6])
	writeQuantity := binary.BigEndian.Uint16(req.Data[6:8])
	writeByteCount := req.Data[8]

	if readQuantity < 1 || readQuantity > 125 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}
	if writeQuantity < 1 || writeQuantity > 121 {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}
	if writeByteCount != byte(writeQuantity*2) || len(req.Data) < int(9+writeByteCount) {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataValue)
	}

	// Write first
	writeRegisters := bytesToRegisters(req.Data[9 : 9+writeByteCount])
	if err := h.dataStore.WriteMultipleRegisters(writeAddress, writeRegisters); err != nil {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataAddress)
	}

	// Then read
	readRegisters, err := h.dataStore.ReadHoldingRegisters(readAddress, readQuantity)
	if err != nil {
		return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalDataAddress)
	}

	return &modbus.ProtocolDataUnit{
		FunctionCode: req.FunctionCode,
		Data:         registersToBytes(readRegisters),
	}
}

func (h *Handler) handleReadFIFOQueue(req *modbus.ProtocolDataUnit) *modbus.ProtocolDataUnit {
	// FIFO queue not implemented - return illegal function exception
	return newExceptionResponse(req.FunctionCode, modbus.ExceptionCodeIllegalFunction)
}

// Helper functions

func newExceptionResponse(functionCode, exceptionCode byte) *modbus.ProtocolDataUnit {
	return &modbus.ProtocolDataUnit{
		FunctionCode: functionCode | 0x80, // Set high bit for exception
		Data:         []byte{exceptionCode},
	}
}

// boolsToBytes converts a slice of bools to Modbus byte format.
// The byte count is prepended, and bits are packed LSB first.
func boolsToBytes(values []bool) []byte {
	byteCount := (len(values) + 7) / 8
	result := make([]byte, 1+byteCount)
	result[0] = byte(byteCount)

	for i, val := range values {
		if val {
			byteIndex := i/8 + 1
			bitIndex := uint(i % 8)
			result[byteIndex] |= 1 << bitIndex
		}
	}
	return result
}

// bytesToBools converts Modbus byte format to a slice of bools.
// Expects packed bits LSB first, extracts quantity bits.
func bytesToBools(data []byte, quantity uint16) []bool {
	result := make([]bool, quantity)
	for i := uint16(0); i < quantity; i++ {
		byteIndex := i / 8
		bitIndex := uint(i % 8)
		result[i] = (data[byteIndex] & (1 << bitIndex)) != 0
	}
	return result
}

// registersToBytes converts a slice of uint16 registers to Modbus byte format.
// The byte count is prepended, and each register is encoded big-endian.
func registersToBytes(registers []uint16) []byte {
	byteCount := len(registers) * 2
	result := make([]byte, 1+byteCount)
	result[0] = byte(byteCount)

	for i, reg := range registers {
		binary.BigEndian.PutUint16(result[1+i*2:], reg)
	}
	return result
}

// bytesToRegisters converts Modbus byte format to a slice of uint16 registers.
// Each pair of bytes is decoded big-endian.
func bytesToRegisters(data []byte) []uint16 {
	count := len(data) / 2
	result := make([]uint16, count)
	for i := 0; i < count; i++ {
		result[i] = binary.BigEndian.Uint16(data[i*2:])
	}
	return result
}
