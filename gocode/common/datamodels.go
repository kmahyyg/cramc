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
type IPC_SingleDocToBeSanitized struct {
	Path          string `json:"path"`
	Action        string `json:"action"`
	DetectionName string `json:"detectionName"`
	DestModule    string `json:"module"`
}

// IPC, response to sanitization request
type IPC_Resp_SingleDocToBeSanitized struct {
	Code          uint32 `json:"code"`
	Path          string `json:"path"`
	Result        string `json:"result"`
	AdditionalMsg string `json:"additionalMsg"`
}

// IPC, server control msg
type IPC_ServerControl struct {
	ControlAction string `json:"controlAction"`
}

// IPC, response to server control msg
type IPC_ServerControlResp struct {
	IPC_ServerControl
	ResultCode uint32 `json:"resultCode"`
}

type HardeningAction struct {
	Name                string                 `json:"name"`
	AllowRepeatedHarden bool                   `json:"allowRepeatedHarden"`
	ActionLst           []*SingleHardenMeasure `json:"actionLst"`
}
