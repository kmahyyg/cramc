package vba

import "errors"

var (
	ErrDecompressionFailed          = errors.New("vba: decompression failed")
	ErrCompressionFailed            = errors.New("vba: compression failed")
	ErrInvalidSignature             = errors.New("vba: invalid compressed container signature")
	ErrModuleNotFound               = errors.New("vba: module not found")
	ErrDirStreamMalformed           = errors.New("vba: malformed dir stream")
	ErrUnsupportedEncryptedProject  = errors.New("vba: password protected project is unsupported")
	ErrProjectStorageNotFound       = errors.New("vba: project storage not found")
	ErrProjectMetadataStreamMissing = errors.New("vba: project metadata stream missing")
)
