package vba

import (
	"bytes"
	"encoding/binary"
)

const (
	compressedContainerSignature = 0x01
	compressedChunkMaxPayload    = 4096
)

func Decompress(src []byte) ([]byte, error) {
	if len(src) == 0 {
		return nil, nil
	}
	if src[0] != compressedContainerSignature {
		return nil, ErrInvalidSignature
	}
	var dst bytes.Buffer
	pos := 1
	for pos < len(src) {
		if pos+2 > len(src) {
			return nil, ErrDecompressionFailed
		}
		header := binary.LittleEndian.Uint16(src[pos : pos+2])
		pos += 2
		chunkSize := int(header&0x0FFF) + 3
		if chunkSize < 2 || pos+chunkSize-2 > len(src) {
			return nil, ErrDecompressionFailed
		}
		compressed := (header & 0x8000) != 0
		chunkData := src[pos : pos+chunkSize-2]
		pos += chunkSize - 2
		if !compressed {
			dst.Write(chunkData)
			continue
		}
		chunkOut := make([]byte, 0, compressedChunkMaxPayload)
		cursor := 0
		for cursor < len(chunkData) {
			flags := chunkData[cursor]
			cursor++
			for bit := 0; bit < 8 && cursor < len(chunkData); bit++ {
				if flags&(1<<bit) == 0 {
					chunkOut = append(chunkOut, chunkData[cursor])
					cursor++
					if len(chunkOut) > compressedChunkMaxPayload {
						return nil, ErrDecompressionFailed
					}
					continue
				}
				if cursor+2 > len(chunkData) {
					return nil, ErrDecompressionFailed
				}
				token := binary.LittleEndian.Uint16(chunkData[cursor : cursor+2])
				cursor += 2
				offsetBits := copyTokenOffsetBits(len(chunkOut))
				lengthBits := 16 - offsetBits
				lengthMask := uint16((1 << lengthBits) - 1)
				offset := int(token>>lengthBits) + 1
				length := int(token&lengthMask) + 3
				if offset <= 0 || offset > len(chunkOut) {
					return nil, ErrDecompressionFailed
				}
				for idx := 0; idx < length; idx++ {
					chunkOut = append(chunkOut, chunkOut[len(chunkOut)-offset])
					if len(chunkOut) > compressedChunkMaxPayload {
						return nil, ErrDecompressionFailed
					}
				}
			}
		}
		dst.Write(chunkOut)
	}
	return dst.Bytes(), nil
}

func Compress(src []byte) ([]byte, error) {
	var dst bytes.Buffer
	dst.WriteByte(compressedContainerSignature)
	for len(src) > 0 {
		chunkLen := compressedChunkMaxPayload
		if len(src) < chunkLen {
			chunkLen = len(src)
		}
		chunk := src[:chunkLen]
		src = src[chunkLen:]
		chunkData, compressed := compressChunk(chunk)
		if !compressed {
			chunkData = make([]byte, compressedChunkMaxPayload)
			copy(chunkData, chunk)
		}
		size := len(chunkData) + 2
		header := uint16(0x3000 | ((size - 3) & 0x0FFF))
		if compressed {
			header |= 0x8000
		}
		if err := binary.Write(&dst, binary.LittleEndian, header); err != nil {
			return nil, ErrCompressionFailed
		}
		if _, err := dst.Write(chunkData); err != nil {
			return nil, ErrCompressionFailed
		}
	}
	return dst.Bytes(), nil
}

func compressChunk(chunk []byte) ([]byte, bool) {
	encoded := make([]byte, 0, len(chunk))
	for pos := 0; pos < len(chunk); {
		flagPos := len(encoded)
		encoded = append(encoded, 0)
		var flags byte
		for bit := 0; bit < 8 && pos < len(chunk); bit++ {
			offset, length := findBestMatch(chunk, pos)
			if offset > 0 && length >= 3 {
				lengthBits := 16 - copyTokenOffsetBits(pos)
				token := uint16(offset-1)<<lengthBits | uint16(length-3)
				encoded = append(encoded, byte(token), byte(token>>8))
				flags |= 1 << bit
				pos += length
				continue
			}
			encoded = append(encoded, chunk[pos])
			pos++
		}
		encoded[flagPos] = flags
		if len(encoded) > compressedChunkMaxPayload {
			return nil, false
		}
	}
	return encoded, true
}

func findBestMatch(chunk []byte, pos int) (int, int) {
	if pos < 1 {
		return 0, 0
	}
	bestOffset := 0
	bestLength := 0
	_, _, maxLength := copyTokenLayout(pos)
	for candidate := pos - 1; candidate >= 0; candidate-- {
		length := 0
		for pos+length < len(chunk) && chunk[candidate+length] == chunk[pos+length] {
			length++
		}
		if length > bestLength {
			bestLength = length
			bestOffset = pos - candidate
			if bestLength >= maxLength {
				bestLength = maxLength
				break
			}
		}
	}
	if bestLength < 3 {
		return 0, 0
	}
	if bestLength > maxLength {
		bestLength = maxLength
	}
	return bestOffset, bestLength
}

func copyTokenOffsetBits(current int) int {
	bits := 4
	for bits < 12 && (1<<bits) < current {
		bits++
	}
	return bits
}

func copyTokenLayout(current int) (int, uint16, int) {
	offsetBits := copyTokenOffsetBits(current)
	lengthBits := 16 - offsetBits
	lengthMask := uint16((1 << lengthBits) - 1)
	return offsetBits, lengthMask, int(lengthMask) + 3
}
