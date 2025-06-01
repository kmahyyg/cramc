package customerrs

import (
	"errors"
)

var (
	ErrInsufficientPrivilege        = errors.New("insufficient privileges")
	ErrUnsupportedPlatform          = errors.New("unsupported platform")
	ErrUnknownInternalError         = errors.New("unknown internal error")
	ErrDecryptionFailed             = errors.New("decryption failed, integrity check failed")
	ErrDeviceInaccessible           = errors.New("disk device is inaccessible")
	ErrInvalidInput                 = errors.New("invalid input")
	ErrFallbackToCompatibleSolution = errors.New("cannot using boosted solution, fallback")
)
