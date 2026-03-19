package vba

import "errors"

// Error types for VBA file handling
var (
	// Stream parsing errors
	ErrInvalidProjectFormat    = errors.New("invalid PROJECT stream format")
	ErrInvalidProjectWM        = errors.New("invalid PROJECTwm stream format")
	ErrInvalidDirFormat        = errors.New("invalid dir stream format")
	ErrInvalidVBAProjectFormat = errors.New("invalid _VBA_PROJECT stream format")
	ErrModuleNotFound          = errors.New("module not found")
	ErrStreamNotFound          = errors.New("VBA stream not found")

	// Compression errors
	ErrDecompressionFailed = errors.New("failed to decompress dir stream")

	// Encryption errors
	ErrDecryptionFailed = errors.New("failed to decrypt encrypted stream")
	ErrPasswordRequired = errors.New("password required for encrypted module")
	ErrInvalidPassword  = errors.New("invalid password for decryption")
)
