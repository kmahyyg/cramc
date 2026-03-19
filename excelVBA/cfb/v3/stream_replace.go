package v3

import "fmt"

// replaceStream replaces a stream payload without reallocating FAT/MiniFAT chains.
func (r *Reader) replaceStream(entry *DirectoryEntry, payload []byte) error {
	if payload == nil {
		payload = []byte{}
	}

	if entry == nil {
		return fmt.Errorf("nil stream entry")
	}

	// Existing empty streams cannot grow because this writer currently does not allocate new chains.
	if entry.Size == 0 && len(payload) > 0 {
		return fmt.Errorf("%w: empty stream %q cannot grow without sector allocation", ErrUnsupportedReallocation, entry.Name)
	}

	// Streams that currently live in MiniFAT must stay mini-streams.
	if entry.Size > 0 && entry.Size < uint64(r.header.MiniStreamCutoff) {
		if len(payload) >= int(r.header.MiniStreamCutoff) {
			return fmt.Errorf("%w: stream %q cannot cross mini-stream cutoff", ErrUnsupportedReallocation, entry.Name)
		}
		return r.replaceMiniStream(entry, payload)
	}

	return r.replaceRegularStream(entry, payload)
}

func (r *Reader) replaceRegularStream(entry *DirectoryEntry, payload []byte) error {
	chain, err := r.getSectorChain(entry.StartSector)
	if err != nil {
		return fmt.Errorf("failed to get sector chain for stream %q: %w", entry.Name, err)
	}

	capacity := len(chain) * SECTOR_SIZE
	if len(payload) > capacity {
		return fmt.Errorf("%w: stream %q capacity=%d payload=%d", ErrStreamCapacityExceeded, entry.Name, capacity, len(payload))
	}

	entry.Size = uint64(len(payload))
	entryID := r.findEntryID(entry)
	if entryID == NOSTREAM {
		return fmt.Errorf("stream entry ID not found for %q", entry.Name)
	}

	copied := make([]byte, len(payload))
	copy(copied, payload)
	r.modifiedStreams[entryID] = copied
	return nil
}

func (r *Reader) replaceMiniStream(entry *DirectoryEntry, payload []byte) error {
	miniChain, err := r.getMiniSectorChain(entry.StartSector)
	if err != nil {
		return fmt.Errorf("failed to get mini-sector chain for stream %q: %w", entry.Name, err)
	}

	miniSectorSize := int(r.header.GetMiniSectorSize())
	capacity := len(miniChain) * miniSectorSize
	if len(payload) > capacity {
		return fmt.Errorf("%w: stream %q mini-capacity=%d payload=%d", ErrStreamCapacityExceeded, entry.Name, capacity, len(payload))
	}

	// Zero-fill the entire allocated mini chain and copy the new payload.
	dataOffset := 0
	for _, miniSector := range miniChain {
		miniOffset := int(miniSector) * miniSectorSize
		if miniOffset+miniSectorSize > len(r.miniStream) {
			return fmt.Errorf("mini-sector %d for stream %q is out of mini-stream bounds", miniSector, entry.Name)
		}

		segment := r.miniStream[miniOffset : miniOffset+miniSectorSize]
		for i := range segment {
			segment[i] = 0
		}

		if dataOffset < len(payload) {
			n := len(payload) - dataOffset
			if n > miniSectorSize {
				n = miniSectorSize
			}
			copy(segment[:n], payload[dataOffset:dataOffset+n])
			dataOffset += n
		}
	}

	entry.Size = uint64(len(payload))

	// Mini streams live inside the root mini-stream container, which is a regular stream.
	rootID := r.findEntryID(r.rootEntry)
	if rootID == NOSTREAM {
		return fmt.Errorf("root stream entry ID not found")
	}

	miniStreamCopy := make([]byte, len(r.miniStream))
	copy(miniStreamCopy, r.miniStream)
	r.modifiedStreams[rootID] = miniStreamCopy

	return nil
}

func (r *Reader) patchModifiedStreams(fileData []byte) error {
	for entryID, payload := range r.modifiedStreams {
		if entryID >= uint32(len(r.dirEntries)) {
			return fmt.Errorf("invalid modified stream entry ID: %d", entryID)
		}
		entry := r.dirEntries[entryID]
		if entry == nil || (!entry.IsStream() && entry != r.rootEntry) {
			return fmt.Errorf("invalid modified stream entry at ID %d", entryID)
		}

		chain, err := r.getSectorChain(entry.StartSector)
		if err != nil {
			return fmt.Errorf("failed to get sector chain for modified stream %q: %w", entry.Name, err)
		}

		capacity := len(chain) * SECTOR_SIZE
		if len(payload) > capacity {
			return fmt.Errorf("%w: stream %q capacity=%d payload=%d", ErrStreamCapacityExceeded, entry.Name, capacity, len(payload))
		}

		payloadOffset := 0
		for _, sector := range chain {
			sectorOffset := int64(sector+1) * int64(HEADER_SIZE)
			if sectorOffset < 0 || sectorOffset+SECTOR_SIZE > int64(len(fileData)) {
				return fmt.Errorf("sector %d for stream %q is out of file bounds", sector, entry.Name)
			}

			start := int(sectorOffset)
			end := start + SECTOR_SIZE
			block := fileData[start:end]
			for i := range block {
				block[i] = 0
			}

			if payloadOffset < len(payload) {
				n := len(payload) - payloadOffset
				if n > SECTOR_SIZE {
					n = SECTOR_SIZE
				}
				copy(block[:n], payload[payloadOffset:payloadOffset+n])
				payloadOffset += n
			}
		}
	}
	return nil
}
