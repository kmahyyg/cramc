package sanitizer_ole

import (
	"context"
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/sanitizer_ole/pbrpc"
	"cramc_go/telemetry"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type SimpleRPCServer struct {
	pbrpc.UnimplementedExcelSanitizerRPCServer

	wg *sync.WaitGroup

	eWorker    *ExcelWorker
	eWorkerSet atomic.Bool

	quitOnce *sync.Once
	quitChan chan struct{}
}

func InitSimpleRPCServer(worker *ExcelWorker, qchan chan struct{}, qchanOnce *sync.Once, wg *sync.WaitGroup) (*SimpleRPCServer, error) {
	// eworker is always initialized, check if nil is sufficient
	if worker == nil || wg == nil || qchan == nil || qchanOnce == nil {
		return nil, customerrs.ErrUnknownInternalError
	}
	srv := &SimpleRPCServer{
		quitChan:   qchan,
		quitOnce:   qchanOnce,
		eWorker:    worker,
		eWorkerSet: atomic.Bool{},
		wg:         wg,
	}
	srv.eWorkerSet.Store(true)

	return srv, nil
}

func (s *SimpleRPCServer) ControlServer(_ context.Context, cMsg *pbrpc.ControlMsg) (*pbrpc.UniversalResponse, error) {
	common.Logger.Info(fmt.Sprintf("Connection from client ID: %s , MessageID: %d ", cMsg.GetMeta().GetClientID(), cMsg.GetMeta().GetMessageID()))
	respMeta := &pbrpc.GeneralMessageMeta{}
	respMeta.SetMessageID(cMsg.GetMeta().GetMessageID())
	respMeta.SetClientID(cMsg.GetMeta().GetClientID())
	uniResp := &pbrpc.UniversalResponse{}
	uniResp.SetMeta(respMeta)
	switch cMsg.GetAction() {
	default:
		fallthrough
	case pbrpc.ControlAction_CONTROL_ACTION_UNKNOWN:
		uniResp.SetResultCode(400)
		uniResp.SetAdditionalMsg("unknown action found, ignored.")
		common.Logger.Info("Received unknown control action: " + cMsg.GetAction().String())
		return uniResp, customerrs.ErrInvalidInput
	case pbrpc.ControlAction_PING:
		uniResp.SetResultCode(200)
		uniResp.SetAdditionalMsg("pong")
		common.Logger.Info("Received ping from client: " + cMsg.GetMeta().GetClientID())
		return uniResp, nil
	case pbrpc.ControlAction_QUIT:
		uniResp.SetResultCode(202)
		uniResp.SetAdditionalMsg("ACK")
		common.Logger.Info("Responded to quit from client: " + cMsg.GetMeta().GetClientID())
		s.quitOnce.Do(func() {
			s.quitChan <- struct{}{}
			close(s.quitChan)
		})
		return uniResp, nil
	}
}

func (s *SimpleRPCServer) SanitizeDocument(stream pbrpc.ExcelSanitizerRPC_SanitizeDocumentServer) error {
	for {
		inObj, err := stream.Recv()
		if err == io.EOF {
			common.Logger.Info("Client disconnected in SanitizeDocumentCall.")
			return nil
		}
		if err != nil {
			return err
		}
		common.Logger.Info(fmt.Sprintf("Received SanitizeDocument request from client ID: %s , MessageID: %d ", inObj.GetMeta().GetClientID(), inObj.GetMeta().GetMessageID()))
		respMeta := &pbrpc.GeneralMessageMeta{}
		respMeta.SetMessageID(inObj.GetMeta().GetMessageID())
		respMeta.SetClientID(inObj.GetMeta().GetClientID())
		uniResp := &pbrpc.UniversalResponse{}
		uniResp.SetMeta(respMeta)
		common.Logger.Info(fmt.Sprintf("Processing Document: %s, Detection: %s", inObj.GetPath(), inObj.GetDetectionName()))
		if s.eWorker == nil {
			return customerrs.ErrUnknownInternalError
		}
		// change path separator, make sure consistent in os-level
		fPathNonVariant, err2 := filepath.Abs(inObj.GetPath())
		if err2 != nil {
			common.Logger.Error("Failed to get absolute path: " + err2.Error())
			return err2
		}
		// backup file
		err = gzBakFile(fPathNonVariant)
		if err != nil {
			common.Logger.Error("Backup file failed: " + err.Error())
		}
		common.Logger.Info("Original file backup succeeded: " + inObj.GetPath())
		// remove file in queue notice, just processing one by one, as there's already queueing in grpc
		func() {
			ctxForSani, cancelF := context.WithTimeout(context.TODO(), 180*time.Second)
			defer cancelF()
			errC := make(chan error, 1)
			common.Logger.Info("Waiting for file to be cleaned up: " + fPathNonVariant)
			s.excelFileCleanProcedure(ctxForSani, fPathNonVariant, inObj.GetAction(), inObj.GetDestModule(), errC)
			common.Logger.Info("excelFileCleanProcedure finished.")
		}()
		uniResp.SetResultCode(202)
		uniResp.SetAdditionalMsg("File Proceeded. Please check log for more details.")
		err = stream.Send(uniResp)
		if err != nil {
			common.Logger.Error("Failed to send response to client: " + err.Error())
			return err
		}
		common.Logger.Info("Response Sent to End User: " + inObj.GetPath())
	}
}

func (s *SimpleRPCServer) excelFileCleanProcedure(ctx context.Context, fPath string, targetOp string, targetMod string, errC chan error) {
	// start actual processing
	s.wg.Add(1)
	defer close(errC)
	go func() {
		defer s.wg.Done()
		// get lock first
		s.eWorker.Lock()
		defer s.eWorker.Unlock()
		// open workbook
		common.Logger.Info("Opening workbook in sanitizer: " + fPath)
		err3 := s.eWorker.OpenWorkbook(fPath)
		if err3 != nil {
			common.Logger.Error("Failed to open workbook in sanitizer: " + err3.Error())
			errC <- err3
			return
		}
		common.Logger.Debug("Workbook opened: " + fPath)
		// defer save and close
		defer func() {
			err4 := s.eWorker.SaveAndCloseWorkbook()
			if err4 != nil {
				common.Logger.Error("Failed to save and close workbook in defer Sanitizer: " + err4.Error())
			}
			// rename file and save to clean state cache of cloud-storage provider
			err4 = renameFileAndSave(fPath)
			if err4 != nil {
				common.Logger.Error("Rename file failed in sanitizer: " + err4.Error())
			}
			common.Logger.Info("Workbook Sanitized: " + fPath)
		}()
		// sanitize
		common.Logger.Debug("Sanitize Workbook VBA Module now.")
		err3 = s.eWorker.SanitizeWorkbook(targetOp, targetMod)
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
		s.eWorkerSet.Store(false)
		// for GC, cleanup and rebuild excel instance
		originalDbgStatus := s.eWorker.inDbg
		s.eWorker.Quit(true)
		// safely ignore errors as it's already built correctly before
		_ = s.eWorker.Init(originalDbgStatus)
		_ = s.eWorker.GetWorkbooks()
		// set mark again for ready to use
		s.eWorkerSet.Store(true)
		return
	}
}
