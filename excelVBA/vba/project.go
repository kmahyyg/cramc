package vba

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Project represents the parsed PROJECT stream
type Project struct {
	Name            string
	HelpFile        string
	HelpContextID   uint32
	Description     string
	Version         string
	Modules         []ModuleInfo
	References      []Reference
	HostExtender    map[string]string
	Workspace       map[string]string
	
	// Encryption fields (raw hex strings from PROJECT stream)
	CMGRaw string // ProjectProtectionState (encrypted)
	DPBRaw string // ProjectPassword (encrypted)
	GCRaw  string // ProjectVisibilityState (encrypted)
	
	// Decrypted encryption data
	CMG *EncryptedData // ProjectProtectionState (decrypted)
	DPB *EncryptedData // ProjectPassword (decrypted)
	GC  *EncryptedData // ProjectVisibilityState (decrypted)
	
	// Encryption status
	IsEncrypted bool
	EncryptionReason string
}

// ParseProject parses a PROJECT stream from an io.Reader
func ParseProject(stream io.Reader) (*Project, error) {
	project := &Project{
		Modules:      []ModuleInfo{},
		References:   []Reference{},
		HostExtender: make(map[string]string),
		Workspace:    make(map[string]string),
	}

	scanner := bufio.NewScanner(stream)
	currentSection := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Check for section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line[1 : len(line)-1]
			continue
		}

		// Parse key-value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // Skip malformed lines
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Handle different sections and keys
		switch currentSection {
		case "Host Extender Info":
			project.HostExtender[key] = value
		case "Workspace":
			project.Workspace[key] = value
		default:
			// Handle root-level keys
			switch key {
			case "ID":
				// Project ID (ignored for now)
			case "Document":
				// ProjectDocModule: "Document=" ModuleIdentifier %x2f DocTlibVer
				// Format: Document=ModuleIdentifier/HEXINT32
				moduleInfo := parseDocumentModule(value)
				if moduleInfo != nil {
					project.Modules = append(project.Modules, *moduleInfo)
				}
			case "Package":
				// Package reference
			case "BaseClass":
				// ProjectDesignerModule: "BaseClass=" ModuleIdentifier
				moduleInfo := ModuleInfo{
					Name: value,
					Type: ModuleTypeDesigner,
					Path: fmt.Sprintf("/VBA/%s", value),
				}
				project.Modules = append(project.Modules, moduleInfo)
			case "HelpFile":
				project.HelpFile = value
			case "Name":
				project.Name = value
			case "HelpContextID":
				// Parse as uint32 (simplified)
				// In practice, this should be parsed properly
			case "Description":
				project.Description = value
			case "VersionCompatible32":
				project.Version = value
			case "CMG":
				// ProjectProtectionState (encrypted)
				project.CMGRaw = value
				decrypted, err := DecryptProjectField(value)
				if err == nil {
					project.CMG = decrypted
				}
			case "DPB":
				// ProjectPassword (encrypted)
				project.DPBRaw = value
				decrypted, err := DecryptProjectField(value)
				if err == nil {
					project.DPB = decrypted
				}
			case "GC":
				// ProjectVisibilityState (encrypted)
				project.GCRaw = value
				decrypted, err := DecryptProjectField(value)
				if err == nil {
					project.GC = decrypted
				}
			case "Module":
				// ProjectStdModule: "Module=" ModuleIdentifier
				moduleInfo := ModuleInfo{
					Name: value,
					Type: ModuleTypeStandard,
					Path: fmt.Sprintf("/VBA/%s", value),
				}
				project.Modules = append(project.Modules, moduleInfo)
			case "Class":
				// ProjectClassModule: "Class=" ModuleIdentifier
				moduleInfo := ModuleInfo{
					Name: value,
					Type: ModuleTypeClass,
					Path: fmt.Sprintf("/VBA/%s", value),
				}
				project.Modules = append(project.Modules, moduleInfo)
			case "Reference":
				// Parse reference
				ref := parseReference(value)
				if ref != nil {
					project.References = append(project.References, *ref)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidProjectFormat, err)
	}

	// Determine encryption status
	project.IsEncrypted, project.EncryptionReason = IsProjectEncrypted(project.CMG, project.DPB, project.GC)

	return project, nil
}

// parseDocumentModule parses a Document module entry
// Format: Document=ModuleIdentifier/HEXINT32 or Document=ModuleIdentifier/&HHEXINT32
// According to MS-OVBA: ProjectDocModule = "Document=" ModuleIdentifier %x2f DocTlibVer
func parseDocumentModule(value string) *ModuleInfo {
	// Split by "/" to separate ModuleIdentifier and DocTlibVer
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 {
		// If no "/" found, treat as just module identifier with DocTlibVer = 0
		return &ModuleInfo{
			Name:      value,
			Type:      ModuleTypeDocument,
			Path:      fmt.Sprintf("/VBA/%s", value),
			DocTlibVer: 0,
		}
	}

	moduleIdentifier := strings.TrimSpace(parts[0])
	docTlibVerStr := strings.TrimSpace(parts[1])

	// Parse DocTlibVer - can be in format &HXXXXXXXX or just HEXINT32
	var docTlibVer uint32 = 0
	if strings.HasPrefix(docTlibVerStr, "&H") || strings.HasPrefix(docTlibVerStr, "&h") {
		// Format: &HXXXXXXXX
		hexStr := docTlibVerStr[2:] // Remove "&H" or "&h"
		if parsed, err := strconv.ParseUint(hexStr, 16, 32); err == nil {
			docTlibVer = uint32(parsed)
		}
	} else {
		// Try parsing as hex without prefix
		if parsed, err := strconv.ParseUint(docTlibVerStr, 16, 32); err == nil {
			docTlibVer = uint32(parsed)
		}
	}

	return &ModuleInfo{
		Name:      moduleIdentifier,
		Type:      ModuleTypeDocument,
		Path:      fmt.Sprintf("/VBA/%s", moduleIdentifier),
		DocTlibVer: docTlibVer,
	}
}

// parseReference parses a reference string
// Format: Reference=*\G{XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX}#X.X#X.X#path
func parseReference(refStr string) *Reference {
	// Simplified reference parsing
	// Full implementation would parse GUID, versions, and path
	parts := strings.Split(refStr, "#")
	if len(parts) < 3 {
		return nil
	}

	// Extract GUID from first part
	guidPart := parts[0]
	if strings.HasPrefix(guidPart, "*\\G{") && strings.HasSuffix(guidPart, "}") {
		guid := guidPart[4 : len(guidPart)-1]
		ref := &Reference{
			GUID: guid,
		}

		// Parse versions if available
		if len(parts) >= 3 {
			// Major.Minor format
			versionParts := strings.Split(parts[1], ".")
			if len(versionParts) >= 1 {
				// Parse major (simplified)
			}
			if len(versionParts) >= 2 {
				// Parse minor (simplified)
			}
		}

		// Description is typically in the last part
		if len(parts) >= 3 {
			ref.Description = parts[len(parts)-1]
		}

		return ref
	}

	return nil
}

// GetModuleNames returns all module names in the project
func (p *Project) GetModuleNames() []string {
	names := make([]string, len(p.Modules))
	for i, module := range p.Modules {
		names[i] = module.Name
	}
	return names
}

// FindReference finds a reference by GUID
func (p *Project) FindReference(guid string) *Reference {
	for i := range p.References {
		if p.References[i].GUID == guid {
			return &p.References[i]
		}
	}
	return nil
}
