package vba

import (
	"encoding/binary"
	"fmt"
	"io"
)

// ParseVBAProject parses the _VBA_PROJECT stream according to MS-OVBA section 2.3.4.1.
//
// The stream structure:
//   - Offset 0x0000: Reserved1 (2 bytes, uint16) - MUST be 0x61CC. MUST be ignored.
//   - Offset 0x0002: Version (2 bytes, uint16) - MUST be ignored on read. MUST be 0xFFFF on write.
//   - Offset 0x0004: Reserved2 (1 byte, uint8) - MUST be 0x00. MUST be ignored.
//   - Offset 0x0005: Reserved3 (2 bytes, uint16) - Undefined. MUST be ignored.
//   - Offset 0x0007: PerformanceCache (Blob, variable size) - MUST be ignored on read. MUST NOT be present on write.
//
// IMPORTANT: All fields in the returned VBAProjectStream MUST be ignored per MS-OVBA specification.
// This function parses the structure for informational purposes only.
func ParseVBAProject(stream io.Reader) (*VBAProjectStream, error) {
	data, err := io.ReadAll(stream)
	if err != nil {
		return nil, fmt.Errorf("failed to read _VBA_PROJECT stream: %w", err)
	}

	// Minimum size is 7 bytes (2+2+1+2)
	if len(data) < 7 {
		return nil, fmt.Errorf("%w: stream too short: got %d bytes, expected at least 7", ErrInvalidVBAProjectFormat, len(data))
	}

	vbaProj := &VBAProjectStream{}

	// Read Reserved1 (offset 0x0000, 2 bytes)
	// MUST be 0x61CC. MUST be ignored per MS-OVBA section 2.3.4.1
	vbaProj.Reserved1 = binary.LittleEndian.Uint16(data[0:2])

	// Read Version (offset 0x0002, 2 bytes)
	// MUST be ignored on read per MS-OVBA section 2.3.4.1
	// MUST be 0xFFFF on write
	vbaProj.Version = binary.LittleEndian.Uint16(data[2:4])

	// Read Reserved2 (offset 0x0004, 1 byte)
	// MUST be 0x00. MUST be ignored per MS-OVBA section 2.3.4.1
	vbaProj.Reserved2 = data[4]

	// Read Reserved3 (offset 0x0005, 2 bytes)
	// Undefined. MUST be ignored per MS-OVBA section 2.3.4.1
	vbaProj.Reserved3 = binary.LittleEndian.Uint16(data[5:7])

	// Read PerformanceCache (offset 0x0007, remaining bytes)
	// MUST be ignored on read per MS-OVBA section 2.3.4.1
	// MUST NOT be present on write
	// Length MUST be seven bytes less than the size of _VBA_PROJECT stream
	if len(data) > 7 {
		vbaProj.PerformanceCache = make([]byte, len(data)-7)
		copy(vbaProj.PerformanceCache, data[7:])
	} else {
		vbaProj.PerformanceCache = []byte{}
	}

	// Note: Per MS-OVBA specification, all fields MUST be ignored.
	// Validation below is for informational purposes only and does not affect parsing.
	// We do not return errors for unexpected values since the spec mandates they be ignored.

	return vbaProj, nil
}

// IsValid checks if the VBAProjectStream has expected values.
// Note: Per MS-OVBA section 2.3.4.1, all fields MUST be ignored.
// This validation is for informational purposes only.
func (v *VBAProjectStream) IsValid() bool {
	// Check Reserved1 (MUST be 0x61CC per spec, but MUST be ignored)
	if v.Reserved1 != 0x61CC {
		return false
	}
	// Check Reserved2 (MUST be 0x00 per spec, but MUST be ignored)
	if v.Reserved2 != 0x00 {
		return false
	}
	// Reserved3 and Version may vary and MUST be ignored, so we don't check them strictly
	return true
}

// GetVersionString returns a human-readable version string.
// Note: Per MS-OVBA section 2.3.4.1, the Version field MUST be ignored on read.
// This function is for informational display purposes only.
func (v *VBAProjectStream) GetVersionString() string {
	if v.Version == 0xFFFF {
		return "0xFFFF (Standard VBA Version, MUST be ignored per spec)"
	}
	return fmt.Sprintf("0x%04X (MUST be ignored per spec)", v.Version)
}
