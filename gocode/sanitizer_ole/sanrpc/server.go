package sanrpc

import (
	"cramc_go/common"
	"github.com/Microsoft/go-winio"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type RPCServer struct {
	listener net.Listener
	wg       *sync.WaitGroup
	quit     chan struct{}
}

func NewRPCServer(laddr string) (*RPCServer, error) {
	// https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-security-and-access-rights
	// The ACLs in the default security descriptor for a named pipe grant full control to the LocalSystem account,
	// administrators, and the creator owner. They also grant read access to members of the Everyone group
	// and the anonymous account.
	wPipe, err := winio.ListenPipe(laddr, &winio.PipeConfig{
		MessageMode:      true,
		InputBufferSize:  65536,
		OutputBufferSize: 65536,
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

}
