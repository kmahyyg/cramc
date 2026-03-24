package vba

import (
	"encoding/binary"
	"fmt"
)

// parseProjectReferences parses PROJECTREFERENCES record
// MS-OVBA Section 2.3.4.2.2
// Each REFERENCE record consists of:
//   - REFERENCENAME (0x0016) - mandatory
//   - One of: REFERENCECONTROL (0x002F), REFERENCEORIGINAL (0x0033),
//     REFERENCEREGISTERED (0x000D), or REFERENCEPROJECT (0x000E)
func parseProjectReferences(data []byte, offset int, dir *DirStream, outputCallback ParseOutputCallback) (int, error) {
	for offset < len(data) {
		// Check if we have enough for Id
		if offset+2 > len(data) {
			return offset, fmt.Errorf("insufficient data for reference record Id")
		}

		// Read record Id (2 bytes) to peek
		recordId := binary.LittleEndian.Uint16(data[offset:])

		// Check if this is the start of PROJECTMODULES (0x000F)
		// Note: 0x000F is PROJECTMODULES, 0x000E is REFERENCEPROJECT
		// We distinguish by checking if it's followed by Size=0x00000002
		if recordId == 0x000F {
			// Check if this is PROJECTMODULES (Size must be 0x00000002)
			if offset+6 <= len(data) {
				size := binary.LittleEndian.Uint32(data[offset+2:])
				if size == 0x00000002 {
					// This is PROJECTMODULES, not a reference
					return offset, nil
				}
			}
		}

		// Each REFERENCE must start with REFERENCENAME (0x0016)
		if recordId != 0x0016 {
			// Not a REFERENCENAME - might be the start of PROJECTMODULES or corrupted data
			return offset, nil
		}

		// Parse REFERENCE record (starts with REFERENCENAME, followed by reference type)
		ref, newOffset, err := parseReferenceRecord(data, offset, outputCallback, len(dir.References)+1)
		if err != nil {
			return offset, fmt.Errorf("failed to parse REFERENCE: %v", err)
		}
		dir.References = append(dir.References, *ref)
		offset = newOffset
	}

	return offset, nil
}

