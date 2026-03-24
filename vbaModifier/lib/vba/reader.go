package vba

import (
	"fmt"
	"io"
	"strings"

	cfbv3 "vbaModifier/lib/cfb/v3"
)

// VBAReader provides a high-level interface for reading VBA projects
type VBAReader struct {
	cfbReader  *cfbv3.Reader
	project    *Project
	projectwm  *ProjectWM
	vbaProject *VBAProjectStream
	dirStream  *DirStream
	password   string
	isLegacy   bool
	vbaRoot    *cfbv3.Storage
}

// VBAProject represents a fully parsed VBA project
type VBAProject struct {
	Project    *Project
	ProjectWM  *ProjectWM
	VBAProject *VBAProjectStream
	DirStream  *DirStream
	Modules    []ModuleInfo
}

// OpenVBAProject opens and parses a vbaProject.bin file
func OpenVBAProject(filename string, isLegacyFormat bool) (*VBAReader, error) {
	// Open CFB file
	cfbReader, err := cfbv3.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open CFB file: %w", err)
	}

	return &VBAReader{
		cfbReader: cfbReader,
		password:  "", // Default to no password
		isLegacy:  isLegacyFormat,
	}, nil
}

// OpenVBAProjectBytes opens and parses a vbaProject.bin payload from memory.
func OpenVBAProjectBytes(data []byte, isLegacyFormat bool) (*VBAReader, error) {
	cfbReader, err := cfbv3.OpenBytes(data)
	if err != nil {
		return nil, fmt.Errorf("failed to open CFB data: %w", err)
	}

	return &VBAReader{
		cfbReader: cfbReader,
		password:  "", // Default to no password
		isLegacy:  isLegacyFormat,
	}, nil
}

// Close closes the underlying CFB reader
func (r *VBAReader) Close() error {
	if r.cfbReader != nil {
		return r.cfbReader.Close()
	}
	return nil
}

// SetPassword sets the password for decrypting encrypted modules
func (r *VBAReader) SetPassword(password string) {
	r.password = password
}

// ParseAll parses all three streams (PROJECT, PROJECTwm, dir)
func (r *VBAReader) ParseAll() (*VBAProject, error) {
	root, err := r.cfbReader.OpenRootStorage()
	if err != nil {
		return nil, fmt.Errorf("failed to open root storage: %w", err)
	}

	var legacyRoot *cfbv3.Storage
	var exactRoot *cfbv3.Storage = root
	if r.isLegacy {
		legacyRoot, err = root.OpenStorage("_VBA_PROJECT_CUR")
		if err != nil {
			return nil, fmt.Errorf("open _VBA_PROJECT_CUR: %w", err)
		}
		exactRoot = legacyRoot
	}

	// Parse PROJECT stream
	projectStream, err := exactRoot.OpenStream("PROJECT")
	if err != nil {
		return nil, fmt.Errorf("failed to open PROJECT stream: %w", err)
	}
	defer projectStream.Close()

	project, err := ParseProject(projectStream)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PROJECT stream: %w", err)
	}
	r.project = project

	// Parse PROJECTwm stream (optional - may not exist)
	projectwmStream, err := exactRoot.OpenStream("PROJECTwm")
	if err == nil {
		defer projectwmStream.Close()
		projectwm, err := ParseProjectWM(projectwmStream)
		if err == nil {
			r.projectwm = projectwm
		}
		// Ignore errors for PROJECTwm as it's optional
	}

	// Parse _VBA_PROJECT stream (optional - located in VBA storage)
	vbaStorage, err := OpenVBAStorage(root, r.isLegacy)
	if err != nil {
		return nil, fmt.Errorf("failed to open VBA storage: %w", err)
	}

	vbaProjectStream, err := vbaStorage.OpenStream("_VBA_PROJECT")
	if err == nil {
		defer vbaProjectStream.Close()
		vbaProj, err := ParseVBAProject(vbaProjectStream)
		if err == nil {
			r.vbaProject = vbaProj
		}
		// Ignore errors for _VBA_PROJECT as it may not exist in all files
	}

	// Parse dir stream
	dirStream, err := vbaStorage.OpenStream("dir")
	if err != nil {
		return nil, fmt.Errorf("failed to open dir stream: %w", err)
	}
	defer dirStream.Close()

	dirData, err := io.ReadAll(dirStream)
	if err != nil {
		return nil, fmt.Errorf("failed to read dir stream: %w", err)
	}

	dir, err := ParseDirStream(dirData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dir stream: %w", err)
	}
	r.dirStream = dir

	// Combine module information
	modules := make([]ModuleInfo, 0)
	moduleMap := make(map[string]bool)

	// Add modules from PROJECT stream
	for _, module := range project.Modules {
		module.Path = r.normalizeModulePath(module.Path)
		if !moduleMap[module.Name] {
			modules = append(modules, module)
			moduleMap[module.Name] = true
		}
	}

	// Add modules from dir stream
	for _, dirModule := range dir.Modules {
		if !moduleMap[dirModule.Name] {
			moduleInfo := ModuleInfo{
				Name: dirModule.Name,
				Type: ModuleTypeStandard, // Default type
				Path: r.modulePath(dirModule.StreamName),
			}
			modules = append(modules, moduleInfo)
			moduleMap[dirModule.Name] = true
		}
	}

	return &VBAProject{
		Project:    project,
		ProjectWM:  r.projectwm,
		VBAProject: r.vbaProject,
		DirStream:  dir,
		Modules:    modules,
	}, nil
}

