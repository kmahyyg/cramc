package vba

import (
	"bufio"
	"bytes"
	"io"
	"strings"

	cfbv3 "vbaModifier/lib/cfb/v3"
)

type Reader struct {
	cfb         *cfbv3.Reader
	root        *cfbv3.Storage
	projectRoot *cfbv3.Storage
	vbaStorage  *cfbv3.Storage
	legacy      bool
	parsed      *ProjectStreams
}

func OpenVBAProjectBytes(vbaProjectBin []byte, isLegacyFormat bool) (*Reader, error) {
	cfbReader, err := cfbv3.OpenBytes(vbaProjectBin)
	if err != nil {
		return nil, err
	}
	root, err := cfbReader.OpenRootStorage()
	if err != nil {
		_ = cfbReader.Close()
		return nil, err
	}
	projectRoot := root
	if isLegacyFormat {
		projectRoot, err = root.OpenStorage("_VBA_PROJECT_CUR")
		if err != nil {
			_ = cfbReader.Close()
			return nil, ErrProjectStorageNotFound
		}
	}
	vbaStorage, err := OpenVBAStorage(root, isLegacyFormat)
	if err != nil {
		_ = cfbReader.Close()
		return nil, err
	}
	return &Reader{
		cfb:         cfbReader,
		root:        root,
		projectRoot: projectRoot,
		vbaStorage:  vbaStorage,
		legacy:      isLegacyFormat,
	}, nil
}

func (r *Reader) Close() error {
	if r.cfb == nil {
		return nil
	}
	err := r.cfb.Close()
	r.cfb = nil
	r.root = nil
	r.projectRoot = nil
	r.vbaStorage = nil
	r.parsed = nil
	return err
}

func OpenVBAStorage(root *cfbv3.Storage, isLegacyFormat bool) (*cfbv3.Storage, error) {
	base := root
	var err error
	if isLegacyFormat {
		base, err = root.OpenStorage("_VBA_PROJECT_CUR")
		if err != nil {
			return nil, ErrProjectStorageNotFound
		}
	}
	vbaStorage, err := base.OpenStorage("VBA")
	if err != nil {
		return nil, ErrProjectStorageNotFound
	}
	return vbaStorage, nil
}

func (r *Reader) ParseAllStreams() (*ProjectStreams, error) {
	if r.parsed != nil {
		return r.parsed, nil
	}
	projectBytes, err := readStorageStream(r.projectRoot, "PROJECT")
	if err != nil {
		return nil, ErrProjectMetadataStreamMissing
	}
	projectMeta := parseProjectMetadata(projectBytes)
	if r.projectRoot.StreamExists("PROJECTlk") {
		projectMeta.Protection.PasswordProtected = true
	}
	if projectMeta.Protection.PasswordProtected {
		return nil, ErrUnsupportedEncryptedProject
	}
	dirRaw, err := readStorageStream(r.vbaStorage, "dir")
	if err != nil {
		return nil, err
	}
	dirPlain, err := Decompress(dirRaw)
	if err != nil {
		return nil, err
	}
	dirMeta, err := parseDirStream(dirPlain)
	if err != nil {
		return nil, err
	}
	mergeModuleTypes(dirMeta.Modules, projectMeta.Modules)
	r.parsed = &ProjectStreams{
		Project:   projectMeta,
		DirStream: dirMeta,
	}
	return r.parsed, nil
}

func (r *Reader) GetModuleContent(name string) ([]byte, error) {
	streamNames := []string{name}
	if r.parsed != nil && r.parsed.DirStream != nil {
		for _, mod := range r.parsed.DirStream.Modules {
			if strings.EqualFold(mod.Name, name) || strings.EqualFold(mod.StreamName, name) {
				if mod.StreamName != "" && !strings.EqualFold(mod.StreamName, name) {
					streamNames = append(streamNames, mod.StreamName)
				}
				break
			}
		}
	}
	for _, streamName := range streamNames {
		if streamName == "" {
			continue
		}
		content, err := readStorageStream(r.vbaStorage, streamName)
		if err == nil {
			return content, nil
		}
	}
	return nil, ErrModuleNotFound
}

func readStorageStream(storage *cfbv3.Storage, name string) ([]byte, error) {
	rdr, err := storage.OpenStream(name)
	if err != nil {
		return nil, err
	}
	defer rdr.Close()
	return io.ReadAll(rdr)
}

func parseProjectMetadata(raw []byte) *ProjectMetadata {
	metadata := &ProjectMetadata{
		Modules: make(map[string]ModuleType),
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimRight(scanner.Text(), "\r"))
		if line == "" {
			continue
		}
		metadata.RawLines = append(metadata.RawLines, line)
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, "\"")
		switch {
		case strings.EqualFold(key, "Name"):
			metadata.Name = value
		case strings.EqualFold(key, "Description"):
			metadata.Description = value
		case strings.EqualFold(key, "ID"):
			metadata.ID = value
		case strings.EqualFold(key, "Module"):
			metadata.Modules[value] = ModuleTypeStandard
		case strings.EqualFold(key, "Class"):
			metadata.Modules[value] = ModuleTypeClass
		case strings.EqualFold(key, "BaseClass"):
			metadata.Modules[value] = ModuleTypeDesigner
		case strings.EqualFold(key, "Designer"):
			metadata.Modules[value] = ModuleTypeDesigner
		case strings.EqualFold(key, "Document"):
			metadata.Modules[parseDocumentModuleName(value)] = ModuleTypeDocument
		case strings.HasPrefix(strings.ToLower(key), "reference"):
			metadata.References = append(metadata.References, Reference{Kind: key, Value: value})
		case strings.EqualFold(key, "CMG"):
			metadata.Protection.CMG = value
		case strings.EqualFold(key, "DPB"):
			metadata.Protection.DPB = value
		case strings.EqualFold(key, "GC"):
			metadata.Protection.GC = value
		}
	}
	return metadata
}

func parseDocumentModuleName(value string) string {
	name, _, ok := strings.Cut(value, "/")
	if ok {
		return strings.TrimSpace(name)
	}
	return strings.TrimSpace(value)
}

func mergeModuleTypes(modules []Module, projectTypes map[string]ModuleType) {
	for idx := range modules {
		if moduleType, ok := projectTypes[modules[idx].Name]; ok {
			modules[idx].Type = moduleType
			continue
		}
		if moduleType, ok := projectTypes[modules[idx].StreamName]; ok {
			modules[idx].Type = moduleType
		}
	}
}
