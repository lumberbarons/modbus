// Copyright 2014 Quoc-Viet Nguyen. All rights reserved.
// This software may be modified and distributed under the terms
// of the BSD license. See the LICENSE file for details.

package simulator

// lrc8 calculates the LRC (Longitudinal Redundancy Check) for Modbus ASCII.
func lrc8(data []byte) byte {
	var sum uint8
	for _, b := range data {
		sum += b
	}
	// Return twos complement
	return uint8(-int8(sum))
}
