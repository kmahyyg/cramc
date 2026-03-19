package vba

import (
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf16"
)

// ProjectWM represents the parsed PROJECTwm stream
type ProjectWM struct {
	Mappings map[string]string // MBCS -> Unicode
}

// ParseProjectWM parses a PROJECTwm stream from an io.Reader
// MS-OVBA Section 2.3.4.2 specifies the format
// Format: Each NAMEMAP record contains:
//   - ModuleName (MBCS, null-terminated with single 0x00)
//   - ModuleNameUnicode (UTF-16LE, null-terminated with 0x00 0x00)
// The stream ends with a 2-byte terminator 0x00 0x00
func ParseProjectWM(stream io.Reader) (*ProjectWM, error) {
	pwm := &ProjectWM{
		Mappings: make(map[string]string),
	}

	data, err := io.ReadAll(stream)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read stream: %v", ErrInvalidProjectWM, err)
	}

	// Stream must end with 2-byte terminator 0x0000
	// The NameMap array length is 2 bytes less than stream size
	if len(data) < 2 {
		return nil, fmt.Errorf("%w: stream too short (must be at least 2 bytes for terminator)", ErrInvalidProjectWM)
	}

	offset := 0
	for offset < len(data)-1 {
		// Check for final terminator (2-byte 0x0000)
		if offset+1 < len(data) && data[offset] == 0 && data[offset+1] == 0 {
			// This is the final terminator, end of stream
			break
		}

		// Read MBCS name (null-terminated with single 0x00)
		mbcsName, bytesRead, err := readNullTerminatedString(data[offset:], false)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to read MBCS name at offset %d: %v", ErrInvalidProjectWM, offset, err)
		}
		if bytesRead == 0 {
			// Empty string or reached end
			break
		}
		offset += bytesRead

		// Check if we've reached the end
		if offset >= len(data)-1 {
			break
		}

		// Read Unicode name (null-terminated UTF-16LE with 0x00 0x00)
		unicodeName, bytesRead, err := readNullTerminatedUTF16String(data[offset:])
		if err != nil {
			return nil, fmt.Errorf("%w: failed to read Unicode name at offset %d: %v", ErrInvalidProjectWM, offset, err)
		}
		if bytesRead == 0 {
			break // End of stream
		}
		offset += bytesRead

		// Store mapping (allow empty strings as they might be valid)
		pwm.Mappings[mbcsName] = unicodeName
	}

	return pwm, nil
}

// readNullTerminatedString reads a null-terminated string from data
func readNullTerminatedString(data []byte, isUTF16 bool) (string, int, error) {
	if len(data) == 0 {
		return "", 0, nil
	}

	if isUTF16 {
		return readNullTerminatedUTF16String(data)
	}

	// Find null terminator
	for i := 0; i < len(data); i++ {
		if data[i] == 0 {
			return string(data[:i]), i + 1, nil
		}
	}

	// No null terminator found, return entire string
	return string(data), len(data), nil
}

// readNullTerminatedUTF16String reads a null-terminated UTF-16LE string
// Returns the string, number of bytes read (including terminator), and error
func readNullTerminatedUTF16String(data []byte) (string, int, error) {
	if len(data) < 2 {
		return "", 0, fmt.Errorf("insufficient data for UTF-16 string")
	}

	// Find null terminator (two zero bytes)
	// UTF-16LE strings must have even byte length
	var end int = -1
	for i := 0; i <= len(data)-2; i += 2 {
		if data[i] == 0 && data[i+1] == 0 {
			end = i
			break
		}
	}

	if end == -1 {
		// No null terminator found
		// Data must be even length for UTF-16
		if len(data)%2 != 0 {
			return "", 0, fmt.Errorf("UTF-16 string length must be even")
		}
		// Use entire data as the string (shouldn't happen in valid streams)
		end = len(data)
	}

	// Handle empty string (terminator at start)
	if end == 0 {
		return "", 2, nil // Empty string, 2 bytes for terminator
	}

	// Convert UTF-16LE to string
	utf16Bytes := data[:end]
	if len(utf16Bytes)%2 != 0 {
		return "", 0, fmt.Errorf("UTF-16 string data length must be even")
	}

	utf16Values := make([]uint16, len(utf16Bytes)/2)
	for i := 0; i < len(utf16Values); i++ {
		utf16Values[i] = binary.LittleEndian.Uint16(utf16Bytes[i*2:])
	}

	result := string(utf16.Decode(utf16Values))
	bytesRead := end + 2 // Include null terminator (2 bytes)

	return result, bytesRead, nil
}

// GetUnicodeName returns the Unicode name for an MBCS name
func (pwm *ProjectWM) GetUnicodeName(mbcsName string) string {
	if unicodeName, ok := pwm.Mappings[mbcsName]; ok {
		return unicodeName
	}
	return mbcsName // Return original if not found
}

// GetMBCSName returns the MBCS name for a Unicode name
func (pwm *ProjectWM) GetMBCSName(unicodeName string) string {
	for mbcs, unicode := range pwm.Mappings {
		if unicode == unicodeName {
			return mbcs
		}
	}
	return unicodeName // Return original if not found
}
