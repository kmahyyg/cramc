package vba

import (
	"encoding/binary"
	"fmt"
)

// parseProjectInformation parses PROJECTINFORMATION record
// MS-OVBA Section 2.3.4.2.1
func parseProjectInformation(data []byte, offset int, dir *DirStream, outputCallback ParseOutputCallback) (int, error) {
	for offset < len(data) {
		// Check if we have enough for Id and Size
		if offset+6 > len(data) {
			return offset, fmt.Errorf("insufficient data for record header")
		}

		// Read record Id (2 bytes)
		recordId := binary.LittleEndian.Uint16(data[offset:])
		offset += 2

		// Check if this is the end of PROJECTINFORMATION
		// PROJECTREFERENCES starts with a REFERENCE record (Id != PROJECTINFORMATION sub-record)
		// We'll detect this when we see an unknown Id or 0x000F
		if recordId == 0x000F {
			// This is the start of PROJECTMODULES, back up
			offset -= 2
			return offset, nil
		}

		var size uint32
		// Parse based on record Id
		var err error
		switch recordId {
		case 0x0001: // PROJECTSYSKIND
			size = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if size != 4 {
				return offset, fmt.Errorf("PROJECTSYSKIND: invalid size %d (expected 4)", size)
			}
			if offset+4 > len(data) {
				return offset, fmt.Errorf("PROJECTSYSKIND: insufficient data")
			}
			dir.SysKind = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if outputCallback != nil {
				outputCallback("✅ PROJECTSYSKIND\n")
				outputCallback("   Id:     0x%04X\n", recordId)
				outputCallback("   Size:   %d (0x%08X)\n", size, size)
				outputCallback("   SysKind: 0x%08X\n", dir.SysKind)
			}

		case 0x0002: // PROJECTLCID (MS-OVBA Section 2.3.4.2.1.3)
			// According to spec: Id MUST be 0x0002, Size MUST be 0x00000004
			size = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if size != 4 {
				return offset, fmt.Errorf("PROJECTLCID: invalid size %d (expected 4)", size)
			}
			if offset+4 > len(data) {
				return offset, fmt.Errorf("PROJECTLCID: insufficient data")
			}
			dir.Lcid = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if outputCallback != nil {
				outputCallback("✅ PROJECTLCID\n")
				outputCallback("   Id:   0x%04X\n", recordId)
				outputCallback("   Size: %d (0x%08X)\n", size, size)
				outputCallback("   Lcid: 0x%08X\n", dir.Lcid)
			}

		case 0x0014: // PROJECTLCIDINVOKE (MS-OVBA Section 2.3.4.2.1.4)
			// According to spec: Id MUST be 0x0014, Size MUST be 0x00000004
			size = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if size != 4 {
				return offset, fmt.Errorf("PROJECTLCIDINVOKE: invalid size %d (expected 4)", size)
			}
			if offset+4 > len(data) {
				return offset, fmt.Errorf("PROJECTLCIDINVOKE: insufficient data")
			}
			dir.LcidInvoke = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if outputCallback != nil {
				outputCallback("✅ PROJECTLCIDINVOKE\n")
				outputCallback("   Id:         0x%04X\n", recordId)
				outputCallback("   Size:       %d (0x%08X)\n", size, size)
				outputCallback("   LcidInvoke: 0x%08X\n", dir.LcidInvoke)
			}

		case 0x004A: // PROJECTCOMPATVERSION (MS-OVBA Section 2.3.4.2.1.2) - optional
			// According to spec: Id MUST be 0x004A, Size MUST be 0x00000004
			size = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if size != 4 {
				// Non-standard size - skip it
				if offset+int(size) > len(data) {
					return offset, fmt.Errorf("PROJECTCOMPATVERSION: insufficient data (size: %d)", size)
				}
				offset += int(size)
				if outputCallback != nil {
					outputCallback("⚠️  PROJECTCOMPATVERSION: skipped (unexpected size: %d)\n", size)
					outputCallback("   Id:   0x%04X\n", recordId)
					outputCallback("   Size: %d (0x%08X) [expected 4]\n", size, size)
				}
			} else {
				if offset+4 > len(data) {
					return offset, fmt.Errorf("PROJECTCOMPATVERSION: insufficient data")
				}
				// VersionMajor (2 bytes), VersionMinor (2 bytes)
				major := binary.LittleEndian.Uint16(data[offset:])
				minor := binary.LittleEndian.Uint16(data[offset+2:])
				offset += 4
				if outputCallback != nil {
					outputCallback("✅ PROJECTCOMPATVERSION\n")
					outputCallback("   Id:           0x%04X\n", recordId)
					outputCallback("   Size:         %d (0x%08X)\n", size, size)
					outputCallback("   VersionMajor: %d\n", major)
					outputCallback("   VersionMinor: %d\n", minor)
					outputCallback("   Version:      %d.%d\n", major, minor)
				}
			}

		case 0x0003: // PROJECTCODEPAGE (MS-OVBA Section 2.3.4.2.1.5)
			// According to spec: Id MUST be 0x0003, Size MUST be 0x00000002
			size = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if size != 2 {
				return offset, fmt.Errorf("PROJECTCODEPAGE: invalid size %d (expected 2)", size)
			}
			if offset+2 > len(data) {
				return offset, fmt.Errorf("PROJECTCODEPAGE: insufficient data")
			}
			dir.CodePage = binary.LittleEndian.Uint16(data[offset:])
			offset += 2
			if outputCallback != nil {
				outputCallback("✅ PROJECTCODEPAGE\n")
				outputCallback("   Id:      0x%04X\n", recordId)
				outputCallback("   Size:    %d (0x%08X)\n", size, size)
				outputCallback("   CodePage: 0x%04X\n", dir.CodePage)
			}

		case 0x0004: // PROJECTNAME (MS-OVBA Section 2.3.4.2.1.6)
			// According to spec: Id MUST be 0x0004
			// Structure: The Size field (4 bytes) IS the SizeOfProjectName
			// ProjectName follows immediately (variable, 1-128 bytes)
			// Validate SizeOfProjectName: MUST be 1-128
			size = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			nameSize := int(size)
			if nameSize < 1 || nameSize > 128 {
				return offset, fmt.Errorf("PROJECTNAME: invalid SizeOfProjectName %d (must be 1-128)", nameSize)
			}
			if offset+nameSize > len(data) {
				return offset, fmt.Errorf("PROJECTNAME: insufficient data for ProjectName (size: %d)", nameSize)
			}
			// ProjectName is encoded using the code page specified in PROJECTCODEPAGE (typically 1252/ASCII)
			dir.Name = string(data[offset : offset+nameSize])
			offset += nameSize
			if outputCallback != nil {
				outputCallback("✅ PROJECTNAME\n")
				outputCallback("   Id:               0x%04X\n", recordId)
				outputCallback("   Size:             %d (0x%08X) [SizeOfProjectName]\n", size, size)
				outputCallback("   ProjectName:      %q\n", dir.Name)
			}

		case 0x0005: // PROJECTDOCSTRING (MS-OVBA Section 2.3.4.2.1.7)
			// PROJECTDOCSTRING (0x0005) does NOT have a Size field - it goes directly from ID to SizeOfDocString
			offset, err = parseDocStringRecord(data, offset, &dir.DocString, &dir.DocStringUnicode)
			if err != nil {
				return offset, fmt.Errorf("PROJECTDOCSTRING: %v", err)
			}
			if outputCallback != nil {
				outputCallback("✅ PROJECTDOCSTRING\n")
				outputCallback("   Id:                   0x%04X\n", recordId)
				outputCallback("   DocStringSize:         %d\n", len(dir.DocString))
				if dir.DocString != "" {
					outputCallback("   DocString:             %q\n", dir.DocString)
				} else {
					outputCallback("   DocString:             (empty)\n")
				}
				if dir.DocStringUnicode != "" {
					outputCallback("   DocStringUnicodeSize:  %d\n", len(dir.DocStringUnicode)*2)
					outputCallback("   DocStringUnicode:      %q\n", dir.DocStringUnicode)
				} else {
					outputCallback("   DocStringUnicode:      (empty)\n")
				}
			}

		case 0x0006: // PROJECTHELPFILEPATH (MS-OVBA Section 2.3.4.2.1.8)
			offset, err = parseHelpFilePathRecord(data, offset, &dir.HelpFile, &dir.HelpFileUnicode)
			if err != nil {
				return offset, fmt.Errorf("PROJECTHELPFILEPATH: %v", err)
			}
			if outputCallback != nil {
				outputCallback("✅ PROJECTHELPFILEPATH\n")
				outputCallback("   Id:               0x%04X\n", recordId)
				outputCallback("   Size:              %d (0x%08X)\n", size, size)
				outputCallback("   HelpFile1Size:     %d\n", len(dir.HelpFile))
				outputCallback("   HelpFile1:         %q\n", dir.HelpFile)
				if dir.HelpFileUnicode != "" {
					outputCallback("   HelpFile2Size:     %d\n", len(dir.HelpFileUnicode)*2)
					outputCallback("   HelpFile2:         %q\n", dir.HelpFileUnicode)
				}
			}

		case 0x0007: // PROJECTHELPCONTEXT (MS-OVBA Section 2.3.4.2.1.9)
			// According to spec: Id MUST be 0x0007, Size MUST be 0x00000004
			size = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if size != 4 {
				return offset, fmt.Errorf("PROJECTHELPCONTEXT: invalid size %d (expected 4)", size)
			}
			if offset+4 > len(data) {
				return offset, fmt.Errorf("PROJECTHELPCONTEXT: insufficient data")
			}
			// HelpContext (4 bytes): An unsigned integer that specifies the Help topic identifier
			// in the Help file specified by PROJECTHELPFILEPATH (section 2.3.4.2.1.8)
			dir.HelpContext = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if outputCallback != nil {
				outputCallback("✅ PROJECTHELPCONTEXT\n")
				outputCallback("   Id:          0x%04X\n", recordId)
				outputCallback("   Size:        %d (0x%08X)\n", size, size)
				outputCallback("   HelpContext: Topic No.0x%08X\n", dir.HelpContext)
			}

		case 0x0008: // PROJECTLIBFLAGS (MS-OVBA Section 2.3.4.2.1.10)
			// According to spec: Id MUST be 0x0008, Size MUST be 0x00000004
			size = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if size != 4 {
				return offset, fmt.Errorf("PROJECTLIBFLAGS: invalid size %d (expected 4)", size)
			}
			if offset+4 > len(data) {
				return offset, fmt.Errorf("PROJECTLIBFLAGS: insufficient data")
			}
			// ProjectLibFlags (4 bytes): An unsigned integer that specifies LIBFLAGS
			// for the VBA project's Automation type library as specified in [MS-OAUT] section 2.2.20.
			// MUST be 0x00000000 according to spec, but we'll read whatever value is present
			dir.LibFlags = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if outputCallback != nil {
				outputCallback("✅ PROJECTLIBFLAGS\n")
				outputCallback("   Id:           0x%04X\n", recordId)
				outputCallback("   Size:         %d (0x%08X)\n", size, size)
				outputCallback("   ProjectLibFlags: 0x%08X\n", dir.LibFlags)
			}

		case 0x0009: // PROJECTVERSION (MS-OVBA Section 2.3.4.2.1.11)
			// According to spec: Id MUST be 0x0009
			// Structure: Reserved (4 bytes, MUST be 0x00000004), VersionMajor (4 bytes), VersionMinor (2 bytes)
			// Note: The Size field in the record format is the Reserved field
			reserved := binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if reserved != 4 {
				return offset, fmt.Errorf("PROJECTVERSION: invalid Reserved value %d (expected 4)", reserved)
			}
			// Check if we have enough data for VersionMajor (4 bytes) + VersionMinor (2 bytes) = 6 bytes
			if offset+6 > len(data) {
				return offset, fmt.Errorf("PROJECTVERSION: insufficient data (need 6 bytes for VersionMajor and VersionMinor)")
			}
			// VersionMajor (4 bytes): An unsigned integer that specifies the major version of the VBA project
			dir.MajorVersion = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			// VersionMinor (2 bytes): An unsigned integer that specifies the minor version of the VBA project
			dir.MinorVersion = binary.LittleEndian.Uint16(data[offset:])
			offset += 2
			if outputCallback != nil {
				outputCallback("✅ PROJECTVERSION\n")
				outputCallback("   Id:           0x%04X\n", recordId)
				outputCallback("   Reserved:     %d (0x%08X) [MUST be 4]\n", reserved, reserved)
				outputCallback("   VersionMajor: %d\n", dir.MajorVersion)
				outputCallback("   VersionMinor: %d\n", dir.MinorVersion)
			}

		case 0x000C: // PROJECTCONSTANTS (MS-OVBA Section 2.3.4.2.1.12) - optional
			// SizeOfConstants (4 bytes): MUST be less than or equal to 1015
			if offset+4 > len(data) {
				return offset, fmt.Errorf("PROJECTCONSTANTS: insufficient data for SizeOfConstants")
			}
			constantsSize := binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if constantsSize > 1015 {
				return offset, fmt.Errorf("PROJECTCONSTANTS: SizeOfConstants exceeds maximum (size: %d, max: 1015)", constantsSize)
			}

			// Constants (variable): MBCS characters encoded using code page from PROJECTCODEPAGE
			if constantsSize > 0 {
				if offset+int(constantsSize) > len(data) {
					return offset, fmt.Errorf("PROJECTCONSTANTS: insufficient data for Constants (size: %d)", constantsSize)
				}
				dir.Constants = string(data[offset : offset+int(constantsSize)])
				offset += int(constantsSize)
			} else {
				dir.Constants = ""
			}

			// Reserved (2 bytes): MUST be 0x003C, MUST be ignored
			if offset+2 > len(data) {
				return offset, fmt.Errorf("PROJECTCONSTANTS: insufficient data for Reserved")
			}
			reserved := binary.LittleEndian.Uint16(data[offset:])
			if reserved != 0x003C {
				// According to spec, MUST be 0x003C, but we'll just warn and continue
			}
			offset += 2

			// SizeOfConstantsUnicode (4 bytes): MUST be even
			if offset+4 > len(data) {
				return offset, fmt.Errorf("PROJECTCONSTANTS: insufficient data for SizeOfConstantsUnicode")
			}
			constantsUnicodeSize := binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if constantsUnicodeSize > 0 && constantsUnicodeSize%2 != 0 {
				return offset, fmt.Errorf("PROJECTCONSTANTS: SizeOfConstantsUnicode must be even (size: %d)", constantsUnicodeSize)
			}

			// ConstantsUnicode (variable): UTF-16 characters, MUST contain UTF-16 encoding of Constants
			if constantsUnicodeSize > 0 {
				if offset+int(constantsUnicodeSize) > len(data) {
					return offset, fmt.Errorf("PROJECTCONSTANTS: insufficient data for ConstantsUnicode (size: %d)", constantsUnicodeSize)
				}
				unicodeBytes := data[offset : offset+int(constantsUnicodeSize)]
				dir.ConstantsUnicode = decodeUTF16LE(unicodeBytes)
				offset += int(constantsUnicodeSize)
			} else {
				dir.ConstantsUnicode = ""
			}

			if outputCallback != nil {
				outputCallback("✅ PROJECTCONSTANTS\n")
				outputCallback("   Id:                     0x%04X\n", recordId)
				outputCallback("   SizeOfConstants:        %d (0x%08X)\n", constantsSize, constantsSize)
				if dir.Constants != "" {
					outputCallback("   Constants:              %q\n", dir.Constants)
				} else {
					outputCallback("   Constants:              (empty)\n")
				}
				outputCallback("   Reserved:               0x%04X (MUST be 0x003C)\n", reserved)
				outputCallback("   SizeOfConstantsUnicode: %d (0x%08X)\n", constantsUnicodeSize, constantsUnicodeSize)
				if dir.ConstantsUnicode != "" {
					outputCallback("   ConstantsUnicode:       %q\n", dir.ConstantsUnicode)
				} else {
					outputCallback("   ConstantsUnicode:       (empty)\n")
				}
			}

		default:
			// Unknown record type - all known PROJECTINFORMATION records are handled above
			// This must be the start of PROJECTREFERENCES, back up to the start of this record
			if outputCallback != nil {
				outputCallback("=== End of PROJECTINFORMATION (transitioning to PROJECTREFERENCES) ===\n")
			}
			offset -= 2
			return offset, nil
		}
	}

	return offset, nil
}

