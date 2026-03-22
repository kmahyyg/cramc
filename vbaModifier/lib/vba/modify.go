package vba

import (
	"encoding/binary"
	"fmt"
)

// ModuleTextOffsetInfo captures MODULEOFFSET metadata inside a decompressed dir stream.
type ModuleTextOffsetInfo struct {
	Name               string
	StreamName         string
	TextOffset         uint32
	TextOffsetFieldPos int // byte offset of the uint32 value in decompressed dir bytes
}

// PatchModuleTextOffsetInDirStream sets MODULEOFFSET(TextOffset) for a target module in the dir stream.
// It returns the recompressed dir stream and all discovered module TextOffset records.
func PatchModuleTextOffsetInDirStream(dirCompressed []byte, targetModule string, newOffset uint32) ([]byte, []ModuleTextOffsetInfo, error) {
	decompressed, err := Decompress(dirCompressed)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrDecompressionFailed, err)
	}

	scratch := &DirStream{}
	offset := 0
	offset, err = parseProjectInformation(decompressed, offset, scratch, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: failed to parse PROJECTINFORMATION: %v", ErrInvalidDirFormat, err)
	}
	offset, err = parseProjectReferences(decompressed, offset, scratch, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: failed to parse PROJECTREFERENCES: %v", ErrInvalidDirFormat, err)
	}

	infos, err := collectModuleTextOffsetInfos(decompressed, offset)
	if err != nil {
		return nil, nil, err
	}

	found := false
	for _, info := range infos {
		if info.Name == targetModule || info.StreamName == targetModule {
			binary.LittleEndian.PutUint32(decompressed[info.TextOffsetFieldPos:], newOffset)
			found = true
		}
	}
	if !found {
		return nil, infos, fmt.Errorf("%w: module %q", ErrModuleNotFound, targetModule)
	}

	recompressed, err := Compress(decompressed)
	if err != nil {
		return nil, infos, fmt.Errorf("failed to recompress patched dir stream: %w", err)
	}

	return recompressed, infos, nil
}

// StripVBAProjectPerformanceCache removes the PerformanceCache blob from _VBA_PROJECT.
// Per MS-OVBA 2.3.4.1, bytes beyond offset 0x0007 MUST NOT be present on write.
func StripVBAProjectPerformanceCache() []byte {
	// directly hard-coded per standard
	// https://attack.mitre.org/techniques/T1564/007/
	// please do not directly copy first 7 bytes of P-code cache to prevent from executing
	finalOut := []byte{0xCC, 0x61, 0xFF, 0xFF, 0x00, 0x01, 0x00}
	return finalOut
}

func collectModuleTextOffsetInfos(data []byte, offset int) ([]ModuleTextOffsetInfo, error) {
	if offset+2 > len(data) {
		return nil, fmt.Errorf("%w: insufficient data for PROJECTMODULES id", ErrInvalidDirFormat)
	}
	if binary.LittleEndian.Uint16(data[offset:]) != 0x000F {
		return nil, fmt.Errorf("%w: expected PROJECTMODULES id 0x000F", ErrInvalidDirFormat)
	}
	offset += 2

	if offset+4 > len(data) {
		return nil, fmt.Errorf("%w: insufficient data for PROJECTMODULES size", ErrInvalidDirFormat)
	}
	if binary.LittleEndian.Uint32(data[offset:]) != 0x00000002 {
		return nil, fmt.Errorf("%w: invalid PROJECTMODULES size", ErrInvalidDirFormat)
	}
	offset += 4

	if offset+2 > len(data) {
		return nil, fmt.Errorf("%w: insufficient data for module count", ErrInvalidDirFormat)
	}
	moduleCount := binary.LittleEndian.Uint16(data[offset:])
	offset += 2

	if offset+8 > len(data) {
		return nil, fmt.Errorf("%w: insufficient data for PROJECTCOOKIE", ErrInvalidDirFormat)
	}
	offset += 8

	infos := make([]ModuleTextOffsetInfo, 0, moduleCount)
	for i := 0; i < int(moduleCount); i++ {
		info, nextOffset, err := parseModuleTextOffsetInfo(data, offset)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to parse module %d: %v", ErrInvalidDirFormat, i+1, err)
		}
		infos = append(infos, info)
		offset = nextOffset
	}

	return infos, nil
}

