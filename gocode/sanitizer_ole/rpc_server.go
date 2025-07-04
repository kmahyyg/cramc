package sanitizer_ole

import (
	"bufio"
	"context"
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/platform/windoge_utils"
	"cramc_go/telemetry"
	"encoding/json"
	"fmt"
	"github.com/Microsoft/go-winio"
	"github.com/go-ole/go-ole"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	MSG_MAX_SIZE   = 65535
	cleanupComment = `' Sanitized by CRAMC
Private Sub CRAMCPlaceholder()
    ' This ensures the comment above persists
End Sub
`
)

type RPCServer struct {
	listener net.Listener
	wg       *sync.WaitGroup
	quit     chan struct{}
	quitOnce sync.Once

	eWorker    *ExcelWorker
	eWorkerSet atomic.Bool
}

func NewRPCServer(laddr string) (*RPCServer, error) {
	// https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-security-and-access-rights
	// The ACLs in the default security descriptor for a named pipe grant full control to the LocalSystem account,
	// administrators, and the creator owner. They also grant read access to members of the Everyone group
	// and the anonymous account.
	wPipe, err := winio.ListenPipe(laddr, &winio.PipeConfig{
		MessageMode:      true,
		InputBufferSize:  MSG_MAX_SIZE + 1,
		OutputBufferSize: MSG_MAX_SIZE + 1,
	})
	if err != nil {
		return nil, err
	}
	return &RPCServer{
		listener: wPipe,
		wg:       &sync.WaitGroup{},
		quit:     make(chan struct{}),
	}, nil
}

func (r *RPCServer) Start() {
	common.Logger.Info("Server started, listening on: " + r.listener.Addr().String())

	// -------- initialize excel worker -------- //
	// enable scripting access to VBAObject Model
	err := LiftVBAScriptingAccess("16.0", "Excel")
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
	r.eWorker = &ExcelWorker{}
	err = r.eWorker.Init(inDebugging)
	if err != nil {
		common.Logger.Error("Failed to initialize excel worker:" + err.Error())
		return
	}
	defer r.eWorker.Quit(false)
	err = r.eWorker.GetWorkbooks()
	if err != nil {
		common.Logger.Error("Failed to get workbooks:" + err.Error())
		return
	}
	common.Logger.Info("Excel.Application worker initialized.")
	r.eWorkerSet.Store(true)

	// -------- connection handling -------- //
	r.wg.Add(1)
	go r.acceptRPCConnection()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		common.Logger.Info("Received SYS Signal, shutting down")
	case <-r.quit:
		common.Logger.Info("Received QUIT control, shutting down")
	}

	r.Stop()
}

func (r *RPCServer) Stop() {
	common.Logger.Info("Server stopping...")

	if r.listener != nil {
		r.listener.Close()
	}

	select {
	case <-r.quit:
	default:
		r.quitOnce.Do(func() {
			close(r.quit)
		})
	}

	waitD := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(waitD)
	}()

	select {
	case <-waitD:
		common.Logger.Info("All goroutines stopped gracefully")
	case <-time.After(15 * time.Second):
		common.Logger.Warn("Timed out 15 seconds, waiting for goroutines to stop")
	}

	common.Logger.Info("Server stopped")
}

func (r *RPCServer) acceptRPCConnection() {
	defer r.wg.Done()

	// -------- connection handling -------- //
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			select {
			case <-r.quit:
				// Expected error during shutdown
				return
			default:
				common.Logger.Error(fmt.Sprintf("Error accepting RPC connection: %v", err))
				// Add a small delay to prevent tight loop in case of persistent errors
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}

		select {
		case <-r.quit:
			// If we're shutting down, close the connection and return
			conn.Close()
			return
		default:
			common.Logger.Info("Accepted RPC connection.")
			r.wg.Add(1)
			go r.handleRPCConnection(conn)
		}
	}

}