// parseReferenceRecord parses a single REFERENCE record
// MS-OVBA Section 2.3.4.2.2.1
// Structure: REFERENCENAME (0x0016) followed by one of:
//   - REFERENCECONTROL (0x002F)
//   - REFERENCEORIGINAL (0x0033)
//   - REFERENCEREGISTERED (0x000D)
//   - REFERENCEPROJECT (0x000E)
func parseReferenceRecord(data []byte, offset int, outputCallback ParseOutputCallback, refNum int) (*DirReference, int, error) {
	ref := &DirReference{}
	startOffset := offset

	if outputCallback != nil {
		outputCallback("\n--- Reference %d ---\n", refNum)
	}

	var err error
	// Step 1: Parse the REFERENCENAME record header (0x0016)
	ref.RefName, offset, err = parseReferenceNameRecord(data, offset, outputCallback, refNum, false)
	if err != nil {
		return nil, offset, fmt.Errorf("failed to parse REFERENCENAME: %v", err)
	}

	// Step 2: Parse the actual reference type record
	if offset+2 > len(data) {
		return nil, offset, fmt.Errorf("insufficient data for ReferenceType")
	}
	ref.ReferenceType = binary.LittleEndian.Uint16(data[offset:])
	offset += 2

	if outputCallback != nil {
		outputCallback("  ReferenceType: 0x%04X", ref.ReferenceType)
		switch ref.ReferenceType {
		case 0x002F:
			outputCallback(" (REFERENCECONTROL)\n")
		case 0x0033:
			outputCallback(" (REFERENCEORIGINAL)\n")
		case 0x000D:
			outputCallback(" (REFERENCEREGISTERED)\n")
		case 0x000E:
			outputCallback(" (REFERENCEPROJECT)\n")
		default:
			outputCallback(" (UNKNOWN)\n")
		}
	}

	// Parse based on ReferenceType
	switch ref.ReferenceType {
	case 0x0033: // REFERENCEORIGINAL
		// This type of record contains its original fields plus a ReferenceControl field
		// so parse it as-is, then if SizeOfLibidOriginal is not zero, proceed with cautious and fallthrough
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for SizeOfLibidOriginal")
		}
		libidOriginalSize := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		if outputCallback != nil {
			outputCallback("    REFERENCEORIGINAL sub-records:\n")
			outputCallback("      SizeOfLibidOriginal: %d (0x%08X)\n", libidOriginalSize, libidOriginalSize)
		}
		if libidOriginalSize == 0 {
			return nil, offset, fmt.Errorf("invalid SizeOfLibidOriginal value: 0x%08X (expected non-zero)", libidOriginalSize)
		}
		ref.LibidOriginal = string(data[offset : offset+int(libidOriginalSize)])
		offset += int(libidOriginalSize)
		if outputCallback != nil {
			outputCallback("      LibidOriginal:       %q\n", ref.LibidOriginal)
			outputCallback("      ======= WARNING: UN-TESTED PARSER CODE REACHED =======")
			outputCallback("      ReferenceOriginal record is followed by a ReferenceControl record")
			outputCallback("      which is not yet supported by this parser. Please report")
			outputCallback("      this file if you encounter it.")
			outputCallback("      =====================Fallthrough NOW==================\n")
			outputCallback("      ======================================================\n")
		}
		fallthrough
	case 0x002F: // REFERENCECONTROL
		// SizeTwiddled (4 bytes): An unsigned integer that specifies the sum of the size in bytes of SizeOfLibidTwiddled, LibidTwiddled, Reserved1, and Reserved2.
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for SizeTwiddled")
		}
		twiddledSize := binary.LittleEndian.Uint32(data[offset:])
		offset += 4

		if outputCallback != nil {
			outputCallback("    REFERENCECONTROL sub-records:\n")
			outputCallback("      SizeTwiddled: %d (0x%08X)\n", twiddledSize, twiddledSize)
		}
		// SizeOfLibidTwiddled (4 bytes)
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for SizeOfLibidTwiddled")
		}
		libidSize := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		if outputCallback != nil {
			outputCallback("      SizeOfLibidTwiddled: %d (0x%08X)\n", libidSize, libidSize)
		}

		// Sanity check: libidSize should be reasonable
		if libidSize > uint32(len(data)-offset) {
			return nil, offset, fmt.Errorf("invalid LibidTwiddled size: %d (exceeds remaining data or too large, remaining data size should be at most %d)", libidSize, len(data)-offset)
		}

		// LibidTwiddled (variable)
		if offset+int(libidSize) > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for LibidTwiddled (size: %d)", libidSize)
		}
		libidTwiddled := string(data[offset : offset+int(libidSize)])
		offset += int(libidSize)
		if outputCallback != nil {
			outputCallback("      LibidTwiddled:      %q\n", libidTwiddled)
		}
		ref.LibidTwiddled = libidTwiddled

		// Reserved1 (4 bytes), must be 0x00000000
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for Reserved1")
		}
		reserved1 := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		if reserved1 != 0x00000000 {
			return nil, offset, fmt.Errorf("invalid Reserved1 value: 0x%08X (expected 0x00000000)", reserved1)
		}
		if outputCallback != nil {
			outputCallback("      Reserved1:          0x%08X\n", reserved1)
		}

		// Reserved2 (2 bytes), must be 0x0000
		if offset+2 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for Reserved2")
		}
		reserved2 := binary.LittleEndian.Uint16(data[offset:])
		offset += 2
		if reserved2 != 0x0000 {
			return nil, offset, fmt.Errorf("invalid Reserved2 value: 0x%04X (expected 0x0000)", reserved2)
		}
		if outputCallback != nil {
			outputCallback("      Reserved2:          0x%04X\n", reserved2)
		}

		// NameRecordExtended (variable): A REFERENCENAME Record (section 2.3.4.2.2.2) that specifies the name of the
		// extended type library. This field is optional.
		possibleExtRecordId := binary.LittleEndian.Uint16(data[offset:])
		if possibleExtRecordId == 0x0016 {
			ref.RefNameExtended, offset, err = parseReferenceNameRecord(data, offset, outputCallback, refNum, true)
			if err != nil {
				return nil, offset, fmt.Errorf("unexpected error during parsing NameRecordExtended: %v", err)
			}
		}

		// Reserved3 (2 bytes), must be 0x0030
		if offset+2 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for Reserved3")
		}
		reserved3 := binary.LittleEndian.Uint16(data[offset:])
		offset += 2
		if reserved3 != 0x0030 {
			return nil, offset, fmt.Errorf("invalid Reserved3 value: 0x%04X (expected 0x0030)", reserved3)
		}
		if outputCallback != nil {
			outputCallback("      Reserved3:          0x%08X\n", reserved3)
		}

		// SizeOfNameExtended (4 bytes)
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for SizeOfNameExtended")
		}
		nameExtSize := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		if outputCallback != nil {
			outputCallback("      SizeOfNameExtended: %d (0x%08X)\n", nameExtSize, nameExtSize)
		}

		// SizeOfLibidExtended (4 bytes)
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for SizeOfLibidExtended")
		}
		libidExtSize := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		if outputCallback != nil {
			outputCallback("      SizeOfLibidExtended: %d (0x%08X)\n", libidExtSize, libidExtSize)
		}

		// LibidExtended (variable)
		if libidExtSize > 0 {
			if offset+int(libidExtSize) > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for LibidExtended (size: %d)", libidExtSize)
			}
			ref.LibidExtended = string(data[offset : offset+int(libidExtSize)])
			offset += int(libidExtSize)
			if outputCallback != nil {
				outputCallback("      LibidExtended:      %q\n", ref.LibidExtended)
			}
		} else {
			if outputCallback != nil {
				outputCallback("      LibidExtended:      (empty)\n")
			}
		}

		// Reserved4 (4 bytes)
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for Reserved4")
		}
		reserved4 := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		if reserved4 != 0x00000000 {
			return nil, offset, fmt.Errorf("invalid Reserved4 value: 0x%08X (expected 0x00000000)", reserved4)
		}
		if outputCallback != nil {
			outputCallback("      Reserved4:          0x%08X\n", reserved4)
		}

		// Reserved5 (2 bytes) - MUST be 0x0000
		if offset+2 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for Reserved5")
		}
		reserved5 := binary.LittleEndian.Uint16(data[offset:])
		offset += 2
		if reserved5 != 0x0000 {
			return nil, offset, fmt.Errorf("invalid Reserved5 value: 0x%04X (expected 0x0000)", reserved5)
		}
		if outputCallback != nil {
			outputCallback("      Reserved5:          0x%04X\n", reserved5)
		}

		// OriginalTypeLib (16 bytes)
		if offset+16 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for OriginalTypeLib")
		}

		originalTypeLib := data[offset : offset+16]

		var tmpOriginalTypeLib [16]byte
		copy(tmpOriginalTypeLib[:], originalTypeLib)
		ref.OriginalTypeLib = tmpOriginalTypeLib

		offset += 16
		if outputCallback != nil {
			outputCallback("      OriginalTypeLib:    ")
			for i, b := range originalTypeLib {
				if i > 0 {
					outputCallback(":")
				}
				outputCallback("%02X", b)
			}
			outputCallback("\n")
		}

		// Cookie (4 bytes)
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for Cookie")
		}
		cookie := binary.LittleEndian.Uint16(data[offset:])
		offset += 4
		if outputCallback != nil {
			outputCallback("      Cookie:             0x%08X\n", cookie)
		}

	case 0x000D: // REFERENCEREGISTERED
		if outputCallback != nil {
			outputCallback("    REFERENCEREGISTERED sub-records:\n")
		}

		// Size (4 bytes): An unsigned integer that specifies the total size in bytes of SizeOfLibid, Libid, Reserved1, and Reserved2.
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for Size")
		}
		fullSize := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		if outputCallback != nil {
			outputCallback("      Size:                %d (0x%08X)\n", fullSize, fullSize)
		}

		// SizeOfLibid (4 bytes)
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for SizeOfLibid")
		}
		libidSize := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		if outputCallback != nil {
			outputCallback("      SizeOfLibid:         %d (0x%08X)\n", libidSize, libidSize)
		}

		// Libid (variable)
		if offset+int(libidSize) > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for Libid (size: %d)", libidSize)
		}
		libid := string(data[offset : offset+int(libidSize)])
		ref.Libid = libid
		offset += int(libidSize)
		if outputCallback != nil {
			outputCallback("      Libid:               %q\n", libid)
		}

		// Reserved1 (4 bytes)
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for Reserved1")
		}
		reserved1 := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		if reserved1 != 0x00000000 {
			return nil, offset, fmt.Errorf("invalid Reserved1 value: 0x%08X (expected 0x00000000)", reserved1)
		}
		if outputCallback != nil {
			outputCallback("      Reserved1:          0x%08X\n", reserved1)
		}

		// Reserved2 (2 bytes) - MUST be 0x0000
		if offset+2 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for Reserved2")
		}
		reserved2 := binary.LittleEndian.Uint16(data[offset:])
		offset += 2
		if outputCallback != nil {
			outputCallback("      Reserved2:          0x%04X (MUST be 0x0000)\n", reserved2)
		}
		if reserved2 != 0x0000 {
			if outputCallback != nil {
				outputCallback("      ⚠️  Warning: Reserved2 is 0x%04X, expected 0x0000\n", reserved2)
			}
		}

	case 0x000E: // REFERENCEPROJECT
		if outputCallback != nil {
			outputCallback("    REFERENCEPROJECT sub-records:\n")
		}

		// Size (4 bytes)
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for Size")
		}
		fullSize := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		if outputCallback != nil {
			outputCallback("      Size:                %d (0x%08X)\n", fullSize, fullSize)
		}

		// SizeOfLibidAbsolute (4 bytes)
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for SizeOfLibidAbsolute")
		}
		libidSize := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		if outputCallback != nil {
			outputCallback("      SizeOfLibidAbsolute: %d (0x%08X)\n", libidSize, libidSize)
		}

		// LibidAbsolute (variable)
		if offset+int(libidSize) > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for LibidAbsolute (size: %d)", libidSize)
		}
		libidAbsolute := string(data[offset : offset+int(libidSize)])
		ref.LibidAbsolute = libidAbsolute
		offset += int(libidSize)
		if outputCallback != nil {
			outputCallback("      LibidAbsolute:       %q\n", libidAbsolute)
		}

		// SizeOfLibidRelative (4 bytes)
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for SizeOfLibidRelative")
		}
		libidRelSize := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		if outputCallback != nil {
			outputCallback("      SizeOfLibidRelative: %d (0x%08X)\n", libidRelSize, libidRelSize)
		}

		// LibidRelative (variable)
		if libidRelSize > 0 {
			if offset+int(libidRelSize) > len(data) {
				return nil, offset, fmt.Errorf("insufficient data for LibidRelative (size: %d)", libidRelSize)
			}
			ref.LibidRelative = string(data[offset : offset+int(libidRelSize)])
			offset += int(libidRelSize)
			if outputCallback != nil {
				outputCallback("      LibidRelative:       %q\n", ref.LibidRelative)
			}
		} else {
			if outputCallback != nil {
				outputCallback("      LibidRelative:       (empty)\n")
			}
		}

		// MajorVersion (4 bytes)
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for MajorVersion")
		}
		ref.MajorVersion = binary.LittleEndian.Uint32(data[offset:])
		offset += 2
		if outputCallback != nil {
			outputCallback("      MajorVersion:        %d\n", ref.MajorVersion)
		}

		// MinorVersion (2 bytes)
		if offset+2 > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for MinorVersion")
		}
		ref.MinorVersion = binary.LittleEndian.Uint16(data[offset:])
		offset += 2
		if outputCallback != nil {
			outputCallback("      MinorVersion:        %d\n", ref.MinorVersion)
		}

	default:
		nextRecordId := binary.LittleEndian.Uint16(data[offset:])
		if nextRecordId == 0x000F {
			// next record is PROJECTMODULES record
			offset -= 2
			if outputCallback != nil {
				outputCallback("=== End of PROJECTREFERENCES (transitioning to PROJECTMODULES) ===\n")
			}
			return nil, offset, nil
		} else {
			if outputCallback != nil {
				outputCallback("Unknown next REFERENCE record Id found: 0x%04X\n", nextRecordId)
				return nil, offset, fmt.Errorf("unknown REFERENCE record Id: 0x%04X", nextRecordId)
			}
		}
	}

	if outputCallback != nil {
		parsedSize := offset - startOffset
		outputCallback("  [Reference %d parsed: %d bytes]\n", refNum, parsedSize)
	}

	return ref, offset, nil
}

