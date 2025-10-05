package telemetry

import (
	"cramc_go/common"
	"cramc_go/customerrs"
)

type NoOpSender struct{}

func (noOp *NoOpSender) CaptureMessage(level string, message string) {
	// as there's always log.print followed, no further action required
	return
}

func (noOp *NoOpSender) CaptureException(err error, source string) {
	return
}

func (noOp *NoOpSender) SetDefaultSender() {
	currentSender = noOp
	senderInited.Store(true)
	return
}

func (noOp *NoOpSender) CaptureExceptionWithPath(err error, source string, fpath string) {
	return
}

func NewNoOpSender() *NoOpSender {
	if !hostInited.Load() {
		common.Logger.Error(customerrs.ErrTelemetryMustBeInitedFirst.Error())
		return nil
	}
	return &NoOpSender{}
}
