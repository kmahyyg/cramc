package vba

// ModuleType represents the type of a VBA module
type ModuleType string

const (
	ModuleTypeStandard ModuleType = "Module"
	ModuleTypeClass    ModuleType = "Class"
	ModuleTypeForm     ModuleType = "Form"
	ModuleTypeDocument ModuleType = "Document"
	ModuleTypeDesigner ModuleType = "Designer"
)

// ModuleInfo represents information about a VBA module
type ModuleInfo struct {
	Name       string
	Type       ModuleType // Module, Class, Document, Designer, etc.
	Path       string     // Path in the VBA storage
	DocTlibVer uint32     // Automation server version for Document modules (from Document=ModuleIdentifier/HEXINT32)
}

// Reference represents a VBA project reference
type Reference struct {
	Name        string
	Description string
	GUID        string
	Major       uint16
	Minor       uint16
}

// DirReferenceName represents the name of a referenceName record
type DirReferenceName struct {
	ReferenceType uint16
	Name          string
	// NameUnicode   string
}

// DirReference represents a reference in the dir stream
type DirReference struct {
	ReferenceType uint16
	RefName       *DirReferenceName

	// ReferenceControl
	LibidTwiddled   string
	RefNameExtended *DirReferenceName

	LibidExtended   string
	OriginalTypeLib [16]byte
	Cookie          uint32

	// ReferenceOriginal
	LibidOriginal string

	// ReferenceRegistered
	Libid string

	// ReferenceProject
	LibidAbsolute string
	LibidRelative string
	MajorVersion  uint32
	MinorVersion  uint16
}

// DirModule represents a module in the dir stream
type DirModule struct {
	// ModuleName record 0x19
	Name string
	// ModuleNameUnicode (optional) record 0x47
	NameUnicode string
	// ModuleStreamName record 0x1A
	StreamName string
	// StreamNameUnicode string

	// ModuleDocString record 0x1C
	DocString string
	//DocStringUnicode string

	// ModuleOffset record 0x31
	TextOffset uint32

	// ModuleHelpContext record, 4 bytes, topic identifier, 0x1E
	HelpContext uint32

	// ModuleCookie record, 0x2C
	// Cookie uint16

	// ModuleType record, 0x21 for procedural modules, 0x22 for document/class/designer modules
	ModuleType         uint16
	IsProceduralModule bool // if ID=0x21, true, if ID=0x22, false

	// ModuleReadOnlyRecord (optional), 0x25
	ReadOnly bool

	// ModulePrivate record (optional), 0x28
	Private bool

	// terminator, 0x002B, 2 bytes
	// reserved, 0x00000000, 4 bytes
}

// NameMapping represents a mapping between MBCS and Unicode names
type NameMapping struct {
	MBCSName    string
	UnicodeName string
}

// VBAProjectStream represents the parsed _VBA_PROJECT stream
// According to MS-OVBA section 2.3.4.1
// Note: All fields in this structure MUST be ignored on read per specification.
type VBAProjectStream struct {
	// Reserved1 (2 bytes): MUST be 0x61CC. MUST be ignored.
	Reserved1 uint16

	// Version (2 bytes): Specifies the version of VBA used to create the VBA project.
	// MUST be ignored on read. MUST be 0xFFFF on write.
	Version uint16

	// Reserved2 (1 byte): MUST be 0x00. MUST be ignored.
	Reserved2 uint8

	// Reserved3 (2 bytes): Undefined. MUST be ignored.
	Reserved3 uint16

	// PerformanceCache (variable): Implementation-specific and version-dependent performance cache.
	// The length MUST be seven bytes less than the size of _VBA_PROJECT stream.
	// MUST be ignored on read. MUST NOT be present on write.
	PerformanceCache []byte
}

// DirStream represents the parsed dir stream
// MS-OVBA Section 2.3.4.2 specifies the format
type DirStream struct {
	// PROJECTINFORMATION fields
	SysKind          uint32
	Lcid             uint32
	LcidInvoke       uint32
	CodePage         uint16
	Name             string
	DocString        string
	DocStringUnicode string
	HelpFile         string
	HelpFileUnicode  string
	HelpContext      uint32
	LibFlags         uint32
	MajorVersion     uint32
	MinorVersion     uint16
	Constants        string
	ConstantsUnicode string

	// PROJECTREFERENCES
	References []DirReference

	// PROJECTMODULES
	ModulesCount  uint16
	ProjectCookie uint64 // it's the full record, only last 2 bytes in little endian is cookie itself.
	Modules       []DirModule
}

// ParseOutputCallback is a function type for incremental output during parsing
type ParseOutputCallback func(format string, args ...interface{})