func (r *RPCServer) handleRPCConnection(conn net.Conn) {
	defer r.wg.Done()
	defer conn.Close()
	common.Logger.Info("Connection established, now handling.")

	// create a context that gets cancelled when quit is signaled
	ctx, cancelF := context.WithCancel(context.Background())
	defer cancelF()
	// monitor any quit
	go func() {
		select {
		case <-r.quit:
			cancelF()
		case <-ctx.Done():
		}
	}()

	// line delimited json
	scanner := bufio.NewScanner(conn)
	msgChan := make(chan string, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(msgChan)
		for scanner.Scan() {
			select {
			case msgChan <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case errChan <- err:
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-r.quit:
			return
		case curMsg, ok := <-msgChan:
			if !ok {
				// Channel closed, connection ended
				return
			}
			curMsg = strings.TrimSpace(curMsg)
			if curMsg == "" {
				continue
			}
			var rpcMsg = &common.IPCReqMessageBase{}
			if err := json.Unmarshal([]byte(curMsg), rpcMsg); err != nil {
				common.Logger.Error(fmt.Sprintf("Error unmarshalling message: %v", err))
				continue
			}
			if rpcMsg.ClientID == "" {
				common.Logger.Info("Illegal RPC Message, ignoring...")
				continue
			}
			common.Logger.Info(fmt.Sprintf("Connection from client ID: %s , MessageID: %d ", rpcMsg.ClientID, rpcMsg.MessageID))
			common.Logger.Debug("Recved msg: " + curMsg)
			err2 := r.handleMessage(conn, rpcMsg)
			select {
			case <-r.quit:
				common.Logger.Info("Received QUIT control, shutting down from HandleFunc")
				return
			default:
				if err2 != nil {
					common.Logger.Error(fmt.Sprintf("Error handling message: %v", err2))
				}
			}
		case err := <-errChan:
			select {
			case <-r.quit:
				// expected error
				return
			default:
				common.Logger.Error(fmt.Sprintf("Error reading from connection: %v", err))
				return
			}
		}
	}
}

func (r *RPCServer) handleMessage(conn net.Conn, msg *common.IPCReqMessageBase) error {
	switch msg.MsgType {
	case "control":
		var controlMsg = &common.IPCServerControl{}
		err := json.Unmarshal(msg.MsgData, controlMsg)
		if err != nil {
			common.Logger.Error(fmt.Sprintf("Error unmarshalling control message: %v", err))
			return err
		}
		switch controlMsg.ControlAction {
		case "ping":
			_, err = conn.Write(buildServerRespInBytes(msg, 200, "pong"))
			return err
		case "disconn":
			_, err = conn.Write(buildServerRespInBytes(msg, 200, "ack"))
			return err
		case "quit":
			common.Logger.Info("Received QUIT control msg")
			_, err = conn.Write(buildServerRespInBytes(msg, 200, "ack"))
			r.quit <- struct{}{}
			r.quitOnce.Do(func() {
				close(r.quit)
			})
			return err
		default:
			_, err = conn.Write(buildServerRespInBytes(msg, 400, "invalid request"))
			common.Logger.Info("Received unknown control action: " + controlMsg.ControlAction)
			return err
		}
	case "sanitize":
		var docSanitizeMsg = &common.IPCSingleDocToBeSanitized{}
		err := json.Unmarshal(msg.MsgData, docSanitizeMsg)
		if err != nil {
			common.Logger.Error(fmt.Sprintf("Error unmarshalling sanitize message: %v", err))
			return err
		}
		if !r.eWorkerSet.Load() || r.eWorker == nil {
			common.Logger.Error("eWorker does not initialized correctly.")
			r.quit <- struct{}{}
			return customerrs.ErrExcelWorkerUninitialized
		}
		// change path separator, make sure consistent in os-level
		fPathNonVariant, err2 := filepath.Abs(docSanitizeMsg.Path)
		if err2 != nil {
			common.Logger.Error("Failed to get absolute path: " + err2.Error())
			return err2
		}
		// backup file
		err = gzBakFile(fPathNonVariant)
		if err != nil {
			common.Logger.Error("Backup file failed: " + err.Error())
		}
		common.Logger.Info("Original file backup succeeded: " + docSanitizeMsg.Path)
		// sleep 1 second to leave space for saving
		time.Sleep(1 * time.Second)
		// send response and processing using another goroutine
		_, err = conn.Write(buildServerRespInBytes(msg, 202, "file enqueued"))
		if err != nil {
			common.Logger.Error(fmt.Sprintf("Error writing to connection: %v", err))
			return err
		}
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			// 60 seconds should be sufficient for opening and sanitizing a single normal doc
			//
			// unfortunately, in some rare cases, it cost around 109 seconds for open.
			// in case of such a sucking document, have to change timeout to 180s
			ctx, cancelF := context.WithTimeout(context.TODO(), 180*time.Second)
			defer cancelF()
			errC := make(chan error, 1)
			common.Logger.Info("Waiting for file to be cleaned up: " + fPathNonVariant)
			r.excelFileCleanProcedure(ctx, fPathNonVariant, docSanitizeMsg.Action, docSanitizeMsg.DestModule, errC)
			common.Logger.Debug("excelFileCleanProcedure finished.")
		}()
	}
	return nil
}