// parseDocStringRecord parses a PROJECTDOCSTRING record (Id 0x0005)
// MS-OVBA Section 2.3.4.2.1.7
func parseDocStringRecord(data []byte, offset int, docString *string, docStringUnicode *string) (int, error) {
	// SizeOfDocString (4 bytes)
	// MUST be less than or equal to 2000
	if offset+4 > len(data) {
		return offset, fmt.Errorf("insufficient data for SizeOfDocString")
	}
	docStringSize := binary.LittleEndian.Uint32(data[offset:])
	if docStringSize > 2000 {
		return offset, fmt.Errorf("SizeOfDocString exceeds maximum (size: %d, max: 2000)", docStringSize)
	}
	offset += 4

	// DocString (variable)
	// Array of SizeOfDocString bytes, MBCS characters encoded using code page from PROJECTCODEPAGE
	if offset+int(docStringSize) > len(data) {
		return offset, fmt.Errorf("insufficient data for DocString (size: %d)", docStringSize)
	}
	if docStringSize > 0 {
		*docString = string(data[offset : offset+int(docStringSize)])
		offset += int(docStringSize)
	} else {
		*docString = ""
	}

	// Reserved (2 bytes)
	// MUST be 0x0040, MUST be ignored
	if offset+2 > len(data) {
		return offset, fmt.Errorf("insufficient data for Reserved")
	}
	reserved := binary.LittleEndian.Uint16(data[offset:])
	if reserved != 0x0040 {
		// According to spec, MUST be 0x0040, but we'll just warn and continue
		// (some files might not strictly follow the spec)
	}
	offset += 2

	// SizeOfDocStringUnicode (4 bytes)
	// MUST be even
	if offset+4 > len(data) {
		return offset, fmt.Errorf("insufficient data for SizeOfDocStringUnicode")
	}
	docStringUnicodeSize := binary.LittleEndian.Uint32(data[offset:])
	if docStringUnicodeSize > 0 && docStringUnicodeSize%2 != 0 {
		return offset, fmt.Errorf("SizeOfDocStringUnicode must be even (size: %d)", docStringUnicodeSize)
	}
	offset += 4

	// DocStringUnicode (variable)
	// Array of SizeOfDocStringUnicode bytes, UTF-16 characters
	if docStringUnicodeSize > 0 {
		if offset+int(docStringUnicodeSize) > len(data) {
			return offset, fmt.Errorf("insufficient data for DocStringUnicode (size: %d)", docStringUnicodeSize)
		}
		// Convert UTF-16LE to string
		unicodeBytes := data[offset : offset+int(docStringUnicodeSize)]
		*docStringUnicode = decodeUTF16LE(unicodeBytes)
		offset += int(docStringUnicodeSize)
	} else {
		*docStringUnicode = ""
	}

	return offset, nil
}

