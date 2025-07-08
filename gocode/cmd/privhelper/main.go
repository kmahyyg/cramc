//go:build windows

//go:generate go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
//go:generate protoc --go_out=../../sanitizer_ole/pbrpc --proto_path=../../protos --go_opt=paths=source_relative --go-grpc_out=../../sanitizer_ole/pbrpc --go-grpc_opt=paths=source_relative excel_unpriv_rpc.proto

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
	"fmt"
	"github.com/Microsoft/go-winio"
	"github.com/go-ole/go-ole"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"net"
	"os"
	"os/signal"
	"os/user"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	betterStackURL         = "https://s1358347.eu-nbg-2.betterstackdata.com"
	betterStackBearerToken = "26Y9ahkqDMsQgLN9yTb1JETU"

	lockFile        = "privhelper.lock"
	listenWinIOPipe = `\\.\pipe\cramcPriv`
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

	// panic capture
	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			telemetry.CaptureMessage("panic", fmt.Sprintf("Panic on [%v], stacktrace: %s", r, string(debug.Stack())))
			os.Exit(1)
		}
	}()

	// check if debug
	inDebugging := false
	if data := os.Getenv("RunEnv"); data == "DEBUG" || data == "NOSPAWN" {
		inDebugging = true
	}

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
	// call killer in defer again to avoid locking with existing file
	defer func() { _, _ = windoge_utils.KillAllOfficeProcesses() }()

	// prepare to spawn
	var parentWg = &sync.WaitGroup{}
	var workerTrack = &sync.Map{}
	var stopSign = &atomic.Bool{}
	stopSign.Store(false)
	var jobQueue = make(chan *common.IPCSingleDocToBeSanitized, 50)
	// determine worker number
	cpuCores := runtime.NumCPU()
	maxWorker := int(cpuCores/2) + 1
	common.Logger.Info(fmt.Sprintf("Detected %d CPU cores, will spawn %d workers. ", cpuCores, maxWorker))
	// output error to channel async
	var retErrCh = make(chan error, 50)
	go func() {
		if e, ok := <-retErrCh; ok {
			if e != nil {
				telemetry.CaptureException(e, "ExcelWorker.HandleIncomingTasks.errC")
				common.Logger.Error("HandleJob.CleanupProcedure returned: " + e.Error())
			} else {
				common.Logger.Info("HandleJob.CleanupProcedure successfully returned. ")
			}
		}
	}()

	for i := 0; i < maxWorker; i++ {
		// spawn 3 workers maximum to avoid race condition
		parentWg.Add(1)
		go func() {
			// worker done
			defer parentWg.Done()
			// handling thread-local state issue
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			err = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED|ole.COINIT_DISABLE_OLE1DDE)
			if err != nil {
				telemetry.CaptureException(err, "ole.CoInitializeEx.WorkerThread")
				panic(err)
			}
			defer ole.CoUninitialize()
			// build worker and save to tracker map
			eWorker := &sanitizer_ole.ExcelWorker{}
			workerTrack.Store(i, eWorker)
			// common initialization
			err = eWorker.Init(inDebugging, stopSign)
			if err != nil {
				telemetry.CaptureException(err, "eWorkerInit.WorkerThread")
				panic(err)
			}
			err = eWorker.GetWorkbooks()
			if err != nil {
				common.Logger.Error("Failed to get workbooks:" + err.Error())
				return
			}
			common.Logger.Info("Excel.Application worker initialized.")
			defer eWorker.Quit()
			// start processing and wait for termination signal
			eWorker.HandleIncomingTasks(jobQueue, retErrCh)
		}()
	}

	// prepare for simpleGRPC
	quitMsgChan := make(chan struct{}, 1)
	quitMsgOnce := &sync.Once{}
	sGRPCsrv, err := sanitizer_ole.InitSimpleRPCServer(jobQueue, quitMsgChan, quitMsgOnce, stopSign)
	if err != nil {
		common.Logger.Error("Failed to initialize simple RPC server:" + err.Error())
		return
	}

	// start RPC server
	var servOpts = []grpc.ServerOption{grpc.Creds(insecure.NewCredentials())}
	gRSrv := grpc.NewServer(servOpts...)
	pbrpc.RegisterExcelSanitizerRPCServer(gRSrv, sGRPCsrv)
	var fListener net.Listener
	defer func() {
		if fListener != nil {
			common.Logger.Info("Closing listener in defer func as cleanup.")
			_ = fListener.Close()
		}
	}()
	go func() {
		if os.Getenv("RunEnv") == "DEBUG" {
			// enable service reflection
			// https://github.com/grpc/grpc-go/blob/master/Documentation/server-reflection-tutorial.md#enable-server-reflection
			reflection.Register(gRSrv)
			// listen on tcp, call using `grpcurl -plaintext`
			fListener, err = net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				common.Logger.Info("Failed to listen on tcp: " + err.Error())
				return
			}
			common.Logger.Info("RunEnv==DEBUG detected, listen on TCP: " + fListener.Addr().String() + " for debugging.")
			if err3 := gRSrv.Serve(fListener); err3 != nil && !errors.Is(err3, grpc.ErrServerStopped) {
				common.Logger.Error("GRPC Server Listen Returned Error:" + err3.Error())
				return
			}
		} else {
			// listen on named pipe
			fListener, err = winio.ListenPipe(listenWinIOPipe, &winio.PipeConfig{
				InputBufferSize:  65536,
				OutputBufferSize: 65536,
			})
			if err != nil {
				common.Logger.Log(context.TODO(), logging.LevelFatal, "Failed to listen on pipe: "+err.Error())
				os.Exit(-1)
				return
			}
			common.Logger.Info("Listening on named pipe: " + listenWinIOPipe + " for RPC.")
			if err3 := gRSrv.Serve(fListener); err3 != nil && !errors.Is(err3, grpc.ErrServerStopped) {
				common.Logger.Error("GRPC Server Listen Returned Error:" + err3.Error())
				return
			}
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
			for {
				if len(jobQueue) > 0 {
					common.Logger.Info("There's still jobs in queue waiting to be processed. Wait for 10 seconds.")
					time.Sleep(10 * time.Second)
				} else {
					break
				}
			}
			// wait for group process
			parentWg.Wait()
			workerTrack.Clear()
			close(retErrCh)
			close(waitDChan)
		}()
		// always wait, no timeout
		<-waitDChan
		// wait done, safe to terminate grpc coroutine
		gRSrv.Stop()
		common.Logger.Info("All goroutines stopped correctly. Now exit.")
	}

	signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)

	// either system stop or actively stop
	select {
	case <-osSignals:
		common.Logger.Info("Received OS Signal, shutting down")
		quitMsgOnce.Do(func() {
			close(jobQueue)
			close(quitMsgChan)
		})
		exitCleanupF()
	case <-quitMsgChan:
		common.Logger.Info("Received QUIT control, shutting down")
		exitCleanupF()
	}
}
