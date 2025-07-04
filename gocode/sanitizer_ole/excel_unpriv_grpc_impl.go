package sanitizer_ole

import (
	"context"
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/sanitizer_ole/pbrpc"
	"fmt"
	"sync"
)

type SimpleRPCServer struct {
	pbrpc.UnimplementedExcelSanitizerRPCServer

	wg sync.WaitGroup

	quitOnce sync.Once
	quitChan chan struct{}
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
		uniResp.SetResultCode(0)
		uniResp.SetAdditionalMsg("pong")
		common.Logger.Info("Received ping from client: " + cMsg.GetMeta().GetClientID())
		return uniResp, nil
	case pbrpc.ControlAction_QUIT:
		uniResp.SetResultCode(0)
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

}
