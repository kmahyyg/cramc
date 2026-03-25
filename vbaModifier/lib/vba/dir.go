package vba

import (
	"encoding/binary"
	"strings"
)

func PatchModuleTextOffsetInDirStream(rawCompressedDir []byte, moduleName string, newOffset uint32) ([]byte, []ModuleTextOffsetInfo, error) {
	plain, err := Decompress(rawCompressedDir)
	if err != nil {
		return nil, nil, err
	}
	dirMeta, err := parseDirStream(plain)
	if err != nil {
		return nil, nil, err
	}
	patched := append([]byte(nil), plain...)
	infos := make([]ModuleTextOffsetInfo, 0, len(dirMeta.Modules))
	for idx := range dirMeta.Modules {
		module := &dirMeta.Modules[idx]
		infos = append(infos, ModuleTextOffsetInfo{
			Name:       module.Name,
			StreamName: module.StreamName,
			TextOffset: module.TextOffset,
		})
		if strings.EqualFold(module.Name, moduleName) || strings.EqualFold(module.StreamName, moduleName) {
			module.TextOffset = newOffset
			if module.offsetPos >= 0 && module.offsetPos+4 <= len(patched) {
				binary.LittleEndian.PutUint32(patched[module.offsetPos:module.offsetPos+4], newOffset)
			}
		}
	}
	compressed, err := Compress(patched)
	if err != nil {
		return nil, nil, err
	}
	return compressed, infos, nil
}

func parseDirStream(raw []byte) (*DirStream, error) {
	modules := make([]Module, 0)
	for pos := 0; pos+6 <= len(raw); {
		recordID := binary.LittleEndian.Uint16(raw[pos : pos+2])
		if recordID != 0x0019 {
			pos++
			continue
		}
		nameLen := int(binary.LittleEndian.Uint32(raw[pos+2 : pos+6]))
		nameStart := pos + 6
		nameEnd := nameStart + nameLen
		if nameLen <= 0 || nameEnd > len(raw) {
			pos++
			continue
		}
		module := Module{
			Name:      strings.TrimSpace(string(raw[nameStart:nameEnd])),
			Type:      ModuleTypeUnknown,
			offsetPos: -1,
		}
		if module.Name == "" {
			pos++
			continue
		}
		blockEnd := findNextModuleRecord(raw, nameEnd)
		parseModuleBlock(raw[nameEnd:blockEnd], nameEnd, &module)
		modules = append(modules, module)
		pos = blockEnd
	}
	if len(modules) == 0 {
		return nil, ErrDirStreamMalformed
	}
	return &DirStream{Modules: modules}, nil
}

func findNextModuleRecord(raw []byte, start int) int {
	for idx := start; idx+6 <= len(raw); idx++ {
		if binary.LittleEndian.Uint16(raw[idx:idx+2]) != 0x0019 {
			continue
		}
		length := int(binary.LittleEndian.Uint32(raw[idx+2 : idx+6]))
		if length <= 0 || idx+6+length > len(raw) {
			continue
		}
		return idx
	}
	return len(raw)
}

func parseModuleBlock(block []byte, base int, module *Module) {
	for pos := 0; pos+2 <= len(block); {
		id := binary.LittleEndian.Uint16(block[pos : pos+2])
		switch id {
		case 0x001A:
			if pos+6 > len(block) {
				return
			}
			length := int(binary.LittleEndian.Uint32(block[pos+2 : pos+6]))
			dataStart := pos + 6
			dataEnd := dataStart + length
			if length > 0 && dataEnd <= len(block) && module.StreamName == "" {
				module.StreamName = strings.TrimSpace(string(block[dataStart:dataEnd]))
			}
			pos = dataEnd
		case 0x0031:
			if pos+10 > len(block) {
				return
			}
			if binary.LittleEndian.Uint32(block[pos+2:pos+6]) == 4 {
				module.TextOffset = binary.LittleEndian.Uint32(block[pos+6 : pos+10])
				module.offsetPos = base + pos + 6
			}
			pos += 10
		case 0x0025:
			module.ReadOnly = true
			pos = advanceFixedRecord(block, pos, 6)
		case 0x0028:
			module.Private = true
			pos = advanceFixedRecord(block, pos, 6)
		case 0x0021:
			if module.Type == ModuleTypeUnknown {
				module.Type = ModuleTypeStandard
			}
			pos = advanceFixedRecord(block, pos, 6)
		case 0x0022:
			if module.Type == ModuleTypeUnknown {
				module.Type = ModuleTypeClass
			}
			pos = advanceFixedRecord(block, pos, 6)
		case 0x002B:
			return
		default:
			if pos+6 > len(block) {
				return
			}
			length := int(binary.LittleEndian.Uint32(block[pos+2 : pos+6]))
			if length < 0 || pos+6+length > len(block) {
				pos++
				continue
			}
			pos += 6 + length
		}
	}
	if module.StreamName == "" {
		module.StreamName = module.Name
	}
}

func advanceFixedRecord(block []byte, pos int, fallback int) int {
	if pos+6 <= len(block) {
		length := int(binary.LittleEndian.Uint32(block[pos+2 : pos+6]))
		if length >= 0 && pos+6+length <= len(block) {
			return pos + 6 + length
		}
	}
	if pos+fallback <= len(block) {
		return pos + fallback
	}
	return len(block)
}

// StripVBAProjectPerformanceCache removes the PerformanceCache blob from _VBA_PROJECT.
// Per MS-OVBA 2.3.4.1, bytes beyond offset 0x0007 MUST NOT be present on write.
func StripVBAProjectPerformanceCache(raw []byte) []byte {
	// directly hard-coded per standard
	// https://attack.mitre.org/techniques/T1564/007/
	// please do not directly copy first 7 bytes of P-code cache to prevent from executing
	finalOut := []byte{0xCC, 0x61, 0xFF, 0xFF, 0x00, 0x01, 0x00}
	return finalOut
}
