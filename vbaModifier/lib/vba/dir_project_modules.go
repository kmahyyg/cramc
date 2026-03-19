package vba

import (
	"encoding/binary"
	"fmt"
)

// parseModuleRecord parses a single MODULE record
// MS-OVBA Section 2.3.4.2.3.1
func parseModuleRecord(data []byte, offset int, outputCallback ParseOutputCallback) (*DirModule, int, error) {
	module := &DirModule{}
	startOffset := offset

	// NameRecord must be the first record in the module.
	// Read Id (2 bytes)
	if offset+2 > len(data) {
		return nil, offset, fmt.Errorf("insufficient data for MODULE Id")
	}
	id := binary.LittleEndian.Uint16(data[offset:])
	if id != 0x0019 {
		return nil, offset, fmt.Errorf("invalid MODULE Id: 0x%04X (expected 0x0019)", id)
	}
	offset += 2

	// Read SizeOfModuleName (4 bytes)
	if offset+4 > len(data) {
		return nil, offset, fmt.Errorf("insufficient data for SizeOfModuleName")
	}
	moduleNameSize := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	// ModuleName (variable)
	if offset+int(moduleNameSize) > len(data) {
		return nil, offset, fmt.Errorf("insufficient data for ModuleName (size: %d)", moduleNameSize)
	}
	module.Name = string(data[offset : offset+int(moduleNameSize)])
	offset += int(moduleNameSize)
	if outputCallback != nil {
		outputCallback("  MODULENAME (0x0019):\n")
		outputCallback("    Name:          %q\n", module.Name)
	}

	for offset < len(data) {
		if offset+2 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for next record Id")
		}
		nextRecordId := binary.LittleEndian.Uint16(data[offset:])
		if nextRecordId == 0x002B {
			if outputCallback != nil {
				outputCallback("    Terminator of current MODULE record found, 0x002B.\n")
			}
			break
		}
		offset += 2

		switch nextRecordId {
		case 0x0047: // ModuleNameUnicode
			if offset+4 > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for SizeOfModuleNameUnicode")
			}
			sizeOfModuleNameUnicode := binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if sizeOfModuleNameUnicode > 0 {
				if offset+int(sizeOfModuleNameUnicode) > len(data) {
					return nil, offset, fmt.Errorf("insufficient data for ModuleNameUnicode (size: %d)", sizeOfModuleNameUnicode)
				}
				module.NameUnicode = decodeUTF16LE(data[offset : offset+int(sizeOfModuleNameUnicode)])
				offset += int(sizeOfModuleNameUnicode)
				if outputCallback != nil {
					outputCallback("  MODULENAMEUNICODE (0x0047):\n")
					outputCallback("    ModuleNameUnicode:    %q\n", module.NameUnicode)
				}
			}
			continue
		case 0x001A: // ModuleStreamName
			// Read SizeOfStreamName (4 bytes)
			if offset+4 > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for SizeOfStreamName")
			}
			streamNameSize := binary.LittleEndian.Uint32(data[offset:])
			offset += 4

			// StreamName (variable)
			if offset+int(streamNameSize) > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for StreamName (size: %d)", streamNameSize)
			}
			module.StreamName = string(data[offset : offset+int(streamNameSize)])
			offset += int(streamNameSize)
			if outputCallback != nil {
				outputCallback("  MODULESTREAMNAME (0x001A):\n")
				outputCallback("    StreamName:           %q\n", module.StreamName)
			}

			reserved := binary.LittleEndian.Uint16(data[offset:])
			if reserved != 0x0032 {
				return nil, offset, fmt.Errorf("invalid reserved value: 0x%04X (expected 0x0032)", reserved)
			}
			offset += 2

			// Read SizeOfStreamNameUnicode (4 bytes)
			if offset+4 > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for SizeOfStreamNameUnicode")
			}
			streamNameUnicodeSize := binary.LittleEndian.Uint32(data[offset:])
			offset += 4

			// StreamNameUnicode (variable)
			if streamNameUnicodeSize > 0 {
				if offset+int(streamNameUnicodeSize) > len(data) {
					return nil, offset, fmt.Errorf("insufficient data for StreamNameUnicode (size: %d)", streamNameUnicodeSize)
				}
				unicodeBytes := data[offset : offset+int(streamNameUnicodeSize)]
				streamNameUnicode := decodeUTF16LE(unicodeBytes)
				offset += int(streamNameUnicodeSize)
				if outputCallback != nil {
					outputCallback("    StreamNameUnicode:    %q\n", streamNameUnicode)
				}
			}
			continue
		case 0x001C: // ModuleDocString record
			// Read SizeOfDocString (4 bytes)
			if offset+4 > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for SizeOfDocString")
			}
			docStringSize := binary.LittleEndian.Uint32(data[offset:])
			offset += 4

			if docStringSize > 0 {
				// DocString (variable)
				if offset+int(docStringSize) > len(data) {
					return nil, offset, fmt.Errorf("insufficient data for DocString (size: %d)", docStringSize)
				}
				module.DocString = string(data[offset : offset+int(docStringSize)])
				offset += int(docStringSize)
				if outputCallback != nil {
					outputCallback("  MODULEDOCSSTRING (0x001C):\n")
					outputCallback("    DocString:            %q\n", module.DocString)
				}
			} else {
				if outputCallback != nil {
					outputCallback("  MODULEDOCSSTRING (0x001C):\n")
					outputCallback("    DocString:       (empty)\n")
				}
			}

			reserved := binary.LittleEndian.Uint16(data[offset:])
			if reserved != 0x0048 {
				return nil, offset, fmt.Errorf("invalid reserved value: 0x%04X (expected 0x0048)", reserved)
			}
			offset += 2

			// Read SizeOfDocStringUnicode (4 bytes)
			if offset+4 > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for SizeOfDocStringUnicode")
			}
			docStringUnicodeSize := binary.LittleEndian.Uint32(data[offset:])
			offset += 4

			// DocStringUnicode (variable)
			if docStringUnicodeSize > 0 {
				if offset+int(docStringUnicodeSize) > len(data) {
					return nil, offset, fmt.Errorf("insufficient data for DocStringUnicode (size: %d)", docStringUnicodeSize)
				}
				unicodeBytes := data[offset : offset+int(docStringUnicodeSize)]
				docStringUnicode := decodeUTF16LE(unicodeBytes)
				offset += int(docStringUnicodeSize)
				if outputCallback != nil {
					outputCallback("    DocStringUnicode:     %q\n", docStringUnicode)
				}
			} else {
				if outputCallback != nil {
					outputCallback("  MODULEDOCSSTRING (0x001C):\n")
					outputCallback("    DocStringUnicode:    (empty)\n")
				}
			}
			continue
		case 0x0031: // ModuleOffset Record
			if offset+4 > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for SizeOfTextOffset")
			}
			sizeOfTextOffset := binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if sizeOfTextOffset != 0x00000004 {
				return nil, offset, fmt.Errorf("invalid SizeOfTextOffset value: 0x%08X (expected 0x00000004)", sizeOfTextOffset)
			}
			if offset+4 > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for TextOffset")
			}
			module.TextOffset = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if outputCallback != nil {
				outputCallback("  MODULETEXTOFFSET (0x0031):\n")
				outputCallback("    TextOffset:           0x%08X\n", module.TextOffset)
			}
			continue
		case 0x001E: // ModuleHelpContext record
			sizeOfHelpContext := binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if sizeOfHelpContext != 0x00000004 {
				return nil, offset, fmt.Errorf("invalid SizeOfHelpContext value: 0x%08X (expected 0x00000004)", sizeOfHelpContext)
			}
			if offset+4 > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for HelpContext")
			}
			module.HelpContext = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if outputCallback != nil {
				outputCallback("  MODULEHELPCONTEXT (0x001E):\n")
				outputCallback("    HelpContext:          0x%08X\n", module.HelpContext)
			}
			continue
		case 0x002C: // ModuleCookie record
			// Read SizeOfCookie (4 bytes)
			if offset+4 > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for SizeOfCookie")
			}
			cookieSize := binary.LittleEndian.Uint32(data[offset:])
			offset += 4

			if cookieSize != 0x02 {
				return nil, offset, fmt.Errorf("invalid SizeOfCookie value: 0x%08X (expected 0x00000002)", cookieSize)
			}

			// Read Cookie, must be ignored on read, must be 0xFFFF on write
			if offset+2 > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for Cookie")
			}
			cookieVal := binary.LittleEndian.Uint16(data[offset:])
			offset += 2
			if outputCallback != nil {
				outputCallback("  MODULECOOKIE (0x002C):\n")
				outputCallback("    Cookie:                0x%04X\n", cookieVal)
			}
			continue
		case 0x0021: // ModuleType record, but for procedural
			module.IsProceduralModule = true
			if outputCallback != nil {
				outputCallback("  MODULETYPE_PROCEDURAL (0x0021). \n")
			}
			fallthrough
		case 0x0022: // ModuleType record, but for document/class/designer
			if outputCallback != nil {
				outputCallback("  MODULETYPE_DOCUMENT/CLASS/DESIGNER (0x0022). \n")
			}
			if offset+4 > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for ModuleType Reserved.")
			}
			reserved := binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if reserved != 0x00000000 {
				return nil, offset, fmt.Errorf("invalid ModuleType Reserved value: 0x%08X (expected 0x00000000)", reserved)
			}
			continue
		case 0x0025: // ModuleReadOnly record
			if outputCallback != nil {
				outputCallback("  MODULEREADONLY (0x0025). \n")
			}
			if offset+4 > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for ModuleReadOnly Reserved.")
			}
			module.ReadOnly = true
			reserved := binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if reserved != 0x00000000 {
				return nil, offset, fmt.Errorf("invalid ModuleReadOnly Reserved value: 0x%08X (expected 0x00000000)", reserved)
			}
			continue
		case 0x0028: // ModulePrivate record
			if outputCallback != nil {
				outputCallback("  MODULEPRIVATE (0x0028). \n")
			}
			if offset+4 > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for ModulePrivate Reserved.")
			}
			module.Private = true
			reserved := binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if reserved != 0x00000000 {
				return nil, offset, fmt.Errorf("invalid ModulePrivate Reserved value: 0x%08X (expected 0x00000000)", reserved)
			}
		default:
			// end of parsing projectModules
			if outputCallback != nil {
				outputCallback("  Found unknown PROJECTMODULE record.\n")
				return nil, offset, fmt.Errorf("unknown PROJECTMODULE record.")
			}
			offset -= 2
			break
		}
	}

	if offset+2 > len(data) {
		return nil, offset, fmt.Errorf("insufficient data for terminator")
	}
	terminator := binary.LittleEndian.Uint16(data[offset:])
	offset += 2
	if terminator != 0x002B {
		return nil, offset, fmt.Errorf("invalid terminator value: 0x%04X (expected 0x002B)", terminator)
	}

	if offset+4 > len(data) {
		return nil, offset, fmt.Errorf("insufficient data for last reserved")
	}
	lastReserved := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	if lastReserved != 0x00000000 {
		return nil, offset, fmt.Errorf("invalid last reserved value: 0x%08X (expected 0x00000000)", lastReserved)
	}
	if outputCallback != nil {
		outputCallback("  Terminator and reserved field check passed.\n")
		outputCallback("  Parsed %d bytes for current MODULE record.\n", offset-startOffset)
		outputCallback("  END OF PROJECTMODULES.\n")
	}
	return module, offset, nil
}
