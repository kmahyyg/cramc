package vba

import (
	"fmt"
)

const maxChunkDecompressedSize = 4096

// Compress encodes raw bytes into an MS-OVBA CompressedContainer (2.4.1).
func Compress(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return []byte{0x01}, nil
	}

	container := make([]byte, 0, len(raw)+8)
	container = append(container, 0x01) // CompressedContainer signature

	for offset := 0; offset < len(raw); {
		remaining := len(raw) - offset
		chunkPlainSize := remaining
		if chunkPlainSize > maxChunkDecompressedSize {
			chunkPlainSize = maxChunkDecompressedSize
		}

		chunkData, usedPlainSize, err := compressChunkToFit(raw[offset : offset+chunkPlainSize])
		if err != nil {
			return nil, err
		}

		// Header format:
		// bits 0-11: CompressedChunkSize (chunk size including 2-byte header minus 3).
		// bits 12-14: signature 0b011.
		// bit 15: compressed flag (1).
		header := uint16(len(chunkData)-1) | 0x3000 | 0x8000
		container = append(container, byte(header), byte(header>>8))
		container = append(container, chunkData...)

		offset += usedPlainSize
	}

	return container, nil
}

func compressChunkToFit(raw []byte) ([]byte, int, error) {
	if len(raw) == 0 {
		return nil, 0, fmt.Errorf("cannot compress empty chunk")
	}

	chunkSize := len(raw)
	for chunkSize > 0 {
		chunkData, err := compressChunk(raw[:chunkSize])
		if err != nil {
			return nil, 0, err
		}
		if len(chunkData) <= maxChunkDecompressedSize {
			return chunkData, chunkSize, nil
		}

		// Reduce in token-sequence units so we can eventually satisfy the 4096-byte chunk-data ceiling.
		if chunkSize > 8 {
			chunkSize -= 8
		} else {
			chunkSize--
		}
	}

	return nil, 0, fmt.Errorf("failed to compress chunk to <= %d bytes", maxChunkDecompressedSize)
}

func compressChunk(raw []byte) ([]byte, error) {
	out := make([]byte, 0, len(raw))
	i := 0

	for i < len(raw) {
		flagPos := len(out)
		out = append(out, 0x00) // placeholder for flag byte
		var flag byte

		for bit := 0; bit < 8 && i < len(raw); bit++ {
			distance := i
			if distance == 0 {
				distance = 1
			}

			bestOffset := 0
			bestLength := 0
			if i > 0 {
				lengthBits, offsetBits, err := copyTokenBitCounts(distance)
				if err != nil {
					return nil, err
				}

				maxOffset := i
				if maxOffset > (1 << offsetBits) {
					maxOffset = 1 << offsetBits
				}
				maxLength := (1 << lengthBits) + 2
				if maxLength > len(raw)-i {
					maxLength = len(raw) - i
				}

				for offsetCandidate := 1; offsetCandidate <= maxOffset; offsetCandidate++ {
					matched := overlapMatchLength(raw, i, offsetCandidate, maxLength)
					if matched >= 3 && matched > bestLength {
						bestLength = matched
						bestOffset = offsetCandidate
						if matched == maxLength {
							break
						}
					}
				}
			}

			if bestLength >= 3 {
				token, err := packCopyToken(bestOffset, bestLength, distance)
				if err != nil {
					return nil, err
				}
				flag |= 1 << bit
				out = append(out, byte(token), byte(token>>8))
				i += bestLength
				continue
			}

			out = append(out, raw[i])
			i++
		}

		out[flagPos] = flag
	}

	return out, nil
}

func overlapMatchLength(raw []byte, pos int, offset int, maxLength int) int {
	matched := 0
	for matched < maxLength && pos+matched < len(raw) {
		expected := raw[pos-offset+(matched%offset)]
		if raw[pos+matched] != expected {
			break
		}
		matched++
	}
	return matched
}

