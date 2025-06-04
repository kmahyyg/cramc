package common

import (
	"github.com/sirupsen/logrus"
	"runtime"
	"sync"
)

var (
	Logger *logrus.Logger

	CleanupDB *CRAMCCleanupDB

	IsRunningOnWin = runtime.GOOS == "windows"

	RPCCleanerHash string
	VersionStr     string

	RPCHandlingStatus string
	RPCHandlingQueue  = make(chan *IPC_SingleDocToBeSanitized)
	RPCServerSecret   string
	RPCServerListen   string

	DryRunOnly                 bool
	EnableHardening            bool
	HardeningQueue             = make(chan *HardeningAction)
	HardenedDetectionTypesLock = &sync.Mutex{}
	HardenedDetectionTypes     []string
)

const (
	// it's insecure, don't hardcode any password, but i'm lazy, so here, it's intended.
	HexEncryptionPassword = "1928da3545b48068e024d06f2f132c728eabcd933a8659e578d7a82fde0cd948"
	ProgramRev            = 3
)
