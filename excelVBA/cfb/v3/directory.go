package v3

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// DirectoryEntry represents a single directory entry in the CFB file
// Each entry is 128 bytes and can be a storage, stream, or root entry
type DirectoryEntry struct {
	Name         string    // Entry name (decoded from UTF-16)
	NameLength   uint16    // Length of name in bytes (including null terminator)
	Type         EntryType // Storage, Stream, or Root
	Color        ColorFlag // Red or Black (RB-tree node color)
	LeftSibling  uint32    // Left sibling directory ID
	RightSibling uint32    // Right sibling directory ID
	Child        uint32    // Child directory ID
	CLSID        [16]byte  // Class ID
	StateBits    uint32    // State bits
	CreationTime uint64    // Creation timestamp (Windows FILETIME)
	ModifiedTime uint64    // Modified timestamp (Windows FILETIME)
	StartSector  uint32    // Starting sector for the stream
	Size         uint64    // Stream size in bytes

	// Parsed relationships (populated during tree building)
	LeftEntry  *DirectoryEntry
	RightEntry *DirectoryEntry
	ChildEntry *DirectoryEntry
}

// loadDirectories reads and parses all directory entries from the file
func (r *Reader) loadDirectories() error {
	// Start from the first directory sector
	dirSector := r.header.FirstDirSector

	if dirSector >= MAXREGSECT {
		return fmt.Errorf("%w: invalid first directory sector %d", ErrCorruptedDirectory, dirSector)
	}

	// Get the chain of directory sectors
	dirChain, err := r.getSectorChain(dirSector)
	if err != nil {
		return fmt.Errorf("failed to get directory sector chain: %w", err)
	}

	// Calculate total number of directory entries
	// Each sector (512 bytes) contains 4 entries (128 bytes each)
	maxEntries := len(dirChain) * ENTRIES_PER_SECTOR
	r.dirEntries = make([]*DirectoryEntry, 0, maxEntries)

	// Read each directory sector
	for _, sector := range dirChain {
		sectorData, err := r.readSector(sector)
		if err != nil {
			return fmt.Errorf("failed to read directory sector %d: %w", sector, err)
		}

		// Parse directory entries from this sector (4 entries per sector)
		for i := 0; i < ENTRIES_PER_SECTOR; i++ {
			offset := i * DIR_ENTRY_SIZE
			entryData := sectorData[offset : offset+DIR_ENTRY_SIZE]

			entry, err := parseDirectoryEntry(entryData)
			if err != nil {
				return fmt.Errorf("failed to parse directory entry at sector %d, offset %d: %w",
					sector, i, err)
			}

			// Add all entries (including invalid ones) to maintain correct indexing
			r.dirEntries = append(r.dirEntries, entry)

			// Safety check for too many entries
			if len(r.dirEntries) > MAX_DIR_ENTRIES {
				return fmt.Errorf("%w: exceeded maximum of %d entries",
					ErrTooManyEntries, MAX_DIR_ENTRIES)
			}
		}
	}

	// Validate we have at least a root entry
	if len(r.dirEntries) == 0 {
		return ErrRootNotFound
	}

	// First entry should be the root
	if r.dirEntries[0].Type != EntryTypeRoot {
		return fmt.Errorf("%w: first entry is not root (type: %s)",
			ErrRootNotFound, r.dirEntries[0].Type)
	}
	r.rootEntry = r.dirEntries[0]

	// Build directory tree structure
	if err := r.buildDirectoryTree(); err != nil {
		return fmt.Errorf("failed to build directory tree: %w", err)
	}

	return nil
}

