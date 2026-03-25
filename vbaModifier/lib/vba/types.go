package vba

type ModuleType string

const (
	ModuleTypeUnknown  ModuleType = "unknown"
	ModuleTypeDocument ModuleType = "document"
	ModuleTypeStandard ModuleType = "standard"
	ModuleTypeClass    ModuleType = "class"
	ModuleTypeDesigner ModuleType = "designer"
)

type Module struct {
	Name       string     `json:"name,omitempty"`
	StreamName string     `json:"streamName,omitempty"`
	Type       ModuleType `json:"type,omitempty"`
	TextOffset uint32     `json:"textOffset,omitempty"`
	ReadOnly   bool       `json:"readOnly,omitempty"`
	Private    bool       `json:"private,omitempty"`

	offsetPos int
}

type ModuleTextOffsetInfo struct {
	Name       string
	StreamName string
	TextOffset uint32
}

type Reference struct {
	Kind  string `json:"kind,omitempty"`
	Value string `json:"value,omitempty"`
}

type ProjectProtection struct {
	PasswordProtected bool   `json:"passwordProtected,omitempty"`
	CMG               string `json:"cmg,omitempty"`
	DPB               string `json:"dpb,omitempty"`
	GC                string `json:"gc,omitempty"`
}

type ProjectMetadata struct {
	Name        string                `json:"name,omitempty"`
	Description string                `json:"description,omitempty"`
	ID          string                `json:"id,omitempty"`
	Modules     map[string]ModuleType `json:"modules,omitempty"`
	References  []Reference           `json:"references,omitempty"`
	Protection  ProjectProtection     `json:"protection,omitempty"`
	RawLines    []string              `json:"rawLines,omitempty"`
}

type DirStream struct {
	Modules []Module `json:"modules,omitempty"`
}

type ProjectStreams struct {
	Project   *ProjectMetadata `json:"project,omitempty"`
	DirStream *DirStream       `json:"dirStream,omitempty"`
}
