package vba

import (
	"archive/zip"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeDPBProtectionState(t *testing.T) {
	tests := []struct {
		name      string
		dpb       string
		wantState bool
		wantKnown bool
	}{
		{
			name:      "password protected marker",
			dpb:       encodeProjectPropertyForTest(t, []byte{0x1d, 0x00, 0x00, 0x00, 0xaa, 0xbb}),
			wantState: true,
			wantKnown: true,
		},
		{
			name:      "not password protected marker",
			dpb:       encodeProjectPropertyForTest(t, []byte{0x01, 0x00, 0x00, 0x00, 0x00}),
			wantState: false,
			wantKnown: true,
		},
		{
			name:      "plaintext password form",
			dpb:       encodeProjectPropertyForTest(t, []byte{0x04, 0x00, 0x00, 0x00, 'p', 'a', 's', 's'}),
			wantState: true,
			wantKnown: true,
		},
		{
			name:      "plaintext empty password",
			dpb:       encodeProjectPropertyForTest(t, []byte{0x00, 0x00, 0x00, 0x00}),
			wantState: false,
			wantKnown: true,
		},
		{
			name:      "invalid hex",
			dpb:       "not-hex",
			wantState: false,
			wantKnown: false,
		},
		{
			name:      "empty value",
			dpb:       "",
			wantState: false,
			wantKnown: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotState, gotKnown := decodeDPBProtectionState(tc.dpb)
			if gotKnown != tc.wantKnown {
				t.Fatalf("known mismatch: got %v want %v", gotKnown, tc.wantKnown)
			}
			if gotState != tc.wantState {
				t.Fatalf("state mismatch: got %v want %v", gotState, tc.wantState)
			}
		})
	}
}

func TestParseAllStreams_EncryptedRealSample(t *testing.T) {
	workbookPath := filepath.Join("..", "..", "assets", "testdata", "Pwd123456-01.xlsm")
	vbaProjectBin, err := readZipMember(workbookPath, "xl/vbaProject.bin")
	if err != nil {
		t.Fatalf("read vbaProject.bin from sample: %v", err)
	}

	rdr, err := OpenVBAProjectBytes(vbaProjectBin, false)
	if err != nil {
		t.Fatalf("open vba project: %v", err)
	}
	defer rdr.Close()

	_, err = rdr.ParseAllStreams()
	if !errors.Is(err, ErrUnsupportedEncryptedProject) {
		t.Fatalf("expected ErrUnsupportedEncryptedProject, got %v", err)
	}
}

func TestParseAllStreams_UnencryptedRealSample(t *testing.T) {
	workbookPath := filepath.Join("..", "..", "assets", "testdata", "M-01.xlsm")
	vbaProjectBin, err := readZipMember(workbookPath, "xl/vbaProject.bin")
	if err != nil {
		t.Fatalf("read vbaProject.bin from sample: %v", err)
	}

	rdr, err := OpenVBAProjectBytes(vbaProjectBin, false)
	if err != nil {
		t.Fatalf("open vba project: %v", err)
	}
	defer rdr.Close()

	streams, err := rdr.ParseAllStreams()
	if err != nil {
		t.Fatalf("expected unencrypted project to parse, got %v", err)
	}
	if streams == nil || streams.Project == nil || streams.DirStream == nil || len(streams.DirStream.Modules) == 0 {
		t.Fatalf("expected parsed streams with modules, got %+v", streams)
	}
	if streams.Project.Protection.PasswordProtected {
		t.Fatal("expected PasswordProtected=false for unencrypted sample")
	}
}

func encodeProjectPropertyForTest(t *testing.T, decoded []byte) string {
	t.Helper()

	seed := byte(0x01)
	version := byte(0x02)
	projectKey := byte(0x42)

	encoded := make([]byte, 3, len(decoded)+3)
	encoded[0] = seed
	encoded[1] = version ^ seed
	encoded[2] = projectKey ^ seed

	pb := projectKey
	for idx := 0; idx < len(decoded); idx++ {
		iter := idx + 3
		next := (encoded[iter-2] + pb) ^ decoded[idx]
		encoded = append(encoded, next)
		pb = decoded[idx]
	}

	return hex.EncodeToString(encoded)
}

func readZipMember(zipPath string, memberName string) ([]byte, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name != memberName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}

	return nil, os.ErrNotExist
}
