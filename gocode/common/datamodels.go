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

type HardeningAction struct {
	Name                string                 `json:"name"`
	AllowRepeatedHarden bool                   `json:"allowRepeatedHarden"`
	ActionLst           []*SingleHardenMeasure `json:"actionLst"`
}
