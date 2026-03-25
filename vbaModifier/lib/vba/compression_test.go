package vba

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestCompressRoundTripShortChunk(t *testing.T) {
	src := bytes.Repeat([]byte("Attribute VB_Name = \"ThisWorkbook\"\r\n"), 22)
	compressed, err := Compress(src)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if len(compressed) < 3 {
		t.Fatal("compressed output too short")
	}
	header := binary.LittleEndian.Uint16(compressed[1:3])
	if header&0x8000 == 0 {
		t.Fatalf("expected short chunk to use compressed-token encoding, header=%04X", header)
	}
	roundTrip, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if !bytes.Equal(roundTrip, src) {
		t.Fatal("round-trip mismatch")
	}
}

func TestCompressNeverEmitsShortRawChunk(t *testing.T) {
	src := make([]byte, 5000)
	for idx := range src {
		src[idx] = byte(idx % 251)
	}
	compressed, err := Compress(src)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	for _, chunk := range readChunkHeaders(t, compressed) {
		if chunk.flag == 0 && chunk.size != 4098 {
			t.Fatalf("raw chunk must be 4098 bytes total, got size=%d header=%04X", chunk.size, chunk.header)
		}
	}
	roundTrip, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if !bytes.Equal(roundTrip, src) {
		t.Fatal("round-trip mismatch")
	}
}

type chunkHeader struct {
	header uint16
	flag   uint16
	size   int
}

func readChunkHeaders(t *testing.T, compressed []byte) []chunkHeader {
	t.Helper()
	if len(compressed) == 0 || compressed[0] != compressedContainerSignature {
		t.Fatal("missing compressed container signature")
	}
	headers := make([]chunkHeader, 0)
	for pos := 1; pos+2 <= len(compressed); {
		header := binary.LittleEndian.Uint16(compressed[pos : pos+2])
		size := int(header&0x0FFF) + 3
		headers = append(headers, chunkHeader{
			header: header,
			flag:   (header >> 15) & 1,
			size:   size,
		})
		pos += size
	}
	return headers
}
