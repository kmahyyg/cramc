package v3

import (
	"encoding/binary"
	"fmt"
)

// loadMiniFAT initializes the Mini FAT and loads the mini stream container
// The MiniFAT is used for small streams (< 4096 bytes by default)
// These streams are stored within the root entry's stream (the mini stream container)
func (r *Reader) loadMiniFAT() error {
	// If there are no MiniFAT sectors, nothing to load
	if r.header.NumMiniFAT == 0 {
		r.miniFat = []uint32{}
		r.miniStream = []byte{}
		return nil
	}

	// Load MiniFAT sectors
	if err := r.loadMiniFATSectors(); err != nil {
		return fmt.Errorf("failed to load MiniFAT sectors: %w", err)
	}

	// Load mini stream container from root entry
	if err := r.loadMiniStreamContainer(); err != nil {
		return fmt.Errorf("failed to load mini stream container: %w", err)
	}

	return nil
}

// loadMiniFATSectors reads and parses the MiniFAT sectors
func (r *Reader) loadMiniFATSectors() error {
	// Get the MiniFAT sector chain
	miniFATChain, err := r.getSectorChain(r.header.FirstMiniFAT)
	if err != nil {
		return fmt.Errorf("failed to get MiniFAT chain: %w", err)
	}

	// Validate MiniFAT sector count
	if uint32(len(miniFATChain)) != r.header.NumMiniFAT {
		return fmt.Errorf("%w: expected %d MiniFAT sectors, got %d",
			ErrCorruptedMiniFAT, r.header.NumMiniFAT, len(miniFATChain))
	}

	// Calculate total MiniFAT entries
	// Each 512-byte sector contains 128 uint32 entries (512 / 4 = 128)
	totalEntries := len(miniFATChain) * FAT_ENTRIES_PER_SECTOR
	r.miniFat = make([]uint32, 0, totalEntries)

	// Read each MiniFAT sector and extract entries
	for _, sector := range miniFATChain {
		sectorData, err := r.readSector(sector)
		if err != nil {
			return fmt.Errorf("failed to read MiniFAT sector %d: %w", sector, err)
		}

		// Parse uint32 entries from sector data
		for i := 0; i < FAT_ENTRIES_PER_SECTOR; i++ {
			offset := i * 4
			entry := binary.LittleEndian.Uint32(sectorData[offset : offset+4])
			r.miniFat = append(r.miniFat, entry)
		}
	}

	return nil
}

// loadMiniStreamContainer loads the mini stream container from the root entry
// The mini stream container holds all the small streams
func (r *Reader) loadMiniStreamContainer() error {
	// The root entry contains the mini stream container
	if r.rootEntry == nil {
		return fmt.Errorf("%w: cannot load mini stream without root entry", ErrRootNotFound)
	}

	// If root entry has no stream, there's no mini stream container
	if r.rootEntry.Size == 0 {
		r.miniStream = []byte{}
		return nil
	}

	// Read the mini stream container from the root entry's stream
	// This uses the regular FAT, not the MiniFAT
	miniStreamData, err := r.readChain(r.rootEntry.StartSector, r.rootEntry.Size)
	if err != nil {
		return fmt.Errorf("failed to read mini stream container: %w", err)
	}

	r.miniStream = miniStreamData
	return nil
}

// freeMiniChain marks a mini sector chain as free in the MiniFAT
func (r *Reader) freeMiniChain(start uint32) {
	if start >= uint32(len(r.miniFat)) {
		return
	}

	current := start
	for current < uint32(len(r.miniFat)) {
		next := r.miniFat[current]
		r.miniFat[current] = FREESECT
		if next >= MAXREGSECT {
			break
		}
		current = next
	}
}

// isValidMiniSector checks if a mini sector ID is valid for data access
func (r *Reader) isValidMiniSector(sector uint32) bool {
	return sector < MAXREGSECT && sector < uint32(len(r.miniFat))
}

// getNextMiniSector returns the next mini sector in a chain
func (r *Reader) getNextMiniSector(sector uint32) (uint32, error) {
	if !r.isValidMiniSector(sector) {
		return 0, fmt.Errorf("%w: mini sector %d", ErrInvalidSector, sector)
	}
	return r.miniFat[sector], nil
}

// readMiniStreamData reads data from a specific position in the mini stream
// This is a helper function for reading mini sectors
func (r *Reader) readMiniStreamData(offset, length uint64) ([]byte, error) {
	if offset+length > uint64(len(r.miniStream)) {
		return nil, fmt.Errorf("mini stream read out of bounds: offset=%d, length=%d, stream_size=%d",
			offset, length, len(r.miniStream))
	}

	data := make([]byte, length)
	copy(data, r.miniStream[offset:offset+length])
	return data, nil
}
