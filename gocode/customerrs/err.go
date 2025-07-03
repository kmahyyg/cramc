package customerrs

import (
	"errors"
)

var (
	ErrInsufficientPrivilege = errors.New("insufficient privileges")
	ErrUnsupportedPlatform   = errors.New("unsupported platform")

	ErrUnknownInternalError = errors.New("unknown internal error")
	ErrDecryptionFailed     = errors.New("decryption failed, integrity check failed")

	ErrDeviceInaccessible = errors.New("disk device is inaccessible")

	ErrInvalidInput                 = errors.New("invalid input")
	ErrFallbackToCompatibleSolution = errors.New("cannot using boosted solution, fallback")

	ErrActionPathMustBeDir   = errors.New("actionPath must be a path to directory")
	ErrFileExistsOnCloudOnly = errors.New("current file only exists on cloud, not on local disk")

	ErrNoScanSetButNoListProvided = errors.New("noDiskScan set, but never provided scan result input list")

	ErrNotLatestVersion = errors.New("not latest version, refuse to continue, please upgrade from https://github.com/kmahyyg/cramc")

	ErrExcelWorkerUninitialized    = errors.New("excel worker object failed to initialize")
	ErrExcelWorkbooksUnable2Fetch  = errors.New("excel workbooks handle is failed to fetch")
	ErrExcelCurrentWorkbookNullPtr = errors.New("excel current workbook pointer is nil")
	ErrExcelNoMacroFound           = errors.New("no macro found in current workbook")

	ErrYaraXCompilationFailure = errors.New("yara-x rule compilation failed")

	ErrTelemetryMustBeInitedFirst = errors.New("telemetry sender must be initialized first")

	ErrRunsOnSystemMachineAccount = errors.New("runs under NT AUTHORITY\\SYSTEM account, refuse to continue")
	ErrPrivHelperLockExists       = errors.New("privhelper lock exists, if you believe it's wrong, please delete lock file")

	ErrRpcConnectionNotEstablished = errors.New("rpc connection not established")
	ErrRpcResponseUnexpected       = errors.New("rpc response unexpected")
)
