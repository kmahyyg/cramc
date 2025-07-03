//go:build windows

package main

import (
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/fileutils"
	"cramc_go/logging"
	"cramc_go/platform/windoge_utils"
	"cramc_go/sanitizer_ole"
	"cramc_go/telemetry"
	"os"
	"os/user"
)

const (
	betterStackURL         = "https://s1358347.eu-nbg-2.betterstackdata.com"
	betterStackBearerToken = "26Y9ahkqDMsQgLN9yTb1JETU"

	lockFile = "privhelper.lock"
)

func main() {
	// init logging
	logger, logfd := logging.NewLogger("cramc_privhelper.log")
	common.Logger = logger
	defer logfd.Close()
	defer logfd.Sync()

	// init telemetry
	telemetry.Init(common.VersionStr + "@priv")
	bsSender := telemetry.NewBetterStackSender(betterStackURL, betterStackBearerToken)
	bsSender.SetDefaultSender()

	// startup behavior
	logger.Infoln("Welcome to CRAMC Privilege Helper RPC Server!")
	logger.Infoln("Current Version: ", common.VersionStr)

	// detect if started as SYSTEM, if yes, abort
	runAsSys, err := windoge_utils.CheckRunningUnderSYSTEM()
	if err != nil {
		telemetry.CaptureException(err, "privHelper.CheckSystemPrivAtMain")
		panic(err)
	}
	if runAsSys {
		panic(customerrs.ErrRunsOnSystemMachineAccount)
	}
	// check current user context
	cUser, err := user.Current()
	if err != nil {
		panic(err)
	}
	telemetry.CaptureMessage("info", "privHelper is running at user: "+cUser.Username)

	// detect lock file
	if fileutils.CheckFileLogicalExists(lockFile) {
		panic(customerrs.ErrPrivHelperLockExists)
	}

	var rServ *sanitizer_ole.RPCServer
	// create lock and listener
	func() {
		lockFd, err2 := os.Create(lockFile)
		if err2 != nil {
			panic(err2)
		}
		defer lockFd.Close()
		// start server
		rServ, err2 = sanitizer_ole.NewRPCServer(sanitizer_ole.RpcCallAddr)
		if err2 != nil {
			panic(err2)
		}
	}()
	// cleanup
	defer os.Remove(lockFile)

	rServ.Start()
}