// parseDirectoryEntry parses a 128-byte directory entry
func parseDirectoryEntry(data []byte) (*DirectoryEntry, error) {
	if len(data) != DIR_ENTRY_SIZE {
		return nil, fmt.Errorf("invalid directory entry size: %d (expected %d)",
			len(data), DIR_ENTRY_SIZE)
	}

	entry := &DirectoryEntry{}
	buf := bytes.NewReader(data)

	// Read name as UTF-16LE (64 bytes = 32 uint16 characters)
	nameBytes := make([]byte, 64)
	if err := binary.Read(buf, binary.LittleEndian, &nameBytes); err != nil {
		return nil, fmt.Errorf("failed to read name: %w", err)
	}

	// Read name length
	if err := binary.Read(buf, binary.LittleEndian, &entry.NameLength); err != nil {
		return nil, err
	}

	// Decode UTF-16LE name
	if entry.NameLength > 0 && entry.NameLength <= 64 {
		// Name length includes null terminator, so we subtract 2 bytes
		nameLen := entry.NameLength
		if nameLen > 2 {
			nameLen -= 2
		}

		// Convert UTF-16LE bytes to uint16 slice
		utf16Chars := make([]uint16, nameLen/2)
		for i := 0; i < len(utf16Chars); i++ {
			utf16Chars[i] = binary.LittleEndian.Uint16(nameBytes[i*2 : i*2+2])
		}

		// Decode UTF-16 to string
		entry.Name = string(utf16.Decode(utf16Chars))
	}

	// Read entry type
	var entryType byte
	if err := binary.Read(buf, binary.LittleEndian, &entryType); err != nil {
		return nil, err
	}
	entry.Type = EntryType(entryType)

	// Read color flag
	var color byte
	if err := binary.Read(buf, binary.LittleEndian, &color); err != nil {
		return nil, err
	}
	entry.Color = ColorFlag(color)

	// Read sibling and child directory IDs
	if err := binary.Read(buf, binary.LittleEndian, &entry.LeftSibling); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &entry.RightSibling); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &entry.Child); err != nil {
		return nil, err
	}

	// Read CLSID
	if err := binary.Read(buf, binary.LittleEndian, &entry.CLSID); err != nil {
		return nil, err
	}

	// Read state bits
	if err := binary.Read(buf, binary.LittleEndian, &entry.StateBits); err != nil {
		return nil, err
	}

	// Read timestamps
	if err := binary.Read(buf, binary.LittleEndian, &entry.CreationTime); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &entry.ModifiedTime); err != nil {
		return nil, err
	}

	// Read starting sector
	if err := binary.Read(buf, binary.LittleEndian, &entry.StartSector); err != nil {
		return nil, err
	}

	// Read size (for version 3, this is only 32-bit, but we store as 64-bit)
	var size32 uint32
	if err := binary.Read(buf, binary.LittleEndian, &size32); err != nil {
		return nil, err
	}
	entry.Size = uint64(size32)

	// Skip reserved bytes (4 bytes at offset 124)
	buf.Seek(4, 1)

	return entry, nil
}

// buildDirectoryTree links directory entries together based on their relationships
func (r *Reader) buildDirectoryTree() error {
	// Link each entry to its siblings and children
	for _, entry := range r.dirEntries {
		// Link left sibling
		if entry.LeftSibling != NOSTREAM && entry.LeftSibling < uint32(len(r.dirEntries)) {
			entry.LeftEntry = r.dirEntries[entry.LeftSibling]
		}

		// Link right sibling
		if entry.RightSibling != NOSTREAM && entry.RightSibling < uint32(len(r.dirEntries)) {
			entry.RightEntry = r.dirEntries[entry.RightSibling]
		}

		// Link child
		if entry.Child != NOSTREAM && entry.Child < uint32(len(r.dirEntries)) {
			entry.ChildEntry = r.dirEntries[entry.Child]
		}
	}

	return nil
}

// GetEntryByID returns a directory entry by its index
func (r *Reader) GetEntryByID(id uint32) (*DirectoryEntry, error) {
	if id == NOSTREAM {
		return nil, nil
	}
	if id >= uint32(len(r.dirEntries)) {
		return nil, fmt.Errorf("%w: entry ID %d out of range (max: %d)",
			ErrInvalidSector, id, len(r.dirEntries)-1)
	}
	return r.dirEntries[id], nil
}

// Bytes returns the 128-byte directory entry as a byte slice
func (e *DirectoryEntry) Bytes() []byte {
	data := make([]byte, DIR_ENTRY_SIZE)

	// Write name as UTF-16LE
	nameUTF16 := utf16.Encode([]rune(e.Name))
	for i := 0; i < len(nameUTF16) && i < 31; i++ {
		binary.LittleEndian.PutUint16(data[i*2:], nameUTF16[i])
	}
	// NameLength includes null terminator (2 bytes)
	binary.LittleEndian.PutUint16(data[64:], e.NameLength)
	data[66] = byte(e.Type)
	data[67] = byte(e.Color)
	binary.LittleEndian.PutUint32(data[68:], e.LeftSibling)
	binary.LittleEndian.PutUint32(data[72:], e.RightSibling)
	binary.LittleEndian.PutUint32(data[76:], e.Child)
	copy(data[80:], e.CLSID[:])
	binary.LittleEndian.PutUint32(data[96:], e.StateBits)
	binary.LittleEndian.PutUint64(data[100:], e.CreationTime)
	binary.LittleEndian.PutUint64(data[108:], e.ModifiedTime)
	binary.LittleEndian.PutUint32(data[116:], e.StartSector)
	// Version 3 uses 32-bit size field
	binary.LittleEndian.PutUint32(data[120:], uint32(e.Size))
	// 4 bytes at offset 124 are reserved (already zero from make)

	return data
}

// IsStorage returns true if this entry is a storage (directory)
func (e *DirectoryEntry) IsStorage() bool {
	return e.Type == EntryTypeStorage || e.Type == EntryTypeRoot
}

// IsStream returns true if this entry is a stream (file)
func (e *DirectoryEntry) IsStream() bool {
	return e.Type == EntryTypeStream
}

// String returns a string representation of the directory entry
func (e *DirectoryEntry) String() string {
	return fmt.Sprintf("DirectoryEntry{Name: %q, Type: %s, Size: %d, StartSector: %d}",
		e.Name, e.Type, e.Size, e.StartSector)
}
