package sanitizer_ole

import (
	"bufio"
	"cramc_go/common"
	"encoding/json"
	"github.com/Microsoft/go-winio"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	MSG_MAX_SIZE = 65535
)

type RPCServer struct {
	listener net.Listener
	wg       *sync.WaitGroup
	quit     chan struct{}

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
	common.Logger.Infoln("Server started, listening on: ", r.listener.Addr().String())

	r.wg.Add(1)
	go r.acceptRPCConnection()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		common.Logger.Infoln("Received SYS Signal, shutting down")
	case <-r.quit:
		common.Logger.Infoln("Received QUIT control, shutting down")
	}

	r.Stop()
}

func (r *RPCServer) Stop() {
	common.Logger.Infoln("Server stopping...")

	if r.listener != nil {
		r.listener.Close()
	}

	select {
	case <-r.quit:
	default:
		close(r.quit)
	}

	r.wg.Wait()
	common.Logger.Infoln("Server stopped")
}

func (r *RPCServer) acceptRPCConnection() {
	defer r.wg.Done()

	for {
		select {
		case <-r.quit:
			return
		default:
			conn, err := r.listener.Accept()
			if err != nil {
				select {
				case <-r.quit:
					return
				default:
					common.Logger.Errorf("Error accepting RPC connection: %v", err)
					continue
				}
			}
			r.wg.Add(1)
			go r.handleRPCConnection(conn)
		}
	}
}

func (r *RPCServer) handleRPCConnection(conn net.Conn) {
	defer r.wg.Done()
	defer conn.Close()
	common.Logger.Infoln("Connection established, now handling.")
	// line delimited json
	scanner := bufio.NewScanner(conn)
	for {
		select {
		case <-r.quit:
			return
		default:
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					common.Logger.Errorf("Error reading from connection: %v", err)
					return
				}
			}
			curMsg := strings.TrimSpace(scanner.Text())
			if curMsg == "" {
				continue
			}
			var rpcMsg = &common.IPCReqMessageBase{}
			if err := json.Unmarshal([]byte(curMsg), rpcMsg); err != nil {
				common.Logger.Errorf("Error unmarshalling message: %v", err)
				continue
			}
			if rpcMsg.ClientID == "" {
				common.Logger.Infoln("Illegal RPC Message, ignoring...")
				continue
			}
			common.Logger.Infoln("Connection from client ID: ", rpcMsg.ClientID, " MessageID: ", rpcMsg.MessageID)
			common.Logger.Debugln("Recved msg: ", curMsg)
			err2 := r.handleMessage(conn, rpcMsg)
			select {
			case <-r.quit:
				return
			default:
				if err2 != nil {
					common.Logger.Errorf("Error handling message: %v", err2)
				}
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
			common.Logger.Errorf("Error unmarshalling control message: %v", err)
			return err
		}
		switch controlMsg.ControlAction {
		case "disconnect":
			_, err = conn.Write(getRespBytes(msg, 0, "ok"))
			return err
		case "quit":
			common.Logger.Infoln("Received QUIT control msg")
			_, err = conn.Write(getRespBytes(msg, 0, "ok"))
			r.quit <- struct{}{}
			return err
		default:
			_, err = conn.Write(getRespBytes(msg, 400, "invalid request"))
			common.Logger.Infoln("Received unknown control action: ", controlMsg.ControlAction)
			return err
		}
	case "sanitize":

	}
	return nil
}

func getRespBytes(msgbase *common.IPCReqMessageBase, resCode uint32, msg string) []byte {
	respS := &common.IPCMessageResp{
		ClientID:      msgbase.ClientID,
		MessageID:     msgbase.MessageID,
		ResultCode:    resCode,
		AdditionalMsg: msg,
	}
	respB, _ := json.Marshal(respS)
	return respB
}
