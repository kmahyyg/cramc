package sanitizer_ole

import (
	"context"
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/sanitizer_ole/pbrpc"
	"fmt"
	"path/filepath"
	"sync"
)

type SimpleRPCServer struct {
	pbrpc.UnimplementedExcelSanitizerRPCServer

	jobQueue       chan *common.IPCSingleDocToBeSanitized
	quitOnce       *sync.Once
	quitChan       chan struct{}
	workerCtrlChan chan struct{}
}

func InitSimpleRPCServer(jchan chan *common.IPCSingleDocToBeSanitized, qchan chan struct{}, qchanOnce *sync.Once, workerCChan chan struct{}) (*SimpleRPCServer, error) {
	// eworker is always initialized, check if nil is sufficient
	if jchan == nil || qchan == nil || qchanOnce == nil {
		return nil, customerrs.ErrUnknownInternalError
	}
	srv := &SimpleRPCServer{
		quitChan:       qchan,
		quitOnce:       qchanOnce,
		jobQueue:       jchan,
		workerCtrlChan: workerCChan,
	}

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
			s.workerCtrlChan <- struct{}{}
			close(s.quitChan)
			close(s.jobQueue)
			close(s.workerCtrlChan)
		})
		return uniResp, nil
	}
}

func (s *SimpleRPCServer) SanitizeDocument(_ context.Context, inObj *pbrpc.SanitizeDocRequest) (*pbrpc.UniversalResponse, error) {
	// error in this function always return nil to make sure message could be passed to client
	common.Logger.Info(fmt.Sprintf("Received SanitizeDocument request from client ID: %s , MessageID: %d ", inObj.GetMeta().GetClientID(), inObj.GetMeta().GetMessageID()))
	respMeta := &pbrpc.GeneralMessageMeta{}
	respMeta.SetMessageID(inObj.GetMeta().GetMessageID())
	respMeta.SetClientID(inObj.GetMeta().GetClientID())
	uniResp := &pbrpc.UniversalResponse{}
	uniResp.SetMeta(respMeta)
	common.Logger.Info(fmt.Sprintf("Processing Document: %s, Detection: %s", inObj.GetPath(), inObj.GetDetectionName()))
	// change path separator, make sure consistent in os-level
	fPathNonVariant, err := filepath.Abs(inObj.GetPath())
	if err != nil {
		common.Logger.Error("Failed to get absolute path: " + err.Error())
		uniResp.SetResultCode(400)
		uniResp.SetAdditionalMsg("Failed to get absolute path. ")
		return uniResp, nil
	}
	// backup file
	err = gzBakFile(fPathNonVariant)
	if err != nil {
		common.Logger.Error("Backup file failed: " + err.Error())
		uniResp.SetResultCode(412)
		uniResp.SetAdditionalMsg("Failed to backup original file. ")
		return uniResp, nil
	}
	common.Logger.Info("Original file backup succeeded: " + inObj.GetPath())
	// just processing with sending files to queue.
	newTsk := &common.IPCSingleDocToBeSanitized{
		Path:          fPathNonVariant, // WARNING!!! YOU MUST USE THIS PATH!
		Action:        inObj.GetAction(),
		DetectionName: inObj.GetDetectionName(),
		DestModule:    inObj.GetDestModule(),
		MessageID:     inObj.GetMeta().GetMessageID(),
	}
	s.jobQueue <- newTsk
	uniResp.SetResultCode(202)
	uniResp.SetAdditionalMsg("File Enqueued. Please check log for more details.")
	common.Logger.Info(fmt.Sprintf("Response to MsgID %d Sent to End User, Involved File: %s ", respMeta.GetMessageID(), inObj.GetPath()))
	return uniResp, nil
}
