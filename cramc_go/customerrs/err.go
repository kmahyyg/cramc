package customerrs

import (
	"errors"
)

var (
	ErrInsufficientPrivilege = errors.New("insufficient privileges")
	ErrUnsupportedPlatform   = errors.New("unsupported platform")

	ErrUnknownInternalError = errors.New("unknown internal error")
	ErrDecryptionFailed     = errors.New("decryption failed, integrity check failed")

	ErrInvalidInput        = errors.New("invalid input")
	ErrOutputAlreadyExists = errors.New("output file already exists, won't overwrite")

	ErrActionPathMustBeDir   = errors.New("actionPath must be a path to directory")
	ErrFileExistsOnCloudOnly = errors.New("current file only exists on cloud, not on local disk")

	ErrNotLatestVersion = errors.New("not latest version, refuse to continue, please upgrade from https://github.com/kmahyyg/cramc")

	ErrYaraXCompilationFailure = errors.New("yara-x rule compilation failed")

	ErrEncSchemaMismatch = errors.New("file version does NOT match bundled encryption schema version")
)
