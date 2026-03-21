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
	Module       string `json:"module"`
}

// request to sanitize
type SanitizeRequest struct {
	Path          string `json:"path"`
	Action        string `json:"action"`
	DetectionName string `json:"detectionName"`
	DestModule    string `json:"module"`

	// additional optional field for grpc
	MessageID uint64 `json:"msgID,omitempty"`
}

type SanitizeResult struct {
	RequestMsgID uint64 `json:"reqMsgID,omitempty"`
	Path         string `json:"path,omitempty"`
	Success      bool   `json:"success,omitempty"`
	Message      string `json:"message,omitempty"`
	Error        error  `json:"error,omitempty"`
}

type ExtractedVBAModule struct {
	ModuleName string `json:"moduleName,omitempty"`
	SourceCode []byte `json:"sourceCode,omitempty"`
	TextOffset uint32 `json:"-"`
}

type ExtractedVBAModules = []*ExtractedVBAModule

type HardeningAction struct {
	Name                string                 `json:"name"`
	AllowRepeatedHarden bool                   `json:"allowRepeatedHarden"`
	ActionLst           []*SingleHardenMeasure `json:"actionLst"`
}
