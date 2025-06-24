package telemetry

import (
	"bytes"
	"cramc_go/common"
	"net/http"
	"os"
	"os/user"
	"strings"
	"sync/atomic"
)

var (
	inited            = new(atomic.Bool)
	currentHostname   string
	currentUsername   string
	currentIP         string
	currentSender     TelemetrySender
	currentRelVersion string
)

type telemetryEvent struct {
	LogLevel           string `json:"logLevel,omitempty"`
	ReleaseVersion     string `json:"releaseVersion,omitempty"`
	HostName           string `json:"hostName,omitempty"`
	UserName           string `json:"userName,omitempty"`
	IpAddress          string `json:"ipAddress,omitempty"`
	RuntimeOS          string `json:"runtimeOS,omitempty"`
	RuntimeArch        string `json:"runtimeArch,omitempty"`
	Message            string `json:"message,omitempty"`
	LocalUnixTimestamp int64  `json:"localUnixTs,omitempty"`
}

type TelemetrySender interface {
	CaptureMessage(level string, message string)
	SetDefaultSender()
}

func Init(relVersion string) {
	var err error
	currentRelVersion = relVersion
	currentHostname, err = os.Hostname()
	if err != nil {
		common.Logger.Errorln(err)
	}
	curUser, err := user.Current()
	if err != nil {
		common.Logger.Errorln(err)
	}
	currentUsername = curUser.Username
	currentIP = getCurrentPublicIP()
	inited.Store(true)
}

func getCurrentPublicIP() string {
	resp, err := http.Get("https://myip.ipip.net")
	if err != nil {
		common.Logger.Errorln(err)
		return ""
	}
	defer resp.Body.Close()
	var buf = bytes.NewBuffer(nil)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		common.Logger.Errorln(err)
		return ""
	}
	finalIp := buf.String()
	return strings.TrimSpace(finalIp)
}
