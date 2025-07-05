//go:build windows

package sanitizer_ole

import (
	"context"
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/sanitizer_ole/pbrpc"
	"cramc_go/telemetry"
	"fmt"
	"github.com/Microsoft/go-winio"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"net"
	"strings"
	"time"
)

type SimpleRPCClient struct {
	dialAddr string
	clientID string

	grpcClientConn *grpc.ClientConn
	rpcClient      pbrpc.ExcelSanitizerRPCClient
}

func InitSimpleRPCClient(clientID string, dialAddr string) (*SimpleRPCClient, error) {
	return &SimpleRPCClient{
		dialAddr: dialAddr,
		clientID: clientID,
	}, nil
}

func (c *SimpleRPCClient) Connect() error {
	var err error
	ctx := context.Background()
	c.grpcClientConn, err = dialContext(ctx, c.dialAddr)
	if err != nil {
		return err
	}
	c.rpcClient = pbrpc.NewExcelSanitizerRPCClient(c.grpcClientConn)
	return nil
}

func (c *SimpleRPCClient) Disconnect() error {
	return c.grpcClientConn.Close()
}

func (c *SimpleRPCClient) Ping() error {
	return c.sendControlMsg(pbrpc.ControlAction_PING)
}

func (c *SimpleRPCClient) SendQuit() error {
	return c.sendControlMsg(pbrpc.ControlAction_QUIT)
}

func (c *SimpleRPCClient) sendControlMsg(actionType pbrpc.ControlAction) error {
	ctx, cancelF := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelF()
	ctrlMsg := &pbrpc.ControlMsg{}
	metaMsg := c.PrepareMsgMeta()
	ctrlMsg.SetMeta(metaMsg)
	common.Logger.Info(fmt.Sprintf("%s Control Message with MsgID: %d Prepared.", actionType.String(), metaMsg.GetMessageID()))
	ctrlMsg.SetAction(actionType)
	resp, err := c.rpcClient.ControlServer(ctx, ctrlMsg)
	if err != nil {
		common.Logger.Error(actionType.String() + " via GRPC failed: " + err.Error())
		return err
	}
	common.Logger.Info(fmt.Sprintf("%s via GRPC succeeded, Code: %d, Msg: %s", actionType.String(), resp.GetResultCode(), resp.GetAdditionalMsg()))
	return nil
}

func (c *SimpleRPCClient) SendDocumentSanitizeRequest(reqDoc *common.IPCSingleDocToBeSanitized) error {
	// get file from queue and send it out, waiting for response
	msgMeta := c.PrepareMsgMeta()
	common.Logger.Info(fmt.Sprintf("New File Sanitize Request Prepared with MsgID: %d , FilePath: %s ", msgMeta.GetMessageID(), reqDoc.Path))
	sanReq := &pbrpc.SanitizeDocRequest{}
	sanReq.SetMeta(msgMeta)
	sanReq.SetAction(reqDoc.Action)
	sanReq.SetDestModule(reqDoc.DestModule)
	sanReq.SetDetectionName(reqDoc.DetectionName)
	sanReq.SetPath(reqDoc.Path)
	ctx, cancelF := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelF()
	resp, err := c.rpcClient.SanitizeDocument(ctx, sanReq)
	if err != nil {
		msg := fmt.Sprintf("SimpleRPCClient.SendDocumentSanitizeRequest Failure: %s, Doc: %s, MsgID: %d", err.Error(), reqDoc.Path, msgMeta.GetMessageID())
		telemetry.CaptureMessage("error", msg)
		common.Logger.Error("Sanitize Request for" + reqDoc.Path + " via GRPC failed: " + err.Error())
		return err
	}
	if resp.GetResultCode() >= 400 {
		msg := fmt.Sprintf("SimpleRPCClient.SendDocumentSanitizeReq_RESP: MsgID: %d, Path: %s, RespCode: %d, RespInfo: %s", resp.GetMeta().GetMessageID(), reqDoc.Path, resp.GetResultCode(), resp.GetAdditionalMsg())
		telemetry.CaptureMessage("error", msg)
		return customerrs.ErrUnknownInternalError
	}
	common.Logger.Info(fmt.Sprintf("Sanitize Response Successfully Received: %s, RespCode: %d, RespInfo: %s, MsgID: %d", reqDoc.Path, resp.GetResultCode(), resp.GetAdditionalMsg(), resp.GetMeta().GetMessageID()))
	return nil
}

func (c *SimpleRPCClient) PrepareMsgMeta() *pbrpc.GeneralMessageMeta {
	msgMeta := &pbrpc.GeneralMessageMeta{}
	msgMeta.SetMessageID(generateMessageId())
	msgMeta.SetClientID(c.clientID)
	return msgMeta
}

func generateMessageId() uint64 {
	return uint64(time.Now().UTC().UnixMilli())
}

// dialContext connects to the serverAddress for grpc.
// it is designed to connect to named pipe (`\\.\\pipe\<addr>`).
// the code is grabbed from the Chromium project:
// https://chromium.googlesource.com/infra/infra/go/src/infra/+/refs/heads/main/build/siso/execute/reproxyexec/dial_windows.go
func dialContext(ctx context.Context, serverAddr string) (*grpc.ClientConn, error) {
	if strings.HasPrefix(serverAddr, `winiopipe://`) {
		return dialPipe(ctx, strings.TrimPrefix(serverAddr, `winiopipe://`))
	}
	var opts = []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(4 * 1024 * 1024)),
	}
	return grpc.NewClient(
		serverAddr, opts...)
}

func dialPipe(ctx context.Context, pipeName string) (*grpc.ClientConn, error) {
	finalAddr := `\\.\pipe\` + pipeName
	var opts = []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(4 * 1024 * 1024)),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return winio.DialPipeContext(ctx, finalAddr)
		}),
	}
	return grpc.NewClient("passthrough:///"+finalAddr, opts...)
}
