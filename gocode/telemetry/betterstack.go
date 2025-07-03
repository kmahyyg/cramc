package telemetry

import (
	"bytes"
	"cramc_go/common"
	"cramc_go/customerrs"
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

type BetterStackSender struct {
	sendURL     string
	bearerToken string
}

func NewBetterStackSender(serverUrl string, authToken string) *BetterStackSender {
	if !hostInited.Load() {
		common.Logger.Error(customerrs.ErrTelemetryMustBeInitedFirst.Error())
		return nil
	}
	return &BetterStackSender{sendURL: serverUrl, bearerToken: authToken}
}

func (bs *BetterStackSender) CaptureException(err error, source string) {
	bs.CaptureMessage("error", "from: "+source+", "+err.Error())
}

func (bs *BetterStackSender) CaptureMessage(level string, message string) {
	tsEv := &telemetryEvent{
		LogLevel:           level,
		ReleaseVersion:     currentRelVersion,
		HostName:           currentHostname,
		UserName:           currentUsername,
		IpAddress:          currentIP,
		RuntimeOS:          runtime.GOOS,
		RuntimeArch:        runtime.GOARCH,
		RunElevated:        common.IsElevated,
		Message:            message,
		LocalUnixTimestamp: time.Now().UnixMilli(),
	}
	sendBody, err := json.Marshal(tsEv)
	if err != nil {
		common.Logger.Error(err.Error())
		return
	}
	postBodyRd := bytes.NewReader(sendBody)
	hReq, err := http.NewRequest("POST", bs.sendURL, postBodyRd)
	if err != nil {
		common.Logger.Error(err.Error())
		return
	}
	hReq.Header.Set("Content-Type", "application/json")
	hReq.Header.Set("Authorization", "Bearer "+bs.bearerToken)
	hReq.Header.Set("User-Agent", "Mozilla/5.0 Chrome/137.0.0.0 Go-CRAMC-Telemetry/1.0")
	resp, err := http.DefaultClient.Do(hReq)
	if err != nil {
		common.Logger.Error(err.Error())
		return
	}
	defer resp.Body.Close()
	common.Logger.Debug("Telemetry sent, response: " + resp.Status)
	return
}

func (bs *BetterStackSender) SetDefaultSender() {
	currentSender = bs
	senderInited.Store(true)
}
