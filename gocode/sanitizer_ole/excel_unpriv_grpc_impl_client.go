//go:build windows

package sanitizer_ole

import (
	"context"
	"cramc_go/common"
	"cramc_go/sanitizer_ole/pbrpc"
	"errors"
	"fmt"
	"github.com/Microsoft/go-winio"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
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

func (c *SimpleRPCClient) CallDocSanitizeReqSender(ctrlChan chan struct{}) (pbrpc.ExcelSanitizerRPC_SanitizeDocumentClient, error) {
	ctx := context.Background()
	stream, err := c.rpcClient.SanitizeDocument(ctx)
	if err != nil {
		close(ctrlChan)
		return nil, err
	}
	go func() {
		for {
			recvObj, err3 := stream.Recv()
			if errors.Is(err, io.EOF) {
				common.Logger.Info("Client disconnected in SanitizeDocumentCall.")
				ctrlChan <- struct{}{}
				close(ctrlChan)
				return
			} else if err3 != nil {
				common.Logger.Error("Failed to receive response to SanitizeDocReq: " + err3.Error())
				continue
			}
			common.Logger.Info(fmt.Sprintf("In Response To SanitizeDocReq ID %d , Response Code - %d, Msg - %s", recvObj.GetMeta().GetMessageID(), recvObj.GetResultCode(), recvObj.GetAdditionalMsg()))
		}
	}()
	return stream, nil
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
