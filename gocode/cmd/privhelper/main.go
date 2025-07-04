//go:build windows

//go:generate go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
//go:generate protoc --go_out=./sanitizer_ole/pbrpc --proto_path=protos --go_opt=paths=source_relative --go-grpc_out=./sanitizer_ole/pbrpc --go-grpc_opt=paths=source_relative excel_unpriv_rpc.proto

package main

import (
	"context"
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/fileutils"
	"cramc_go/logging"
	"cramc_go/platform/windoge_utils"
	"cramc_go/sanitizer_ole"
	"cramc_go/sanitizer_ole/pbrpc"
	"cramc_go/telemetry"
	"errors"
	"github.com/Microsoft/go-winio"
	"github.com/go-ole/go-ole"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"os/signal"
	"os/user"
	"sync"
	"syscall"
	"time"
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
	common.Logger.Info("Welcome to CRAMC Privilege Helper RPC Server!")
	common.Logger.Info("Current Version: " + common.VersionStr)

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

	// create lock and listener
	func() {
		lockFd, err2 := os.Create(lockFile)
		if err2 != nil {
			panic(err2)
		}
		defer lockFd.Close()
	}()
	// cleanup
	defer os.Remove(lockFile)

	// start initialization of server and excelWorker
	// -------- initialize excel worker -------- //
	// enable scripting access to VBAObject Model
	err = sanitizer_ole.LiftVBAScriptingAccess("16.0", "Excel")
	if err != nil {
		common.Logger.Error(err.Error())
		return
	}
	// kill all office processes, to avoid any potential file lock.
	_, _ = windoge_utils.KillAllOfficeProcesses()
	common.Logger.Info("Triggered M365 Office processes killer.")
	// prepare to call ole
	err = ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err != nil {
		common.Logger.Error(err.Error())
		return
	}
	defer ole.CoUninitialize()
	// new approach: bundled
	inDebugging := false
	if data, ok := os.LookupEnv("RunEnv"); ok {
		if data == "DEBUG" {
			inDebugging = true
		}
	}
	eWorker := &sanitizer_ole.ExcelWorker{}
	err = eWorker.Init(inDebugging)
	if err != nil {
		common.Logger.Error("Failed to initialize excel worker:" + err.Error())
		return
	}
	defer eWorker.Quit(false)
	err = eWorker.GetWorkbooks()
	if err != nil {
		common.Logger.Error("Failed to get workbooks:" + err.Error())
		return
	}
	common.Logger.Info("Excel.Application worker initialized.")

	// listen on named pipe
	wPipe, err := winio.ListenPipe(sanitizer_ole.RpcPipeAddr, &winio.PipeConfig{
		InputBufferSize:  65536,
		OutputBufferSize: 65536,
	})
	if err != nil {
		common.Logger.Log(context.TODO(), logging.LevelFatal, "Failed to listen on pipe: "+err.Error())
		os.Exit(-1)
		return
	}

	// prepare for simpleGRPC
	quitMsgChan := make(chan struct{}, 1)
	quitMsgOnce := &sync.Once{}
	extWgWorkerGroup := &sync.WaitGroup{}
	sGRPCsrv, err := sanitizer_ole.InitSimpleRPCServer(eWorker, quitMsgChan, quitMsgOnce, extWgWorkerGroup)
	if err != nil {
		common.Logger.Error("Failed to initialize simple RPC server:" + err.Error())
		return
	}

	// start RPC server
	var servOpts = []grpc.ServerOption{grpc.Creds(insecure.NewCredentials())}
	gRSrv := grpc.NewServer(servOpts...)
	pbrpc.RegisterExcelSanitizerRPCServer(gRSrv, sGRPCsrv)
	go func() {
		if err3 := gRSrv.Serve(wPipe); err3 != nil && !errors.Is(err3, grpc.ErrServerStopped) {
			common.Logger.Error("GRPC Server Listen Returned Error:" + err3.Error())
			return
		}
		common.Logger.Info("GRPC Server Stopped.")
	}()

	// graceful shutdown
	osSignals := make(chan os.Signal, 1)
	// exit cleanup func
	waitDChan := make(chan struct{}, 1)
	exitCleanupF := func() {
		go func() {
			gRSrv.GracefulStop()
			extWgWorkerGroup.Wait()
			close(waitDChan)
		}()
		select {
		case <-waitDChan:
			common.Logger.Info("All goroutines stopped gracefully")
		case <-time.After(210 * time.Second):
			common.Logger.Warn("Timed out for 210 seconds, directly exit.")
			gRSrv.Stop()
			common.Logger.Warn("Forced Stop GRPC Server.")
		}
	}

	signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)

	// either system stop or actively stop
	select {
	case <-osSignals:
		common.Logger.Info("Received OS Signal, shutting down")
		quitMsgOnce.Do(func() {
			close(quitMsgChan)
		})
		exitCleanupF()
	case <-quitMsgChan:
		common.Logger.Info("Received QUIT control, shutting down")
		exitCleanupF()
	}
}
