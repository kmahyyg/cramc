package common

import (
	"log/slog"
	"runtime"
	"sync"
)

var (
	Logger *slog.Logger

	CleanupDB *CRAMCCleanupDB

	IsRunningOnWin = runtime.GOOS == "windows"
	IsElevated     bool

	VersionStr    string
	SanitizeQueue = make(chan *IPCSingleDocToBeSanitized, 100)

	DryRunOnly                 bool
	EnableHardening            bool
	HardeningQueue             = make(chan *HardeningAction, 100)
	HardenedDetectionTypesLock = &sync.Mutex{}
	HardenedDetectionTypes     = make(map[string]bool)
)

const (
	// it's insecure, don't hardcode any password, but i'm lazy, so here, it's intended.
	HexEncryptionPassword = "1928da3545b48068e024d06f2f132c728eabcd933a8659e578d7a82fde0cd948"
	ProgramRev            = 12
)