// GetModuleStream opens the stream for a specific module
// Returns decrypted content if the module is encrypted
func (r *VBAReader) GetModuleStream(moduleName string) (cfbv3.Stream, error) {
	if r.dirStream == nil {
		return nil, fmt.Errorf("dir stream not parsed, call ParseAll() first")
	}

	root, err := r.cfbReader.OpenRootStorage()
	if err != nil {
		return nil, fmt.Errorf("failed to open root storage: %w", err)
	}

	vbaStorage, err := OpenVBAStorage(root, r.isLegacy)
	if err != nil {
		return nil, fmt.Errorf("failed to open VBA storage: %w", err)
	}

	// Get stream name from dir stream
	streamName := r.dirStream.GetModuleStreamName(moduleName)
	if streamName == "" {
		// Fallback to module name
		streamName = moduleName
	}

	stream, err := vbaStorage.OpenStream(streamName)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrModuleNotFound, err)
	}

	// Check if decryption is needed
	// For now, return the stream as-is
	// Decryption will be handled in GetModuleContent
	return stream, nil
}

// GetModuleContent returns the decrypted source code for a module
func (r *VBAReader) GetModuleContent(moduleName string) ([]byte, error) {
	stream, err := r.GetModuleStream(moduleName)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	// Read raw content
	rawContent, err := io.ReadAll(stream)
	if err != nil {
		return nil, fmt.Errorf("failed to read module stream: %w", err)
	}

	// Check if encryption is needed
	if IsEncrypted(rawContent) {
		if r.password == "" {
			return nil, ErrPasswordRequired
		}

		// Decrypt the content
		decryptor, err := NewDecryptor(r.password, r.dirStream)
		if err != nil {
			return nil, fmt.Errorf("failed to create decryptor: %w", err)
		}

		decrypted, err := decryptor.DecryptStream(rawContent)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
		}

		return decrypted, nil
	}

	return rawContent, nil
}

// ListModules returns all module names
func (r *VBAReader) ListModules() []string {
	modules := make([]string, 0)
	moduleMap := make(map[string]bool)

	if r.project != nil {
		for _, module := range r.project.Modules {
			if !moduleMap[module.Name] {
				modules = append(modules, module.Name)
				moduleMap[module.Name] = true
			}
		}
	}

	if r.dirStream != nil {
		for _, module := range r.dirStream.Modules {
			if !moduleMap[module.Name] {
				modules = append(modules, module.Name)
				moduleMap[module.Name] = true
			}
		}
	}

	return modules
}

// ListVBAStreams filters out __SRP streams when listing
func (r *VBAReader) ListVBAStreams() []string {
	root, err := r.cfbReader.OpenRootStorage()
	if err != nil {
		return []string{}
	}

	vbaStorage, err := OpenVBAStorage(root, r.isLegacy)
	if err != nil {
		return []string{}
	}

	allStreams := vbaStorage.ListStreams()
	var filtered []string
	for _, name := range allStreams {
		if !strings.HasPrefix(name, "__SRP") {
			filtered = append(filtered, name)
		}
	}
	return filtered
}

func OpenVBAStorage(root *cfbv3.Storage, isLegacy bool) (*cfbv3.Storage, error) {
	if !isLegacy {
		return root.OpenStorage("VBA")
	}

	legacyRoot, err := root.OpenStorage("_VBA_PROJECT_CUR")
	if err != nil {
		return nil, fmt.Errorf("open _VBA_PROJECT_CUR: %w", err)
	}
	return legacyRoot.OpenStorage("VBA")
}

func (r *VBAReader) moduleRootPath() string {
	if r.isLegacy {
		return "/_VBA_PROJECT_CUR/VBA"
	}
	return "/VBA"
}

func (r *VBAReader) modulePath(streamName string) string {
	return fmt.Sprintf("%s/%s", r.moduleRootPath(), streamName)
}

func (r *VBAReader) normalizeModulePath(path string) string {
	if path == "/VBA" {
		return r.moduleRootPath()
	}
	if strings.HasPrefix(path, "/VBA/") {
		return fmt.Sprintf("%s%s", r.moduleRootPath(), path[len("/VBA"):])
	}
	return path
}
