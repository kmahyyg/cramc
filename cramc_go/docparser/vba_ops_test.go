package docparser

import (
	"encoding/binary"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"vbaModifier/common"
	"vbaModifier/fileutils"
	cfbv3 "vbaModifier/lib/cfb/v3"
	"vbaModifier/lib/vba"
)

func TestExtractVBACodeSamples(t *testing.T) {
	common.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("modern xlsm", func(t *testing.T) {
		vbaProjectBin, err := fileutils.DecompressMacroBin(filepath.Join("..", "assets", "testdata", "TEST.xlsm"))
		if err != nil {
			t.Fatalf("decompress vbaProject.bin: %v", err)
		}
		modules, err := ExtractVBACode(vbaProjectBin, false)
		if err != nil {
			t.Fatalf("extract modern VBA code: %v", err)
		}
		if len(modules) == 0 {
			t.Fatal("expected at least one module in modern workbook")
		}
	})

	t.Run("legacy xls", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join("..", "assets", "testdata", "Book1.xls"))
		if err != nil {
			t.Fatalf("read legacy workbook: %v", err)
		}
		modules, err := ExtractVBACode(raw, true)
		if err != nil {
			t.Fatalf("extract legacy VBA code: %v", err)
		}
		if len(modules) == 0 {
			t.Fatal("expected at least one module in legacy workbook")
		}
	})
}

func TestReplaceMaliciousCodeSamples(t *testing.T) {
	common.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("modern xlsm", func(t *testing.T) {
		source := filepath.Join("..", "assets", "testdata", "TEST.xlsm")
		target := filepath.Join(t.TempDir(), "TEST.xlsm")
		copyFile(t, source, target)

		vbaProjectBin, err := fileutils.DecompressMacroBin(target)
		if err != nil {
			t.Fatalf("decompress copied vbaProject.bin: %v", err)
		}
		modules, err := ExtractVBACode(vbaProjectBin, false)
		if err != nil {
			t.Fatalf("extract source modules: %v", err)
		}
		if len(modules) == 0 {
			t.Fatal("expected at least one module in copied modern workbook")
		}
		moduleName := modules[0].ModuleName

		if err := ReplaceMaliciousCode(target, []string{moduleName}, false); err != nil {
			t.Fatalf("replace code in modern workbook: %v", err)
		}

		modifiedBin, err := fileutils.DecompressMacroBin(target + ".tmp")
		if err != nil {
			t.Fatalf("decompress modified vbaProject.bin: %v", err)
		}
		assertProjectReadable(t, modifiedBin, false)
		assertRewrittenModuleBytes(t, modifiedBin, false, modules[0])
		modifiedModules, err := ExtractVBACode(modifiedBin, false)
		if err != nil {
			t.Fatalf("extract modified modules: %v", err)
		}
		assertModuleContains(t, modifiedModules, moduleName, REPLACED_VBA_SRC_CODE)
	})

	t.Run("legacy xls", func(t *testing.T) {
		source := filepath.Join("..", "assets", "testdata", "Book1.xls")
		target := filepath.Join(t.TempDir(), "Book1.xls")
		copyFile(t, source, target)

		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read copied legacy workbook: %v", err)
		}
		modules, err := ExtractVBACode(raw, true)
		if err != nil {
			t.Fatalf("extract source legacy modules: %v", err)
		}
		if len(modules) == 0 {
			t.Fatal("expected at least one module in copied legacy workbook")
		}
		moduleName := modules[0].ModuleName

		if err := ReplaceMaliciousCode(target, []string{moduleName}, true); err != nil {
			t.Fatalf("replace code in legacy workbook: %v", err)
		}

		modifiedRaw, err := os.ReadFile(target + ".tmp")
		if err != nil {
			t.Fatalf("read modified legacy workbook: %v", err)
		}
		assertProjectReadable(t, modifiedRaw, true)
		assertRewrittenModuleBytes(t, modifiedRaw, true, modules[0])
		modifiedModules, err := ExtractVBACode(modifiedRaw, true)
		if err != nil {
			t.Fatalf("extract modified legacy modules: %v", err)
		}
		assertModuleContains(t, modifiedModules, moduleName, REPLACED_VBA_SRC_CODE)
	})
}

