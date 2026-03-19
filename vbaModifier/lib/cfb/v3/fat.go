package v3

import (
	"encoding/binary"
	"fmt"
)

// loadFAT initializes the File Allocation Table (FAT) from the CFB file
// The FAT contains sector chain information for navigating through the file
func (r *Reader) loadFAT() error {
	// Get FAT sector positions from DIFAT
	fatSectors, err := r.getDIFATSectors()
	if err != nil {
		return fmt.Errorf("failed to get DIFAT sectors: %w", err)
	}

	// Validate FAT sector count
	if uint32(len(fatSectors)) != r.header.NumFATSectors {
		return fmt.Errorf("%w: expected %d FAT sectors, got %d",
			ErrCorruptedFAT, r.header.NumFATSectors, len(fatSectors))
	}

	// Calculate total FAT entries
	// Each 512-byte sector contains 128 uint32 entries (512 / 4 = 128)
	totalEntries := len(fatSectors) * FAT_ENTRIES_PER_SECTOR
	r.fat = make([]uint32, 0, totalEntries)

	// Read each FAT sector and extract entries
	for _, sector := range fatSectors {
		sectorData, err := r.readSector(sector)
		if err != nil {
			return fmt.Errorf("failed to read FAT sector %d: %w", sector, err)
		}

		// Parse uint32 entries from sector data
		for i := 0; i < FAT_ENTRIES_PER_SECTOR; i++ {
			offset := i * 4
			entry := binary.LittleEndian.Uint32(sectorData[offset : offset+4])
			r.fat = append(r.fat, entry)
		}
	}

	return nil
}

// getDIFATSectors returns the list of all FAT sector positions
// Uses both the header DIFAT array and additional DIFAT sectors if needed
func (r *Reader) getDIFATSectors() ([]uint32, error) {
	fatSectors := make([]uint32, 0, r.header.NumFATSectors)

	// First, collect FAT sectors from the header's DIFAT array (first 109 entries)
	for i := 0; i < DIFAT_ARRAY_SIZE && uint32(len(fatSectors)) < r.header.NumFATSectors; i++ {
		sector := r.header.DIFAT[i]
		if sector == FREESECT {
			// Free sector means no more FAT sectors in header array
			break
		}
		if sector >= MAXREGSECT {
			return nil, fmt.Errorf("%w: invalid FAT sector %d in header DIFAT[%d]",
				ErrCorruptedDIFAT, sector, i)
		}
		fatSectors = append(fatSectors, sector)
	}

	// If we still need more FAT sectors, follow the DIFAT chain
	if uint32(len(fatSectors)) < r.header.NumFATSectors {
		if r.header.NumDIFAT == 0 {
			return nil, fmt.Errorf("%w: need %d FAT sectors but only found %d in header DIFAT",
				ErrCorruptedDIFAT, r.header.NumFATSectors, len(fatSectors))
		}

		// Load additional FAT sectors from DIFAT sectors
		additionalSectors, err := r.loadDIFATChain()
		if err != nil {
			return nil, fmt.Errorf("failed to load DIFAT chain: %w", err)
		}
		fatSectors = append(fatSectors, additionalSectors...)
	}

	// Validate we got the correct number of FAT sectors
	if uint32(len(fatSectors)) != r.header.NumFATSectors {
		return nil, fmt.Errorf("%w: expected %d FAT sectors, found %d",
			ErrCorruptedDIFAT, r.header.NumFATSectors, len(fatSectors))
	}

	return fatSectors, nil
}

// loadDIFATChain loads additional FAT sector positions from DIFAT sectors
// DIFAT sectors are used when the file is large and the header DIFAT array isn't enough
func (r *Reader) loadDIFATChain() ([]uint32, error) {
	fatSectors := make([]uint32, 0)
	currentDIFAT := r.header.FirstDIFAT

	// Each DIFAT sector contains 128 entries, but the last one points to next DIFAT sector
	// So we get 127 FAT sector positions per DIFAT sector
	const entriesPerDIFATSector = FAT_ENTRIES_PER_SECTOR - 1

	visitedDIFAT := make(map[uint32]bool)

	for i := uint32(0); i < r.header.NumDIFAT; i++ {
		// Validate DIFAT sector
		if currentDIFAT >= MAXREGSECT {
			if currentDIFAT == ENDOFCHAIN {
				break
			}
			return nil, fmt.Errorf("%w: invalid DIFAT sector %d",
				ErrCorruptedDIFAT, currentDIFAT)
		}

		// Detect circular references
		if visitedDIFAT[currentDIFAT] {
			return nil, fmt.Errorf("%w: circular reference in DIFAT at sector %d",
				ErrCircularReference, currentDIFAT)
		}
		visitedDIFAT[currentDIFAT] = true

		// Read DIFAT sector
		sectorData, err := r.readSector(currentDIFAT)
		if err != nil {
			return nil, fmt.Errorf("failed to read DIFAT sector %d: %w", currentDIFAT, err)
		}

		// Extract FAT sector positions (first 127 entries)
		for j := 0; j < entriesPerDIFATSector; j++ {
			offset := j * 4
			sector := binary.LittleEndian.Uint32(sectorData[offset : offset+4])

			if sector == FREESECT {
				// No more FAT sectors in this DIFAT sector
				break
			}

			if sector >= MAXREGSECT {
				return nil, fmt.Errorf("%w: invalid FAT sector %d in DIFAT sector %d[%d]",
					ErrCorruptedDIFAT, sector, currentDIFAT, j)
			}

			fatSectors = append(fatSectors, sector)
		}

		// Last entry points to next DIFAT sector
		nextOffset := entriesPerDIFATSector * 4
		currentDIFAT = binary.LittleEndian.Uint32(sectorData[nextOffset : nextOffset+4])
	}

	return fatSectors, nil
}

// freeChain marks a sector chain as free in the FAT
func (r *Reader) freeChain(start uint32) {
	if start >= uint32(len(r.fat)) {
		return
	}

	current := start
	for current < uint32(len(r.fat)) {
		next := r.fat[current]
		r.fat[current] = FREESECT
		if next >= MAXREGSECT {
			break
		}
		current = next
	}
}

// isValidSector checks if a sector ID is valid for data access
func (r *Reader) isValidSector(sector uint32) bool {
	return sector < MAXREGSECT && sector < uint32(len(r.fat))
}

// getNextSector returns the next sector in a chain
func (r *Reader) getNextSector(sector uint32) (uint32, error) {
	if !r.isValidSector(sector) {
		return 0, fmt.Errorf("%w: sector %d", ErrInvalidSector, sector)
	}
	return r.fat[sector], nil
}
