package common

import (
	"log/slog"
	"runtime"
)

var (
	Logger *slog.Logger

	IsRunningOnWin    = runtime.GOOS == "windows"
	IsElevated        bool
	IsRunningBySYSTEM bool

	VersionStr string

	DryRunOnly bool
)

const (
	// it's insecure, don't hardcode any password, but i'm lazy, so here, it's intended.
	HexEncryptionPassword = "1928da3545b48068e024d06f2f132c728eabcd933a8659e578d7a82fde0cd948"
	ProgramRev            = 17
)
