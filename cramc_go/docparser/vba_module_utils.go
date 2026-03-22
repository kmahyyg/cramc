package docparser

import (
	"bytes"
	"strings"
	cfbv3 "vbaModifier/lib/cfb/v3"
	"vbaModifier/lib/vba"
)

func removeSRPStreams(vbaStorage *cfbv3.Storage) int {
	removed := 0
	for _, name := range vbaStorage.ListStreams() {
		if strings.HasPrefix(name, "__SRP_") {
			if err := vbaStorage.RemoveStream(name); err == nil {
				removed++
			}
		}
	}
	return removed
}

func findTargetModule(modules []vba.ModuleTextOffsetInfo, moduleName string) *vba.ModuleTextOffsetInfo {
	for i := range modules {
		if modules[i].Name == moduleName || modules[i].StreamName == moduleName {
			return &modules[i]
		}
	}
	return nil
}

func splitVBASource(raw []byte) (attributes string, body string) {
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	var attrs strings.Builder
	var code strings.Builder

	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			continue
		}
		if strings.HasPrefix(line, "Attribute ") {
			attrs.WriteString(line)
			attrs.WriteByte('\n')
			continue
		}
		code.WriteString(line)
		code.WriteByte('\n')
	}

	return attrs.String(), code.String()
}

func buildModuleSource(attributes string, body string) []byte {
	attrs := ensureCRLFTermination(normalizeLineEndingsToCRLF(attributes))
	code := ensureCRLFTermination(normalizeLineEndingsToCRLF(strings.TrimSpace(body)))
	return []byte(attrs + code)
}

func ensureCRLFTermination(s string) string {
	if s == "" {
		return ""
	}
	if strings.HasSuffix(s, "\r\n") {
		return s
	}
	return s + "\r\n"
}

func normalizeLineEndingsToCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}

func writeModifiedCFBBytes(reader *cfbv3.Reader, outputBuf *bytes.Buffer) error {
	buf := &bytes.Buffer{}
	if _, err := reader.WriteTo(buf); err != nil {
		return err
	}
	return nil
}
