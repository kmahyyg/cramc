package vba

import (
	"encoding/hex"
	"fmt"
)

// EncryptedData represents the decrypted structure of CMG, DPB, or GC fields
// Based on MS-OVBA section 2.4.3.1: Encrypted Data Structure
type EncryptedData struct {
	Seed      byte
	Version   byte
	ProjKey   byte
	Ignored   []byte
	DataLength uint32
	Data      []byte
}

// DecryptProjectField decrypts/decodes an encrypted project field (CMG, DPB, or GC)
// The input is a hex string like "EEEC504BD0CDB5D1B5D1B5D1B5D1"
// Based on MS-OVBA section 2.4.3: Data Encryption
// Implements the stateful XOR decryption algorithm from MS-OVBA 2.4.3
//
// Note: This function interprets the obfuscated encryption status fields from the
// PROJECT stream. It does NOT decrypt module content streams, which require a password
// and use RC4 encryption (see encryption.go for module stream decryption).
func DecryptProjectField(hexString string) (*EncryptedData, error) {
	// Remove quotes if present
	hexString = trimQuotes(hexString)
	
	// Decode hex string to bytes
	encryptedData, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex string: %w", err)
	}

	if len(encryptedData) < 3 {
		return nil, fmt.Errorf("encrypted data too short: %d bytes (minimum 3)", len(encryptedData))
	}

	// The first byte is the Seed (not encrypted)
	seed := encryptedData[0]
	
	// Calculate Version: Version = Seed XOR VersionEnc
	// VersionEnc is at position 1
	versionEnc := encryptedData[1]
	version := seed ^ versionEnc
	
	// Version must be 2 according to MS-OVBA
	if version != 2 {
		return nil, fmt.Errorf("invalid encryption version: expected 2, got %d", version)
	}
	
	// Calculate ProjKey: ProjKey = Seed XOR ProjKeyEnc
	// ProjKeyEnc is at position 2
	projKeyEnc := encryptedData[2]
	projKey := seed ^ projKeyEnc

	// Initialize state variables for decryption
	// UnencryptedByte1 = ProjKey (last unencrypted byte)
	// EncryptedByte1 = ProjKeyEnc (last encrypted byte)
	// EncryptedByte2 = VersionEnc (next-to-last encrypted byte)
	unencryptedByte1 := projKey
	encryptedByte1 := projKeyEnc
	encryptedByte2 := versionEnc

	// Calculate IgnoredLength: IgnoredLength = (Seed BAND 6) / 2
	ignoredLength := int((seed & 6) / 2)
	
	// Minimum structure requires: Seed(1) + VersionEnc(1) + ProjKeyEnc(1) + Ignored + DataLength(4) + Data
	pos := 3 // Start after Seed, VersionEnc, ProjKeyEnc
	
	// Decrypt IgnoredEnc bytes
	ignored := make([]byte, ignoredLength)
	for i := 0; i < ignoredLength; i++ {
		if pos >= len(encryptedData) {
			return nil, fmt.Errorf("data too short for Ignored field at position %d", pos)
		}
		
		// Byte = ByteEnc XOR (EncryptedByte2 + UnencryptedByte1)
		byteEnc := encryptedData[pos]
		ignored[i] = byteEnc ^ (encryptedByte2 + unencryptedByte1)
		
		// Update state: move EncryptedByte2 to EncryptedByte1, and current byte to EncryptedByte2
		encryptedByte2 = encryptedByte1
		encryptedByte1 = byteEnc
		unencryptedByte1 = ignored[i]
		pos++
	}

	// Decrypt DataLengthEnc (4 bytes, little-endian)
	if pos+4 > len(encryptedData) {
		return nil, fmt.Errorf("data too short for DataLength: need 4 bytes at position %d", pos)
	}
	
	dataLength := uint32(0)
	for i := 0; i < 4; i++ {
		byteEnc := encryptedData[pos]
		// Byte = ByteEnc XOR (EncryptedByte2 + UnencryptedByte1)
		byteValue := byteEnc ^ (encryptedByte2 + unencryptedByte1)
		
		// TempValue = 256^ByteIndex * Byte
		tempValue := uint32(byteValue) << (8 * uint(i))
		dataLength += tempValue
		
		// Update state
		encryptedByte2 = encryptedByte1
		encryptedByte1 = byteEnc
		unencryptedByte1 = byteValue
		pos++
	}

	// Decrypt DataEnc
	if pos+int(dataLength) > len(encryptedData) {
		return nil, fmt.Errorf("data too short for Data field: need %d bytes at position %d, have %d", 
			dataLength, pos, len(encryptedData)-pos)
	}
	
	data := make([]byte, dataLength)
	for i := uint32(0); i < dataLength; i++ {
		byteEnc := encryptedData[pos]
		// Byte = ByteEnc XOR (EncryptedByte2 + UnencryptedByte1)
		data[i] = byteEnc ^ (encryptedByte2 + unencryptedByte1)
		
		// Update state
		encryptedByte2 = encryptedByte1
		encryptedByte1 = byteEnc
		unencryptedByte1 = data[i]
		pos++
	}

	return &EncryptedData{
		Seed:      seed,
		Version:   version,
		ProjKey:   projKey,
		Ignored:   ignored,
		DataLength: dataLength,
		Data:      data,
	}, nil
}

