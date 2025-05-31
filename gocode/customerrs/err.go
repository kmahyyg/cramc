package customerrs

import (
	"errors"
)

var (
	ErrInsufficientPrivilege = errors.New("insufficient privileges")
	ErrUnsupportedPlatform   = errors.New("unsupported platform")
	ErrUnknownInternalError  = errors.New("unknown internal error")
	ErrDecryptionFailed      = errors.New("decryption failed, integrity check failed")
)