func parseModuleTextOffsetInfo(data []byte, offset int) (ModuleTextOffsetInfo, int, error) {
	info := ModuleTextOffsetInfo{}

	if offset+2 > len(data) {
		return info, offset, fmt.Errorf("insufficient data for MODULENAME id")
	}
	if binary.LittleEndian.Uint16(data[offset:]) != 0x0019 {
		return info, offset, fmt.Errorf("expected MODULENAME record (0x0019)")
	}
	offset += 2

	if offset+4 > len(data) {
		return info, offset, fmt.Errorf("insufficient data for module name size")
	}
	moduleNameSize := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	if offset+int(moduleNameSize) > len(data) {
		return info, offset, fmt.Errorf("insufficient data for module name payload")
	}
	info.Name = string(data[offset : offset+int(moduleNameSize)])
	offset += int(moduleNameSize)

	for {
		if offset+2 > len(data) {
			return info, offset, fmt.Errorf("insufficient data for module record id")
		}
		recordID := binary.LittleEndian.Uint16(data[offset:])
		if recordID == 0x002B {
			break
		}
		offset += 2

		switch recordID {
		case 0x0047:
			size, next, err := skipSizedField(data, offset)
			if err != nil {
				return info, offset, fmt.Errorf("invalid MODULENAMEUNICODE: %v", err)
			}
			_ = size
			offset = next
		case 0x001A:
			streamNameSize, next, err := skipSizedField(data, offset)
			if err != nil {
				return info, offset, fmt.Errorf("invalid MODULESTREAMNAME: %v", err)
			}
			if offset+4+int(streamNameSize) > len(data) {
				return info, offset, fmt.Errorf("invalid MODULESTREAMNAME payload")
			}
			info.StreamName = string(data[offset+4 : offset+4+int(streamNameSize)])
			offset = next

			if offset+2 > len(data) {
				return info, offset, fmt.Errorf("missing MODULESTREAMNAME reserved")
			}
			if binary.LittleEndian.Uint16(data[offset:]) != 0x0032 {
				return info, offset, fmt.Errorf("invalid MODULESTREAMNAME reserved")
			}
			offset += 2

			_, next, err = skipSizedField(data, offset)
			if err != nil {
				return info, offset, fmt.Errorf("invalid MODULESTREAMNAMEUNICODE: %v", err)
			}
			offset = next
		case 0x001C:
			_, next, err := skipSizedField(data, offset)
			if err != nil {
				return info, offset, fmt.Errorf("invalid MODULEDOCSTRING: %v", err)
			}
			offset = next

			if offset+2 > len(data) {
				return info, offset, fmt.Errorf("missing MODULEDOCSTRING reserved")
			}
			if binary.LittleEndian.Uint16(data[offset:]) != 0x0048 {
				return info, offset, fmt.Errorf("invalid MODULEDOCSTRING reserved")
			}
			offset += 2

			_, next, err = skipSizedField(data, offset)
			if err != nil {
				return info, offset, fmt.Errorf("invalid MODULEDOCSTRINGUNICODE: %v", err)
			}
			offset = next
		case 0x0031:
			if offset+8 > len(data) {
				return info, offset, fmt.Errorf("invalid MODULEOFFSET payload")
			}
			if binary.LittleEndian.Uint32(data[offset:]) != 0x00000004 {
				return info, offset, fmt.Errorf("invalid MODULEOFFSET size")
			}
			info.TextOffsetFieldPos = offset + 4
			info.TextOffset = binary.LittleEndian.Uint32(data[offset+4:])
			offset += 8
		case 0x001E:
			if offset+8 > len(data) {
				return info, offset, fmt.Errorf("invalid MODULEHELPCONTEXT payload")
			}
			if binary.LittleEndian.Uint32(data[offset:]) != 0x00000004 {
				return info, offset, fmt.Errorf("invalid MODULEHELPCONTEXT size")
			}
			offset += 8
		case 0x002C:
			if offset+6 > len(data) {
				return info, offset, fmt.Errorf("invalid MODULECOOKIE payload")
			}
			if binary.LittleEndian.Uint32(data[offset:]) != 0x00000002 {
				return info, offset, fmt.Errorf("invalid MODULECOOKIE size")
			}
			offset += 6
		case 0x0021, 0x0022, 0x0025, 0x0028:
			if offset+4 > len(data) {
				return info, offset, fmt.Errorf("invalid module flag payload")
			}
			if binary.LittleEndian.Uint32(data[offset:]) != 0x00000000 {
				return info, offset, fmt.Errorf("invalid module flag reserved")
			}
			offset += 4
		default:
			return info, offset, fmt.Errorf("unknown module record id 0x%04X", recordID)
		}
	}

	if info.TextOffsetFieldPos == 0 {
		return info, offset, fmt.Errorf("MODULEOFFSET record missing")
	}

	if offset+2 > len(data) {
		return info, offset, fmt.Errorf("insufficient data for module terminator")
	}
	if binary.LittleEndian.Uint16(data[offset:]) != 0x002B {
		return info, offset, fmt.Errorf("invalid module terminator")
	}
	offset += 2

	if offset+4 > len(data) {
		return info, offset, fmt.Errorf("insufficient data for module reserved")
	}
	if binary.LittleEndian.Uint32(data[offset:]) != 0x00000000 {
		return info, offset, fmt.Errorf("invalid module reserved")
	}
	offset += 4

	return info, offset, nil
}

func skipSizedField(data []byte, offset int) (uint32, int, error) {
	if offset+4 > len(data) {
		return 0, offset, fmt.Errorf("missing size field")
	}
	size := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(size) > len(data) {
		return 0, offset, fmt.Errorf("field exceeds available data")
	}
	offset += int(size)
	return size, offset, nil
}
