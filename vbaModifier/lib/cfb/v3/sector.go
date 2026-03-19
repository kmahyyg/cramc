package v3

import (
	"fmt"
)

// readSector reads a single sector from the file
// sector is the sector index (0-based)
// Sector offset calculation: (sector + 1) * SECTOR_SIZE
// +1 because sector 0 starts after the 512-byte header
func (r *Reader) readSector(sector uint32) ([]byte, error) {
	if sector >= MAXREGSECT {
		return nil, fmt.Errorf("%w: sector %d", ErrInvalidSector, sector)
	}

	// Calculate file offset: skip header (512 bytes) + sector offset
	offset := int64(sector+1) * SECTOR_SIZE

	// Read sector data
	data := make([]byte, SECTOR_SIZE)
	n, err := r.file.ReadAt(data, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to read sector %d at offset %d: %w", sector, offset, err)
	}
	if n != SECTOR_SIZE {
		return nil, fmt.Errorf("incomplete sector read: got %d bytes, expected %d", n, SECTOR_SIZE)
	}

	return data, nil
}

// getSectorChain follows a chain of sectors starting from startSector
// Returns the complete chain as a slice of sector indices
// Detects and prevents circular references and excessively long chains
func (r *Reader) getSectorChain(startSector uint32) ([]uint32, error) {
	// Special case: empty chain
	if startSector >= MAXREGSECT {
		if startSector == ENDOFCHAIN {
			return []uint32{}, nil
		}
		return nil, fmt.Errorf("%w: start sector %d", ErrInvalidSector, startSector)
	}

	chain := make([]uint32, 0, 64) // Pre-allocate for typical chain length
	visited := make(map[uint32]bool)
	current := startSector

	for current != ENDOFCHAIN && current < MAXREGSECT {
		// Validate sector index
		if current >= uint32(len(r.fat)) {
			return nil, fmt.Errorf("%w: sector %d out of FAT bounds (FAT size: %d)", 
				ErrInvalidSector, current, len(r.fat))
		}

		// Detect circular references
		if visited[current] {
			return nil, fmt.Errorf("%w: sector %d appears twice in chain", 
				ErrCircularReference, current)
		}
		visited[current] = true

		// Add to chain
		chain = append(chain, current)

		// Prevent excessive chains
		if len(chain) > MAX_SECTORS {
			return nil, fmt.Errorf("%w: chain length exceeds %d sectors", 
				ErrChainTooLong, MAX_SECTORS)
		}

		// Get next sector in chain
		current = r.fat[current]
	}

	// Validate chain termination
	if current != ENDOFCHAIN && current >= MAXREGSECT {
		return nil, fmt.Errorf("%w: chain terminated with invalid sector %d", 
			ErrInvalidSector, current)
	}

	return chain, nil
}

// readChain reads the complete data from a sector chain
func (r *Reader) readChain(startSector uint32, size uint64) ([]byte, error) {
	// Get the sector chain
	chain, err := r.getSectorChain(startSector)
	if err != nil {
		return nil, fmt.Errorf("failed to get sector chain: %w", err)
	}

	// Validate size
	if size > uint64(len(chain))*SECTOR_SIZE {
		return nil, fmt.Errorf("size %d exceeds available sectors (%d sectors, %d bytes)", 
			size, len(chain), len(chain)*SECTOR_SIZE)
	}

	// Allocate buffer for the data
	data := make([]byte, size)
	offset := uint64(0)

	// Read each sector in the chain
	for _, sector := range chain {
		sectorData, err := r.readSector(sector)
		if err != nil {
			return nil, fmt.Errorf("failed to read sector %d: %w", sector, err)
		}

		// Calculate how much to copy from this sector
		remaining := size - offset
		toCopy := uint64(SECTOR_SIZE)
		if toCopy > remaining {
			toCopy = remaining
		}

		// Copy data
		copy(data[offset:offset+toCopy], sectorData[:toCopy])
		offset += toCopy

		// Stop if we've read all requested data
		if offset >= size {
			break
		}
	}

	return data, nil
}

// getMiniSectorChain follows a chain of mini sectors in the MiniFAT
// Similar to getSectorChain but uses miniFat instead of fat
func (r *Reader) getMiniSectorChain(startSector uint32) ([]uint32, error) {
	// Special case: empty chain
	if startSector >= MAXREGSECT {
		if startSector == ENDOFCHAIN {
			return []uint32{}, nil
		}
		return nil, fmt.Errorf("%w: start mini sector %d", ErrInvalidSector, startSector)
	}

	chain := make([]uint32, 0, 64)
	visited := make(map[uint32]bool)
	current := startSector

	for current != ENDOFCHAIN && current < MAXREGSECT {
		// Validate mini sector index
		if current >= uint32(len(r.miniFat)) {
			return nil, fmt.Errorf("%w: mini sector %d out of MiniFAT bounds (MiniFAT size: %d)", 
				ErrInvalidSector, current, len(r.miniFat))
		}

		// Detect circular references
		if visited[current] {
			return nil, fmt.Errorf("%w: mini sector %d appears twice in chain", 
				ErrCircularReference, current)
		}
		visited[current] = true

		// Add to chain
		chain = append(chain, current)

		// Prevent excessive chains
		if len(chain) > MAX_SECTORS {
			return nil, fmt.Errorf("%w: mini chain length exceeds %d sectors", 
				ErrChainTooLong, MAX_SECTORS)
		}

		// Get next mini sector in chain
		current = r.miniFat[current]
	}

	// Validate chain termination
	if current != ENDOFCHAIN && current >= MAXREGSECT {
		return nil, fmt.Errorf("%w: mini chain terminated with invalid sector %d", 
			ErrInvalidSector, current)
	}

	return chain, nil
}

// readMiniChain reads data from a mini sector chain
func (r *Reader) readMiniChain(startSector uint32, size uint64) ([]byte, error) {
	// Get the mini sector chain
	chain, err := r.getMiniSectorChain(startSector)
	if err != nil {
		return nil, fmt.Errorf("failed to get mini sector chain: %w", err)
	}

	miniSectorSize := r.header.GetMiniSectorSize()

	// Validate size
	if size > uint64(len(chain))*uint64(miniSectorSize) {
		return nil, fmt.Errorf("size %d exceeds available mini sectors (%d sectors, %d bytes)", 
			size, len(chain), len(chain)*miniSectorSize)
	}

	// Allocate buffer for the data
	data := make([]byte, size)
	offset := uint64(0)

	// Read each mini sector from the mini stream
	for _, miniSector := range chain {
		// Calculate position in mini stream
		miniOffset := uint64(miniSector) * uint64(miniSectorSize)

		// Validate mini stream bounds
		if miniOffset+uint64(miniSectorSize) > uint64(len(r.miniStream)) {
			return nil, fmt.Errorf("mini sector %d at offset %d exceeds mini stream size %d", 
				miniSector, miniOffset, len(r.miniStream))
		}

		// Calculate how much to copy from this mini sector
		remaining := size - offset
		toCopy := uint64(miniSectorSize)
		if toCopy > remaining {
			toCopy = remaining
		}

		// Copy data from mini stream
		copy(data[offset:offset+toCopy], r.miniStream[miniOffset:miniOffset+toCopy])
		offset += toCopy

		// Stop if we've read all requested data
		if offset >= size {
			break
		}
	}

	return data, nil
}
