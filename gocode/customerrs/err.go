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
	ErrActionPathMustBeDir          = errors.New("actionPath must be a path to directory")
	ErrFileExistsOnCloudOnly        = errors.New("current file only exists on cloud, not on local disk")
	ErrNoScanSetButNoListProvided   = errors.New("noDiskScan set, but never provided scan result input list")
)
