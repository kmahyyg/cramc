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
	hostInited        = new(atomic.Bool)
	senderInited      = new(atomic.Bool)
	currentHostname   string
	currentUsername   string
	currentIP         string
	currentSender     TSender
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
	RunElevated        bool   `json:"runElevated,omitempty"`
	LocalUnixTimestamp int64  `json:"localUnixTs,omitempty"`
}

type TSender interface {
	CaptureMessage(level string, message string)
	CaptureException(err error, source string)
	SetDefaultSender()
}

func Init(relVersion string) {
	var err error
	currentRelVersion = relVersion
	currentHostname, err = os.Hostname()
	if err != nil {
		common.Logger.Error(err.Error())
	}
	curUser, err := user.Current()
	if err != nil {
		common.Logger.Error(err.Error())
	}
	if curUser != nil {
		currentUsername = curUser.Username
	} else {
		currentUsername = "unknown-user"
	}
	currentIP = getCurrentPublicIP()
	hostInited.Store(true)
}

func getCurrentPublicIP() string {
	resp, err := http.Get("https://myip.ipip.net")
	if err != nil {
		common.Logger.Error(err.Error())
		return ""
	}
	defer resp.Body.Close()
	var buf = bytes.NewBuffer(nil)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		common.Logger.Error(err.Error())
		return ""
	}
	finalIp := buf.String()
	return strings.TrimSpace(finalIp)
}

func CaptureMessage(level string, message string) {
	if !senderInited.Load() {
		return
	}
	if currentSender != nil {
		currentSender.CaptureMessage(level, message)
	}
}

func CaptureException(err error, source string) {
	if !senderInited.Load() {
		return
	}
	if currentSender != nil {
		currentSender.CaptureException(err, source)
	}
}