// Decompress decompresses data using MS-OVBA compression algorithm
// MS-OVBA Section 2.4.1 specifies a chunk-based compression with Token Sequences
// Reference: https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-ovba/4742b896-b32b-4eb0-8372-fbf01e3c65fd
func Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	// Check for CompressedContainer signature byte (0x01)
	if len(data) < 1 {
		return nil, fmt.Errorf("%w: data too short", ErrDecompressionFailed)
	}

	// Check compression signature
	if data[0] != 0x01 {
		// Uncompressed data - return as-is (skip signature byte if present)
		if len(data) > 1 {
			return data[1:], nil
		}
		return data, nil
	}

	// Initialize decompressed buffer
	decompressed := make([]byte, 0)
	offset := 1 // Skip signature byte

	// Process chunks until we've read all data
	for offset < len(data) {
		// Read chunk header (2 bytes)
		if offset+2 > len(data) {
			return nil, fmt.Errorf("%w: incomplete chunk header", ErrDecompressionFailed)
		}

		header := uint16(data[offset]) | (uint16(data[offset+1]) << 8)
		offset += 2

		// Extract header fields
		// Bits 0-11: CompressedChunkSize (size of entire compressed chunk minus 3, includes header)
		totalChunkSize := int((header & 0x0FFF) + 3)
		// Bits 12-14: CompressedChunkSignature (must be 0b011)
		signature := (header >> 12) & 0x07
		// Bit 15: CompressedChunkFlag (1 = compressed, 0 = uncompressed)
		isCompressed := (header >> 15) & 0x01

		// Validate signature
		if signature != 0x03 {
			return nil, fmt.Errorf("%w: invalid chunk signature (expected 0x03, got 0x%02x)", ErrDecompressionFailed, signature)
		}

		// The totalChunkSize includes the 2-byte header, so chunk data size is totalChunkSize - 2
		chunkDataSize := totalChunkSize - 2

		// Check if we have enough data for this chunk
		if offset+chunkDataSize > len(data) {
			return nil, fmt.Errorf("%w: incomplete chunk data (expected %d bytes, got %d)", ErrDecompressionFailed, chunkDataSize, len(data)-offset)
		}

		chunkData := data[offset : offset+chunkDataSize]
		offset += chunkDataSize

		// Decompress or copy chunk
		if isCompressed == 1 {
			// Compressed chunk - decompress it
			chunkDecompressed, err := decompressChunk(chunkData, len(decompressed), decompressed)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrDecompressionFailed, err)
			}
			decompressed = append(decompressed, chunkDecompressed...)
		} else {
			// Uncompressed chunk - copy directly (should be 4096 bytes)
			if len(chunkData) != 4096 {
				return nil, fmt.Errorf("%w: uncompressed chunk size mismatch (expected 4096, got %d)", ErrDecompressionFailed, len(chunkData))
			}
			decompressed = append(decompressed, chunkData...)
		}
	}

	return decompressed, nil
}

// decompressChunk decompresses a single compressed chunk
// chunkStartOffset is the offset in the decompressed buffer where this chunk starts
// decompressedSoFar is the full decompressed buffer so far (for cross-chunk references)
func decompressChunk(chunkData []byte, chunkStartOffset int, decompressedSoFar []byte) ([]byte, error) {
	// Each chunk typically decompresses to 4096 bytes, but the last chunk may be shorter
	result := make([]byte, 0, 4096)
	chunkOffset := 0

	// Process Token Sequences until we've decompressed 4096 bytes or run out of compressed data
	// According to MS-OVBA spec, we process token sequences until we've decompressed 4096 bytes
	// (or until we run out of compressed data for the last chunk)
	compressedEnd := len(chunkData)

	for len(result) < 4096 {
		// Read flag byte - check if we have data available
		if chunkOffset >= compressedEnd {
			// End of compressed data - this is valid for the last chunk
			// Return what we've decompressed so far
			break
		}

		flagByte := chunkData[chunkOffset]
		chunkOffset++

		// Process 8 tokens (bits 0-7 of flag byte)
		// According to spec: "IF CompressedCurrent is LESS THAN CompressedEnd THEN"
		// We check this for each token individually
		for bit := 0; bit < 8 && len(result) < 4096; bit++ {
			// Check if we have data available before processing this token
			if chunkOffset >= compressedEnd {
				// End of compressed data reached - this is valid for the last chunk
				break
			}

			// Check if this token is a literal (bit = 0) or copy (bit = 1)
			isCopyToken := (flagByte & (1 << bit)) != 0

			if isCopyToken {
				// Copy token (2 bytes)
				if chunkOffset+2 > compressedEnd {
					return nil, fmt.Errorf("incomplete copy token")
				}

				copyToken := uint16(chunkData[chunkOffset]) | (uint16(chunkData[chunkOffset+1]) << 8)
				chunkOffset += 2

				// Unpack copy token to get offset and length
				// Distance is from chunk start to current position within this chunk
				distance := len(result)
				if distance == 0 {
					// At start of chunk, use minimum distance of 1 for bit allocation
					distance = 1
				}
				offset, length, err := unpackCopyToken(copyToken, distance)
				if err != nil {
					return nil, fmt.Errorf("failed to unpack copy token: %v", err)
				}

				// Copy from decompressed buffer
				// Offset is relative to current position in full decompressed buffer
				// Current position in full buffer = chunkStartOffset + len(result)
				currentPos := chunkStartOffset + len(result)
				sourcePos := currentPos - offset

				if sourcePos < 0 {
					return nil, fmt.Errorf("invalid copy offset: %d (current position: %d)", offset, currentPos)
				}

				// Copy the sequence
				// We need to handle copying from both previous chunks and current chunk
				// Note: We can copy from bytes we're currently writing (self-referential copy)
				for i := 0; i < length && len(result) < 4096; i++ {
					srcPos := sourcePos + i

					// Determine if source is in previous chunks or current chunk
					var byteToCopy byte
					if srcPos < chunkStartOffset {
						// Source is in previous chunks
						if srcPos < 0 || srcPos >= len(decompressedSoFar) {
							return nil, fmt.Errorf("copy source out of bounds in previous chunks: pos %d, prevChunks len %d", srcPos, len(decompressedSoFar))
						}
						byteToCopy = decompressedSoFar[srcPos]
					} else {
						// Source is in current chunk (or bytes we're currently writing)
						relativeOffset := srcPos - chunkStartOffset
						// Allow copying from bytes we just wrote (relativeOffset can equal len(result) during copy)
						if relativeOffset < 0 {
							return nil, fmt.Errorf("copy source out of bounds in current chunk: relativeOffset %d (srcPos=%d, chunkStart=%d)", relativeOffset, srcPos, chunkStartOffset)
						}

						// Copy from the result buffer we're building
						// The source position is relative to the start of this chunk
						if relativeOffset < len(result) {
							// Copy from bytes we've already written in this chunk
							byteToCopy = result[relativeOffset]
						} else {
							// Trying to copy from beyond current position - this can happen with self-referential copies
							// We need to copy from bytes we just wrote in this copy operation
							// Calculate relative position within the copy we're doing
							relativePos := relativeOffset - len(result)
							if relativePos < i {
								// Copy from bytes we already wrote in this copy
								byteToCopy = result[len(result)-(i-relativePos)]
							} else {
								return nil, fmt.Errorf("copy source beyond available data: relativeOffset=%d, resultLen=%d, i=%d", relativeOffset, len(result), i)
							}
						}
					}

					result = append(result, byteToCopy)
				}
			} else {
				// Literal token (1 byte)
				// We already checked chunkOffset < compressedEnd above
				literalByte := chunkData[chunkOffset]
				chunkOffset++
				result = append(result, literalByte)
			}
		}
	}

	// Validate result size
	// Each chunk should decompress to 4096 bytes, except the last chunk which may be shorter
	if len(result) == 0 {
		return nil, fmt.Errorf("chunk decompression produced no output")
	}
	if len(result) > 4096 {
		return nil, fmt.Errorf("chunk decompression produced too much output: got %d bytes, expected at most 4096", len(result))
	}

	return result, nil
}

