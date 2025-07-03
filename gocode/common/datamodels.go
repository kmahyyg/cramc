package common

// fully local items
type CRAMCCleanupDB struct {
	Version   int                    `json:"version"`
	Solutions []*SingleVirusSolution `json:"solutions"`
}

type SingleVirusSolution struct {
	Name                string                 `json:"name"`
	DestModule          string                 `json:"module"`
	Action              string                 `json:"action"`
	MustHarden          bool                   `json:"mustHarden,omitempty"`
	AllowRepeatedHarden bool                   `json:"allowRepeatedHarden,omitempty"`
	HardenMeasures      []*SingleHardenMeasure `json:"hardenMeasures,omitempty"`
}

type SingleHardenMeasure struct {
	Action string `json:"action"`
	Type   string `json:"type"`
	Dest   string `json:"dest"`
}

// internal information transfer
type YaraScanResult struct {
	DetectedRule string `json:"detectedRule"`
	FilePath     string `json:"filePath"`
}

// IPC, request to sanitize
type IPCSingleDocToBeSanitized struct {
	Path          string `json:"path"`
	Action        string `json:"action"`
	DetectionName string `json:"detectionName"`
	DestModule    string `json:"module"`
}

// IPC, server control msg
type IPCServerControl struct {
	ControlAction string `json:"controlAction"`
}

// IPC, general response
type IPCMessageResp struct {
	ClientID      string `json:"clientID"`
	MessageID     int64  `json:"messageID"`
	ResultCode    uint32 `json:"resultCode"`
	AdditionalMsg string `json:"additionalMsg"`
}

// IPC, msg framework
type IPCReqMessageBase struct {
	ClientID  string `json:"clientID"`
	MessageID int64  `json:"messageID"`
	MsgType   string `json:"msgType"`
	MsgData   []byte `json:"msgData"`
}

type HardeningAction struct {
	Name                string                 `json:"name"`
	AllowRepeatedHarden bool                   `json:"allowRepeatedHarden"`
	ActionLst           []*SingleHardenMeasure `json:"actionLst"`
}