// parseHelpFilePathRecord parses a HELPFILEPATH record (Id 0x0006)
// MS-OVBA Section 2.3.4.2.1.8
func parseHelpFilePathRecord(data []byte, offset int, helpFile *string, helpFileUnicode *string) (int, error) {
	// SizeOfHelpFile1 (4 bytes)
	if offset+4 > len(data) {
		return offset, fmt.Errorf("insufficient data for SizeOfHelpFile1")
	}
	helpFile1Size := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	// HelpFile1 (variable)
	if offset+int(helpFile1Size) > len(data) {
		return offset, fmt.Errorf("insufficient data for HelpFile1 (size: %d)", helpFile1Size)
	}
	*helpFile = string(data[offset : offset+int(helpFile1Size)])
	offset += int(helpFile1Size)

	// Reserved (2 bytes)
	if offset+2 > len(data) {
		return offset, fmt.Errorf("insufficient data for Reserved1")
	}
	reserved := binary.LittleEndian.Uint16(data[offset:])
	if reserved != 0x003D {
		return offset, fmt.Errorf("invalid Reserved value: 0x%04X (expected 0x003D)", reserved)
	}
	offset += 2

	// SizeOfHelpFile2 (4 bytes)
	if offset+4 > len(data) {
		return offset, fmt.Errorf("insufficient data for SizeOfHelpFile2")
	}
	helpFile2Size := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	// HelpFile2 (variable) - Unicode version
	if helpFile2Size > 0 {
		if offset+int(helpFile2Size) > len(data) {
			return offset, fmt.Errorf("insufficient data for HelpFile2 (size: %d)", helpFile2Size)
		}
		unicodeBytes := data[offset : offset+int(helpFile2Size)]
		*helpFileUnicode = decodeUTF16LE(unicodeBytes)
		offset += int(helpFile2Size)
	}

	return offset, nil
}