// trimQuotes removes surrounding quotes from a string
func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// IsProjectEncrypted determines if a VBA project is encrypted based on decrypted CMG, DPB, and GC values
// Based on MS-OVBA section 2.3.1.15, 2.3.1.16, 2.3.1.17
//
// This function interprets the encryption status from the PROJECT stream fields:
// - CMG (ProjectProtectionState): Indicates if sources are restricted
// - DPB (ProjectPassword): Indicates if a password is set
// - GC (ProjectVisibilityState): Indicates project visibility
//
// Note: This only determines encryption status. Actual module content decryption
// requires a password and uses RC4 (see encryption.go).
func IsProjectEncrypted(cmg, dpb, gc *EncryptedData) (bool, string) {
	if cmg == nil && dpb == nil && gc == nil {
		return false, "No encryption fields found (CMG, DPB, GC missing)"
	}
	
	reasons := []string{}
	isEncrypted := false
	
	// Check CMG (ProjectProtectionState)
	// Data: 0x00000000 means no sources are restricted (not encrypted)
	// Any other value means sources are restricted (encrypted)
	if cmg != nil {
		if len(cmg.Data) >= 4 {
			protectionState := uint32(cmg.Data[0]) |
				uint32(cmg.Data[1])<<8 |
				uint32(cmg.Data[2])<<16 |
				uint32(cmg.Data[3])<<24
			
			if protectionState != 0 {
				isEncrypted = true
				reasons = append(reasons, fmt.Sprintf("ProjectProtectionState (CMG) indicates restricted access: 0x%08X", protectionState))
			} else {
				reasons = append(reasons, "ProjectProtectionState (CMG): No sources restricted (0x00000000)")
			}
		}
	}
	
	// Check DPB (ProjectPassword)
	// Data: 0x00 means no password
	// Any other value means password is set
	if dpb != nil {
		if len(dpb.Data) >= 1 {
			if dpb.Data[0] != 0 {
				isEncrypted = true
				reasons = append(reasons, fmt.Sprintf("ProjectPassword (DPB) is set: 0x%02X", dpb.Data[0]))
			} else {
				reasons = append(reasons, "ProjectPassword (DPB): No password set (0x00)")
			}
		}
	}
	
	// Check GC (ProjectVisibilityState)
	// Data: 0xFF means project is visible
	// 0x00 means project is hidden
	if gc != nil {
		if len(gc.Data) >= 1 {
			if gc.Data[0] == 0xFF {
				reasons = append(reasons, "ProjectVisibilityState (GC): Project is visible (0xFF)")
			} else if gc.Data[0] == 0x00 {
				reasons = append(reasons, "ProjectVisibilityState (GC): Project is hidden (0x00)")
			} else {
				reasons = append(reasons, fmt.Sprintf("ProjectVisibilityState (GC): Unknown state (0x%02X)", gc.Data[0]))
			}
		}
	}
	
	reasonText := "Encryption Status: "
	if isEncrypted {
		reasonText += "ENCRYPTED - "
	} else {
		reasonText += "NOT ENCRYPTED - "
	}
	reasonText += fmt.Sprintf("%s", reasons[0])
	for i := 1; i < len(reasons); i++ {
		reasonText += fmt.Sprintf("; %s", reasons[i])
	}
	
	return isEncrypted, reasonText
}