func buildServerRespInBytes(msgbase *common.IPCReqMessageBase, resCode uint32, msg string) []byte {
	respS := &common.IPCMessageResp{
		ClientID:      msgbase.ClientID,
		MessageID:     msgbase.MessageID,
		ResultCode:    resCode,
		AdditionalMsg: msg,
	}
	respB, _ := json.Marshal(respS)
	return append(respB, byte('\n'))
}

func (r *RPCServer) excelFileCleanProcedure(ctx context.Context, fPath string, targetOp string, targetMod string, errC chan error) {
	// start actual processing
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		// get lock first
		r.eWorker.Lock()
		defer r.eWorker.Unlock()
		// open workbook
		common.Logger.Info("Opening workbook in sanitizer: " + fPath)
		err3 := r.eWorker.OpenWorkbook(fPath)
		if err3 != nil {
			common.Logger.Error("Failed to open workbook in sanitizer: " + err3.Error())
			errC <- err3
			return
		}
		common.Logger.Debug("Workbook opened: " + fPath)
		// defer save and close
		defer func() {
			err4 := r.eWorker.SaveAndCloseWorkbook()
			if err4 != nil {
				common.Logger.Error("Failed to save and close workbook in defer Sanitizer: " + err4.Error())
			}
			time.Sleep(1 * time.Second)
			// rename file and save to clean state cache of cloud-storage provider
			err4 = renameFileAndSave(fPath)
			if err4 != nil {
				common.Logger.Error("Rename file failed in sanitizer: " + err4.Error())
			}
			common.Logger.Info("Workbook Sanitized: " + fPath)
		}()
		// sanitize
		common.Logger.Debug("Sanitize Workbook VBA Module now.")
		err3 = r.eWorker.SanitizeWorkbook(targetOp, targetMod)
		if err3 != nil {
			common.Logger.Error("Failed to sanitize workbook: " + err3.Error())
			errC <- err3
			return
		}
		common.Logger.Info("Finished Sanitizing Workbook: " + fPath)
		common.Logger.Debug("Sanitize Workbook VBA Module finished, doneC returned.")
		errC <- nil
	}()
	select {
	case err := <-errC:
		if err != nil {
			common.Logger.Error("Failed to sanitize workbook, errC returned: " + err.Error())
			telemetry.CaptureException(err, "RPCServer.excelFileCleanProcedure.ErrC")
			telemetry.CaptureMessage("error", "RPCServer.excelFileCleanProcedure.ErrC: "+fPath)
			return
		}
		// properly remediated
		// go ahead
		common.Logger.Debug("Sanitize workbook finished, doneC returned correctly.")
		return
	case <-ctx.Done():
		// timed out or error
		err5 := ctx.Err()
		if err5 != nil {
			telemetry.CaptureException(err5, "RPCServer.excelFileCleanProcedure.CtxTimedOut")
			telemetry.CaptureMessage("error", "RPCServer.excelFileCleanProcedure.CtxTimedOut: "+fPath)
			common.Logger.Error("Failed to sanitize workbook, timed out: " + err5.Error())
		}
		common.Logger.Info("Sanitize workbook timed out, ctx.Done() returned, go to force clean.")
		// set mark for recreation
		r.eWorkerSet.Store(false)
		// for GC, cleanup and rebuild excel instance
		originalDbgStatus := r.eWorker.inDbg
		r.eWorker.Quit(true)
		// safely ignore errors as it's already built correctly before
		_ = r.eWorker.Init(originalDbgStatus)
		_ = r.eWorker.GetWorkbooks()
		// set mark again for ready to use
		r.eWorkerSet.Store(true)
		return
	}
}
