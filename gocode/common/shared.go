package common

import "github.com/sirupsen/logrus"

var (
	Logger     *logrus.Logger
	VersionStr string
)

const (
	// it's insecure, don't hardcode any password, but i'm lazy, so here, it's intended.
	HexEncryptionPassword = "1928da3545b48068e024d06f2f132c728eabcd933a8659e578d7a82fde0cd948"
	ProgramRev            = 2
)