func TestReplaceMaliciousCodeProducesCompliantDirStream(t *testing.T) {
	common.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	source := filepath.Join("..", "assets", "testdata", "M-01.xlsm")
	target := filepath.Join(t.TempDir(), "M-01.xlsm")
	copyFile(t, source, target)

	if err := ReplaceMaliciousCode(target, []string{"ThisWorkbook"}, false); err != nil {
		t.Fatalf("replace code in M-01 workbook: %v", err)
	}

	modifiedBin, err := fileutils.DecompressMacroBin(target + ".tmp")
	if err != nil {
		t.Fatalf("decompress modified vbaProject.bin: %v", err)
	}
	rdr, err := cfbv3.OpenBytes(modifiedBin)
	if err != nil {
		t.Fatalf("open modified VBA project: %v", err)
	}
	defer rdr.Close()
	root, err := rdr.OpenRootStorage()
	if err != nil {
		t.Fatalf("open CFB root storage: %v", err)
	}
	storage, err := vba.OpenVBAStorage(root, false)
	if err != nil {
		t.Fatalf("open VBA storage: %v", err)
	}
	dirReader, err := storage.OpenStream("dir")
	if err != nil {
		t.Fatalf("read dir stream: %v", err)
	}
	defer dirReader.Close()
	dirRaw, err := io.ReadAll(dirReader)
	if err != nil {
		t.Fatalf("copy dir stream: %v", err)
	}
	assertNoShortRawChunks(t, dirRaw)
}

func copyFile(t *testing.T, src string, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

func assertModuleContains(t *testing.T, modules []*common.ExtractedVBAModule, moduleName string, expected string) {
	t.Helper()
	for _, module := range modules {
		if module.ModuleName != moduleName {
			continue
		}
		if !containsString(string(module.SourceCode), expected) {
			t.Fatalf("module %s did not contain replacement marker", moduleName)
		}
		return
	}
	t.Fatalf("module %s not found after replacement", moduleName)
}

func assertProjectReadable(t *testing.T, data []byte, legacy bool) {
	t.Helper()
	rdr, err := vba.OpenVBAProjectBytes(data, legacy)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}
	defer rdr.Close()
	streams, err := rdr.ParseAllStreams()
	if err != nil {
		t.Fatalf("parse project metadata: %v", err)
	}
	if streams == nil || streams.DirStream == nil || len(streams.DirStream.Modules) == 0 {
		t.Fatal("expected parsed project to contain at least one module")
	}
}

func assertRewrittenModuleBytes(t *testing.T, data []byte, legacy bool, original *common.ExtractedVBAModule) {
	t.Helper()
	rdr, err := vba.OpenVBAProjectBytes(data, legacy)
	if err != nil {
		t.Fatalf("open project for module byte check: %v", err)
	}
	defer rdr.Close()
	rawModule, err := rdr.GetModuleContent(original.ModuleName)
	if err != nil {
		t.Fatalf("get rewritten module content: %v", err)
	}
	attributes, _ := splitVBASource(original.SourceCode)
	expectedRaw := buildModuleSource(attributes, REPLACED_VBA_SRC_CODE)
	expectedCompressed, err := vba.Compress(expectedRaw)
	if err != nil {
		t.Fatalf("compress expected rewritten module: %v", err)
	}
	if len(rawModule) != len(expectedCompressed) {
		t.Fatalf("rewritten module length mismatch: got %d want %d", len(rawModule), len(expectedCompressed))
	}
	for idx := range rawModule {
		if rawModule[idx] != expectedCompressed[idx] {
			t.Fatalf("rewritten module bytes differ at offset %d", idx)
		}
	}
}

func containsString(haystack string, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexString(haystack, needle) >= 0)
}

func indexString(haystack string, needle string) int {
	for idx := 0; idx+len(needle) <= len(haystack); idx++ {
		if haystack[idx:idx+len(needle)] == needle {
			return idx
		}
	}
	return -1
}

func assertNoShortRawChunks(t *testing.T, compressed []byte) {
	t.Helper()
	if len(compressed) == 0 || compressed[0] != 0x01 {
		t.Fatal("dir stream is missing the compressed container signature")
	}
	for pos := 1; pos+2 <= len(compressed); {
		header := binary.LittleEndian.Uint16(compressed[pos : pos+2])
		size := int(header&0x0FFF) + 3
		flag := (header >> 15) & 1
		if flag == 0 && size != 4098 {
			t.Fatalf("found non-compliant raw dir chunk: header=%04X size=%d", header, size)
		}
		pos += size
	}
}