func parseReferenceNameRecord(data []byte, offset int, outputCallback ParseOutputCallback, refNum int, isExtended bool) (*DirReferenceName, int, error) {
	startOffset := offset
	dirRefName := &DirReferenceName{}

	// Step 1: Parse REFERENCENAME (0x0016)
	if offset+2 > len(data) {
		return nil, offset, fmt.Errorf("insufficient data for REFERENCENAME Id")
	}
	nameRecordId := binary.LittleEndian.Uint16(data[offset:])
	if nameRecordId != 0x0016 {
		return nil, offset, fmt.Errorf("expected REFERENCENAME (0x0016), got 0x%04X", nameRecordId)
	}
	offset += 2
	dirRefName.ReferenceType = nameRecordId

	// Read SizeOfName (4 bytes)
	if offset+4 > len(data) {
		return nil, offset, fmt.Errorf("insufficient data for SizeOfName")
	}
	nameSize := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	if outputCallback != nil {
		outputCallback("  REFERENCENAME (0x0016):\n")
		outputCallback("    IsExtended: %v\n", isExtended)
		outputCallback("    SizeOfName:   %d (0x%08X)\n", nameSize, nameSize)
	}

	// Name (variable)
	if nameSize > 0 {
		if offset+int(nameSize) > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for Name (size: %d)", nameSize)
		}
		dirRefName.Name = string(data[offset : offset+int(nameSize)])
		offset += int(nameSize)
		if outputCallback != nil {
			outputCallback("    Name:          %q\n", dirRefName.Name)
		}
	} else {
		if outputCallback != nil {
			outputCallback("    Name:          (empty)\n")
		}
	}

	// Reserved (2 bytes) - MUST be 0x003E
	if offset+2 > len(data) {
		return nil, offset, fmt.Errorf("insufficient data for Reserved")
	}
	reserved := binary.LittleEndian.Uint16(data[offset:])
	offset += 2
	if outputCallback != nil {
		outputCallback("    Reserved:      0x%04X (MUST be 0x003E)\n", reserved)
	}
	if reserved != 0x003E {
		// According to spec, MUST be 0x003E, but we'll just warn and continue
		if outputCallback != nil {
			outputCallback("    ⚠️  Warning: Reserved field is 0x%04X, expected 0x003E\n", reserved)
		}
	}

	// SizeOfNameUnicode (4 bytes)
	if offset+4 > len(data) {
		return nil, offset, fmt.Errorf("insufficient data for SizeOfNameUnicode")
	}
	nameUnicodeSize := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	if outputCallback != nil {
		outputCallback("    SizeOfNameUnicode: %d (0x%08X)\n", nameUnicodeSize, nameUnicodeSize)
	}

	// NameUnicode (variable)
	if nameUnicodeSize > 0 {
		if offset+int(nameUnicodeSize) > len(data) {
			return nil, offset, fmt.Errorf("insufficient data for NameUnicode (size: %d)", nameUnicodeSize)
		}
		unicodeBytes := data[offset : offset+int(nameUnicodeSize)]
		nameUnicode := decodeUTF16LE(unicodeBytes)
		offset += int(nameUnicodeSize)
		if outputCallback != nil {
			outputCallback("    NameUnicode:    %q\n", nameUnicode)
		}
	} else {
		if outputCallback != nil {
			outputCallback("    NameUnicode:    (empty)\n")
		}
	}

	if outputCallback != nil {
		parsedSize := offset - startOffset
		outputCallback("  [Extended: %v, ReferenceName %d parsed: %d bytes]\n", isExtended, refNum, parsedSize)
	}

	return dirRefName, offset, nil
}