// unpackCopyToken unpacks a CopyToken to extract offset and length
// Based on MS-OVBA Section 2.4.1.3.2
// distance is the distance from DecompressedChunkStart to DecompressedCurrent
func unpackCopyToken(copyToken uint16, distance int) (offset int, length int, error error) {
	lengthBits, offsetBits, err := copyTokenBitCounts(distance)
	if err != nil {
		return 0, 0, err
	}

	// Extract length (lower bits)
	lengthMask := uint16((1 << lengthBits) - 1)
	length = int(copyToken&lengthMask) + 3 // Minimum length is 3

	// Extract offset (upper bits)
	offsetMask := uint16((1 << offsetBits) - 1)
	offset = int((copyToken>>lengthBits)&offsetMask) + 1 // Minimum offset is 1

	return offset, length, nil
}

func packCopyToken(offset int, length int, distance int) (uint16, error) {
	lengthBits, offsetBits, err := copyTokenBitCounts(distance)
	if err != nil {
		return 0, err
	}

	maxOffset := (1 << offsetBits)
	if offset < 1 || offset > maxOffset || offset > distance {
		return 0, fmt.Errorf("invalid copy-token offset %d (max=%d, distance=%d)", offset, maxOffset, distance)
	}

	maxLength := (1 << lengthBits) + 2
	if length < 3 || length > maxLength {
		return 0, fmt.Errorf("invalid copy-token length %d (max=%d)", length, maxLength)
	}

	lengthField := uint16(length - 3)
	offsetField := uint16(offset - 1)
	token := (offsetField << lengthBits) | lengthField
	return token, nil
}

func copyTokenBitCounts(distance int) (lengthBits int, offsetBits int, err error) {
	switch {
	case distance >= 1 && distance <= 16:
		return 12, 4, nil
	case distance >= 17 && distance <= 32:
		return 11, 5, nil
	case distance >= 33 && distance <= 64:
		return 10, 6, nil
	case distance >= 65 && distance <= 128:
		return 9, 7, nil
	case distance >= 129 && distance <= 256:
		return 8, 8, nil
	case distance >= 257 && distance <= 512:
		return 7, 9, nil
	case distance >= 513 && distance <= 1024:
		return 6, 10, nil
	case distance >= 1025 && distance <= 2048:
		return 5, 11, nil
	case distance >= 2049 && distance <= 4096:
		return 4, 12, nil
	default:
		return 0, 0, fmt.Errorf("invalid distance: %d (must be 1-4096)", distance)
	}
}
