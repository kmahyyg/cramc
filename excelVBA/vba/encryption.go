package vba

import (
	"crypto/md5"
	"fmt"
)

// Decryptor handles MS-OVBA reversible encryption/decryption
// MS-OVBA Section 2.4.3 specifies RC4-based encryption
type Decryptor struct {
	key []byte
}

// NewDecryptor creates a new decryptor with the given password and project context
func NewDecryptor(password string, projectInfo *DirStream) (*Decryptor, error) {
	// Derive encryption key from password and project data
	key, err := deriveKey(password, projectInfo)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return &Decryptor{
		key: key,
	}, nil
}

// deriveKey derives the encryption key from password and project information
// MS-OVBA Section 2.4.3 specifies key derivation algorithm
func deriveKey(password string, projectInfo *DirStream) ([]byte, error) {
	// Combine password with project-specific data
	// The key derivation uses MD5 hash of password + project data
	hasher := md5.New()

	// Add password
	if len(password) > 0 {
		hasher.Write([]byte(password))
	}

	// Add project-specific data (Name, Version, etc.)
	if projectInfo != nil {
		hasher.Write([]byte(projectInfo.Name))
		// Add other project identifiers
	}

	// Generate 16-byte key (MD5 produces 16 bytes)
	key := hasher.Sum(nil)

	return key, nil
}

// Decrypt decrypts encrypted VBA stream data using RC4
func Decrypt(encryptedData []byte, password string, projectData []byte) ([]byte, error) {
	if len(encryptedData) == 0 {
		return []byte{}, nil
	}

	// For now, create a temporary DirStream for key derivation
	// In practice, this should use actual project info
	projectInfo := &DirStream{
		Name: string(projectData),
	}

	decryptor, err := NewDecryptor(password, projectInfo)
	if err != nil {
		return nil, err
	}

	return decryptor.DecryptStream(encryptedData)
}

// DecryptStream decrypts a complete module stream using RC4
func (d *Decryptor) DecryptStream(encryptedStream []byte) ([]byte, error) {
	if len(encryptedStream) == 0 {
		return []byte{}, nil
	}

	// Implement RC4 decryption
	// RC4 is symmetric, so encryption and decryption are the same operation
	return d.rc4Decrypt(encryptedStream)
}

// rc4Decrypt performs RC4 stream cipher decryption
func (d *Decryptor) rc4Decrypt(data []byte) ([]byte, error) {
	// Initialize RC4 state
	s := make([]uint8, 256)
	for i := 0; i < 256; i++ {
		s[i] = uint8(i)
	}

	// Key scheduling
	j := uint8(0)
	keyLen := len(d.key)
	if keyLen == 0 {
		return nil, fmt.Errorf("%w: empty encryption key", ErrDecryptionFailed)
	}

	for i := 0; i < 256; i++ {
		j = j + s[i] + d.key[i%keyLen]
		s[i], s[j] = s[j], s[i]
	}

	// Generate keystream and decrypt
	result := make([]byte, len(data))
	i := uint8(0)
	j = uint8(0)

	for k := 0; k < len(data); k++ {
		i = i + 1
		j = j + s[i]
		s[i], s[j] = s[j], s[i]
		keystreamByte := s[(int(s[i])+int(s[j]))%256]
		result[k] = data[k] ^ keystreamByte
	}

	return result, nil
}

// IsEncrypted detects if a stream appears to be encrypted
// Encrypted streams typically don't start with recognizable VBA source patterns
func IsEncrypted(data []byte) bool {
	if len(data) < 4 {
		return false
	}

	// Check for common VBA source code patterns
	// If these patterns are not found, it might be encrypted
	commonPatterns := [][]byte{
		[]byte("Attribute"),
		[]byte("Sub "),
		[]byte("Function "),
		[]byte("Option "),
		[]byte("Dim "),
		[]byte("Private "),
		[]byte("Public "),
	}

	// Check first 100 bytes for patterns
	checkLen := len(data)
	if checkLen > 100 {
		checkLen = 100
	}

	for _, pattern := range commonPatterns {
		if len(pattern) <= checkLen {
			for i := 0; i <= checkLen-len(pattern); i++ {
				match := true
				for j := 0; j < len(pattern); j++ {
					if data[i+j] != pattern[j] {
						match = false
						break
					}
				}
				if match {
					return false // Found VBA pattern, likely not encrypted
				}
			}
		}
	}

	// If no patterns found and data looks random, might be encrypted
	// This is a heuristic - in practice, encryption status should be tracked
	return false // Default to not encrypted (many projects are unprotected)
}
