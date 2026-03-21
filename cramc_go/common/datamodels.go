package common

// internal information transfer
type YaraScanResult struct {
	DetectedRule string `json:"detectedRule"`
	FilePath     string `json:"filePath"`
	Module       string `json:"module"`
}

type ExtractedVBAModule struct {
	ModuleName string `json:"moduleName,omitempty"`
	SourceCode []byte `json:"sourceCode,omitempty"`
	TextOffset uint32 `json:"-"`
}

type ExtractedVBAModules = []*ExtractedVBAModule
