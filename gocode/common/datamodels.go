package common

// fully local items
type CRAMCCleanupDB struct {
	Version   int64                  `json:"version"`
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

// IPC, single response for /fetchPendingFiles
type IPC_SingleDocToBeSanitized struct {
	Path          string `json:"path"`
	Action        string `json:"action"`
	DetectionName string `json:"detectionName"`
}

// IPC, general response for /fetchPendingFiles
type IPC_DocsToBeSanitizedResp struct {
	Counter   int                           `json:"counter"`
	ToProcess []*IPC_SingleDocToBeSanitized `json:"toProcess"`
}

// IPC, general response for /getStatus
type IPC_StatusResponse struct {
	Status              string `json:"status"`
	FilesPendingInQueue int    `json:"filesPendingInQueue"`
}

type IPC_SingleSanitizedDocResponse struct {
	IPC_SingleDocToBeSanitized
	IsSuccess     bool   `json:"isSuccess"`
	AdditionalMsg string `json:"additionalMsg,omitempty"`
}

type IPC_SanitizedDocsResponse struct {
	Counter   int                               `json:"counter"`
	Processed []*IPC_SingleSanitizedDocResponse `json:"processed"`
}

type HardeningAction struct {
	Name string `json:"name"`
	SingleHardenMeasure
}
