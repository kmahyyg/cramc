package vba

import (
	"encoding/binary"
	"fmt"
)

// ParseDirStream parses a dir stream after decompression
// MS-OVBA Section 2.3.4.2
// If outputCallback is provided, it will be called to display parsed records incrementally
func ParseDirStream(compressedData []byte) (*DirStream, error) {
	return ParseDirStreamWithOutput(compressedData, nil)
}

// ParseDirStreamWithOutput parses a dir stream with optional incremental output
func ParseDirStreamWithOutput(compressedData []byte, outputCallback ParseOutputCallback) (*DirStream, error) {
	// First, decompress the data
	decompressed, err := Decompress(compressedData)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecompressionFailed, err)
	}

	if len(decompressed) < 6 {
		return nil, fmt.Errorf("%w: data too short (need at least 6 bytes for first record)", ErrInvalidDirFormat)
	}

	dir := &DirStream{
		References: []DirReference{},
		Modules:    []DirModule{},
	}

	offset := 0

	// Parse PROJECTINFORMATION (series of sub-records)
	if outputCallback != nil {
		outputCallback("=== Parsing PROJECTINFORMATION ===\n")
	}
	offset, err = parseProjectInformation(decompressed, offset, dir, outputCallback)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse PROJECTINFORMATION: %v", ErrInvalidDirFormat, err)
	}

	// Parse PROJECTREFERENCES (array of REFERENCE records, terminated by 0x000F)
	if outputCallback != nil {
		outputCallback("\n=== Parsing PROJECTREFERENCES ===\n")
	}
	offset, err = parseProjectReferences(decompressed, offset, dir, outputCallback)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse PROJECTREFERENCES: %v", ErrInvalidDirFormat, err)
	}

	// Parse PROJECTMODULES (starts with 0x000F)
	if outputCallback != nil {
		outputCallback("\n=== Parsing PROJECTMODULES ===\n")
	}
	offset, err = parseProjectModules(decompressed, offset, dir, outputCallback)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse PROJECTMODULES: %v", ErrInvalidDirFormat, err)
	}

	// Check for Terminator (0x0010)
	if offset+2 <= len(decompressed) {
		terminator := binary.LittleEndian.Uint16(decompressed[offset:])
		if terminator != 0x0010 {
			return nil, fmt.Errorf("%w: expected terminator 0x0010 at offset %d, found 0x%04X", ErrInvalidDirFormat, offset, terminator)
		}
	}
	offset += 2
	// Check last Reserved (4 bytes) 0x00000000
	if offset+4 <= len(decompressed) {
		reservedEnd := binary.LittleEndian.Uint32(decompressed[offset:])
		if reservedEnd != 0x00000000 {
			return nil, fmt.Errorf("%w: expected stream ended with reserved 0x00000000 at offset %d, found 0x%08X", ErrInvalidDirFormat, offset, reservedEnd)
		}
		offset += 4
	} else {
		return nil, fmt.Errorf("%w: unexpected dir stream end reserved field value, EOF", ErrInvalidDirFormat)
	}

	return dir, nil
}

// decodeUTF16LE decodes UTF-16LE bytes to a Go string
func decodeUTF16LE(data []byte) string {
	if len(data)%2 != 0 {
		// Odd length, pad with zero
		data = append(data, 0)
	}
	runes := make([]rune, 0, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		codeUnit := binary.LittleEndian.Uint16(data[i:])
		if codeUnit == 0 {
			break // Null terminator
		}
		runes = append(runes, rune(codeUnit))
	}
	return string(runes)
}

// parseProjectModules parses PROJECTMODULES record
// MS-OVBA Section 2.3.4.2.3
func parseProjectModules(data []byte, offset int, dir *DirStream, outputCallback ParseOutputCallback) (int, error) {
	// Read Id (2 bytes) - must be 0x000F
	if offset+2 > len(data) {
		return offset, fmt.Errorf("insufficient data for PROJECTMODULES Id")
	}
	id := binary.LittleEndian.Uint16(data[offset:])
	if id != 0x000F {
		return offset, fmt.Errorf("invalid PROJECTMODULES Id: 0x%04X (expected 0x000F)", id)
	}
	offset += 2

	// Read Size Of Count (4 bytes) - must be 0x00000002
	if offset+4 > len(data) {
		return offset, fmt.Errorf("insufficient data for PROJECTMODULES Size")
	}
	size := binary.LittleEndian.Uint32(data[offset:])
	if size != 0x00000002 {
		return offset, fmt.Errorf("invalid PROJECTMODULES Size: 0x%08X (expected 0x00000002)", size)
	}
	offset += 4

	// Read Count (2 bytes)
	if offset+2 > len(data) {
		return offset, fmt.Errorf("insufficient data for Count")
	}
	dir.ModulesCount = binary.LittleEndian.Uint16(data[offset:])
	offset += 2

	// Read PROJECTCOOKIE (8 bytes)
	if offset+8 > len(data) {
		return offset, fmt.Errorf("insufficient data for PROJECTCOOKIE")
	}
	dir.ProjectCookie = binary.LittleEndian.Uint64(data[offset:])
	displayedProjectCookie := binary.LittleEndian.Uint16(data[offset:])
	offset += 8
	// ProjectCookie Validation, last 2 bytes must be ignored, must be 0xFFFF on write
	// rest bytes must be: 0x0013 00000002 FFFF (last 2 bytes may change, little endian)
	if dir.ProjectCookie&0x0000FFFFFFFFFFFF != uint64(0x000000020013) {
		return offset, fmt.Errorf("invalid PROJECTCOOKIE record: 0xZZZZ%016X (expected 0xZZZZ000000020013)", dir.ProjectCookie)
	}

	if outputCallback != nil {
		outputCallback("✅ PROJECTCOOKIE\n")
		outputCallback("   modulesCount:        %d\n", dir.ModulesCount)
		outputCallback("   ProjectCookie:   0x%04X\n", displayedProjectCookie)
	}

	// Parse MODULE records
	for i := uint16(0); i < dir.ModulesCount; i++ {
		module, newOffset, err := parseModuleRecord(data, offset, outputCallback)
		if err != nil {
			return offset, fmt.Errorf("failed to parse MODULE %d: %v", i, err)
		}
		dir.Modules = append(dir.Modules, *module)
		if outputCallback != nil {
			outputCallback("✅ Module %d\n", i+1)
			outputCallback("   Name:              %q\n", module.Name)
			if module.NameUnicode != "" {
				outputCallback("   NameUnicode:       %q\n", module.NameUnicode)
			}
			outputCallback("   StreamName:        %q\n", module.StreamName)
			outputCallback("   DocString:         %q\n", module.DocString)
			outputCallback("   TextOffset:        0x%08X\n", module.TextOffset)
			outputCallback("   HelpContext:       0x%08X\n", module.HelpContext)
			if module.IsProceduralModule {
				outputCallback("   ModuleType:        Procedural (0x0021)\n")
			} else {
				outputCallback("   ModuleType:        Document/Class/Designer (0x0022)\n")
			}
			if module.ReadOnly {
				outputCallback("   ReadOnly:          true\n")
			}
			if module.Private {
				outputCallback("   Private:           true\n")
			}
		}
		offset = newOffset
	}

	return offset, nil
}

// GetModuleStreamName returns the stream name for a module
func (d *DirStream) GetModuleStreamName(moduleName string) string {
	for _, module := range d.Modules {
		if module.Name == moduleName {
			if module.StreamName != "" {
				return module.StreamName
			}
			return module.StreamName
		}
	}
	return ""
}
