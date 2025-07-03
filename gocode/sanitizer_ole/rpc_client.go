package sanitizer_ole

import (
	"bufio"
	"cramc_go/common"
	"cramc_go/customerrs"
	"encoding/json"
	"github.com/Microsoft/go-winio"
	"net"
	"sync/atomic"
	"time"
)

const RpcCallAddr = `\\.\pipe\cramcPriv`

type RPCClient struct {
	dialAddr  string
	currConn  net.Conn
	connected atomic.Bool
	clientID  string
	scanner   *bufio.Scanner
}

func NewRPCClient(dialAddr string, clientID string) *RPCClient {
	return &RPCClient{dialAddr: dialAddr, clientID: clientID}
}

func generateMessageId() int64 {
	return time.Now().UTC().UnixMilli()
}

func (c *RPCClient) Connect() error {
	// cleanup existing connection
	if c.currConn != nil {
		_ = c.currConn.Close()
	}
	c.connected.Store(false)
	// retry 3 times, 5 seconds time out.
	var lastErr error
	timeOut := time.Duration(5 * time.Second)
	for i := 0; i < 3; i++ {
		common.Logger.Infof("Connecting, attempt %d / 3...", i)
		conn, err := winio.DialPipe(c.dialAddr, &timeOut)
		if err != nil {
			common.Logger.Errorln("Attempt to connect, encountered error: ", err)
			lastErr = err
			continue
		}
		c.currConn = conn
		c.scanner = bufio.NewScanner(conn)
		c.connected.Store(true)
		common.Logger.Infof("Successfully connected to %s", c.dialAddr)
		return nil
	}
	if lastErr != nil && !c.connected.Load() {
		return lastErr
	}
	return nil
}

func (c *RPCClient) Disconnect() error {
	msgID, err := c.SendControlMessage("disconn")
	if err != nil {
		return err
	}
	common.Logger.Infof("Disconnect Triggered, Send Control Message with ID: %d ", msgID)
	resp, err := c.procRespData()
	if err != nil {
		return err
	}
	common.Logger.Infof("Received Control Message Response: %v", resp)
	_ = c.ConnClose()
	common.Logger.Infoln("Disconnected")
	return nil
}

func (c *RPCClient) ConnClose() error {
	if c.currConn != nil || c.connected.Load() {
		return customerrs.ErrRpcConnectionNotEstablished
	}
	_ = c.currConn.Close()
	c.connected.Store(false)
	c.currConn = nil
	c.scanner = nil
	return nil
}

func (c *RPCClient) Ping() error {
	if c.currConn == nil || !c.connected.Load() {
		return customerrs.ErrRpcConnectionNotEstablished
	}
	_, err := c.SendControlMessage("ping")
	if err != nil {
		return err
	}
	respObj, err := c.procRespData()
	if err != nil {
		return err
	}
	if respObj.AdditionalMsg == "pong" {
		return nil
	}
	return customerrs.ErrRpcResponseUnexpected
}

func (c *RPCClient) SendSanitizeMessage(docSanitizeMsg *common.IPCSingleDocToBeSanitized) (int64, error) {
	if !c.connected.Load() {
		return -1, customerrs.ErrRpcConnectionNotEstablished
	}
	msgbase := c.prepareMessageSkeleton()
	msgbase.MsgType = "sanitize"
	smsgData, err := json.Marshal(docSanitizeMsg)
	if err != nil {
		return -1, err
	}
	msgbase.MsgData = smsgData
	finalMsg, err := json.Marshal(msgbase)
	if err != nil {
		return -1, err
	}
	finalMsg = append(finalMsg, byte('\n'))
	common.Logger.Infof("Sending sanitize message, ID: %d, File: %s, Detection: %s",
		msgbase.MessageID, docSanitizeMsg.Path, docSanitizeMsg.DetectionName)
	_, err = c.currConn.Write(finalMsg)
	if err != nil {
		return -1, err
	}
	respObj, err := c.procRespData()
	if err != nil {
		return -1, err
	}
	common.Logger.Infof("Sanitize message response: %v ", respObj)
	return msgbase.MessageID, err
}

func (c *RPCClient) SendControlMessage(action string) (int64, error) {
	if !c.connected.Load() {
		return -1, customerrs.ErrRpcConnectionNotEstablished
	}
	msgbase := c.prepareMessageSkeleton()
	msgbase.MsgType = "control"
	controlmsg := &common.IPCServerControl{ControlAction: action}
	cmsgData, err := json.Marshal(controlmsg)
	if err != nil {
		return -1, err
	}
	msgbase.MsgData = cmsgData
	finalMsg, err := json.Marshal(msgbase)
	if err != nil {
		return -1, err
	}
	finalMsg = append(finalMsg, byte('\n'))
	common.Logger.Infof("Sending control message: %s, ID: %d", action, msgbase.MessageID)
	_, err = c.currConn.Write(finalMsg)
	if err != nil {
		return -1, err
	}
	respObj, err := c.procRespData()
	if err != nil {
		return -1, err
	}
	common.Logger.Infof("Control message response: %v ", respObj)
	return msgbase.MessageID, err
}

func (c *RPCClient) prepareMessageSkeleton() *common.IPCReqMessageBase {
	return &common.IPCReqMessageBase{
		ClientID:  c.clientID,
		MessageID: generateMessageId(),
	}
}

func (c *RPCClient) procRespData() (*common.IPCMessageResp, error) {
	if !c.connected.Load() {
		return nil, customerrs.ErrRpcConnectionNotEstablished
	}
	if !c.scanner.Scan() {
		common.Logger.Errorf("Error scanning response: %v", c.scanner.Err())
		return nil, customerrs.ErrUnknownInternalError
	}
	respBytes := c.scanner.Bytes()
	resp := &common.IPCMessageResp{}
	err := json.Unmarshal(respBytes, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *RPCClient) RequestTerminateAndDisconnect() error {
	if !c.connected.Load() {
		return customerrs.ErrRpcConnectionNotEstablished
	}
	// not necessary to check alive
	// direct request termination
	msgId, err := c.SendControlMessage("quit")
	if err != nil {
		return err
	}
	common.Logger.Infof("Request Terminate and Disconnect Triggered, Send Control Message with ID: %d ", msgId)
	resp, err := c.procRespData()
	if err != nil {
		return err
	}
	common.Logger.Infof("Received Quit Message Response: %v", resp)
	_ = c.ConnClose()
	common.Logger.Infoln("Quit initiated and now terminating connection. Waiting for server cleanup.")
	return nil
}
