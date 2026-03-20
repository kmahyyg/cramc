package v3

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Reader represents a CFB (Compound File Binary) file reader for Version 3
type Reader struct {
	file            *os.File
	data            []byte
	header          *Header
	fat             []uint32          // File Allocation Table
	miniFat         []uint32          // Mini FAT for small streams
	miniStream      []byte            // Mini stream container data
	dirEntries      []*DirectoryEntry // All directory entries
	rootEntry       *DirectoryEntry   // Root storage entry
	modifiedStreams map[uint32][]byte // Modified stream payloads keyed by directory entry ID
}

// Open opens and parses a CFB Version 3 file
// It validates the header and initializes the reader structure
func Open(filename string) (*Reader, error) {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Try to read and validate header
	header, err := readHeader(file)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("invalid CFB file: %w", err)
	}

	// Create reader instance
	reader := &Reader{
		file:            file,
		header:          header,
		modifiedStreams: make(map[uint32][]byte),
	}

	// Initialize FAT (File Allocation Table)
	if err := reader.initFAT(); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to initialize FAT: %w", err)
	}

	// Parse directory entries
	if err := reader.initDirectories(); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to initialize directories: %w", err)
	}

	// Initialize MiniFAT and mini stream
	if err := reader.initMiniFAT(); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to initialize MiniFAT: %w", err)
	}

	return reader, nil
}

// OpenBytes opens and parses a CFB Version 3 file from raw bytes.
func OpenBytes(data []byte) (*Reader, error) {
	if len(data) < HEADER_SIZE {
		return nil, fmt.Errorf("invalid CFB file: incomplete header read: got %d bytes, expected %d", len(data), HEADER_SIZE)
	}

	backing := make([]byte, len(data))
	copy(backing, data)

	readerAt := bytes.NewReader(backing)
	header, err := readHeader(readerAt)
	if err != nil {
		return nil, fmt.Errorf("invalid CFB file: %w", err)
	}

	reader := &Reader{
		data:            backing,
		header:          header,
		modifiedStreams: make(map[uint32][]byte),
	}

	if err := reader.initFAT(); err != nil {
		return nil, fmt.Errorf("failed to initialize FAT: %w", err)
	}

	if err := reader.initDirectories(); err != nil {
		return nil, fmt.Errorf("failed to initialize directories: %w", err)
	}

	if err := reader.initMiniFAT(); err != nil {
		return nil, fmt.Errorf("failed to initialize MiniFAT: %w", err)
	}

	return reader, nil
}

// Close closes the underlying file
func (r *Reader) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// GetHeader returns the CFB header
func (r *Reader) GetHeader() *Header {
	return r.header
}

// findEntryID finds the index of a directory entry in the reader's entries
func (r *Reader) findEntryID(entry *DirectoryEntry) uint32 {
	for i, e := range r.dirEntries {
		if e == entry {
			return uint32(i)
		}
	}
	return NOSTREAM
}

// WriteTo implements io.WriterTo interface, writing the modified CFB content to w
func (r *Reader) WriteTo(w io.Writer) (int64, error) {
	// 1. Get original file data
	var data []byte
	if r.file != nil {
		info, err := r.file.Stat()
		if err != nil {
			return 0, fmt.Errorf("failed to stat original file: %w", err)
		}
		size := info.Size()

		// Read entire file into a buffer for patching
		// This is safe for CFB Version 3 which is limited in size (max 2GB, usually much smaller for VBA)
		data = make([]byte, size)
		if _, err := r.file.Seek(0, 0); err != nil {
			return 0, fmt.Errorf("failed to seek original file: %w", err)
		}
		if _, err := io.ReadFull(r.file, data); err != nil {
			return 0, fmt.Errorf("failed to read original file: %w", err)
		}
	} else {
		data = make([]byte, len(r.data))
		copy(data, r.data)
	}

	// 2. Patch modified Header at offset 0
	copy(data[0:], r.header.Bytes())

	// 3. Patch modified FAT sectors
	fatSectors, err := r.getDIFATSectors()
	if err != nil {
		return 0, fmt.Errorf("failed to get FAT sectors for saving: %w", err)
	}
	for i, sector := range fatSectors {
		offset := int64(sector+1) * int64(HEADER_SIZE)
		fatData := make([]byte, HEADER_SIZE)
		// Initialize with 0xFF (FREESECT) to preserve standard padding
		for k := range fatData {
			fatData[k] = 0xFF
		}
		for j := 0; j < FAT_ENTRIES_PER_SECTOR; j++ {
			entryIdx := i*FAT_ENTRIES_PER_SECTOR + j
			if entryIdx < len(r.fat) {
				binary.LittleEndian.PutUint32(fatData[j*4:], r.fat[entryIdx])
			}
		}
		copy(data[offset:], fatData)
	}

	// 4. Patch modified MiniFAT sectors
	if r.header.NumMiniFAT > 0 {
		miniFATChain, err := r.getSectorChain(r.header.FirstMiniFAT)
		if err != nil {
			return 0, fmt.Errorf("failed to get MiniFAT chain for saving: %w", err)
		}
		for i, sector := range miniFATChain {
			offset := int64(sector+1) * int64(HEADER_SIZE)
			miniFATData := make([]byte, HEADER_SIZE)
			// Initialize with 0xFF (FREESECT) to preserve standard padding
			for k := range miniFATData {
				miniFATData[k] = 0xFF
			}
			for j := 0; j < FAT_ENTRIES_PER_SECTOR; j++ {
				entryIdx := i*FAT_ENTRIES_PER_SECTOR + j
				if entryIdx < len(r.miniFat) {
					binary.LittleEndian.PutUint32(miniFATData[j*4:], r.miniFat[entryIdx])
				}
			}
			copy(data[offset:], miniFATData)
		}
	}

	// 5. Patch modified Directory entries
	dirSector := r.header.FirstDirSector
	dirChain, err := r.getSectorChain(dirSector)
	if err != nil {
		return 0, fmt.Errorf("failed to get directory chain for saving: %w", err)
	}
	for i, sector := range dirChain {
		offset := int64(sector+1) * int64(HEADER_SIZE)
		dirData := make([]byte, HEADER_SIZE)
		for j := 0; j < ENTRIES_PER_SECTOR; j++ {
			entryIdx := i*ENTRIES_PER_SECTOR + j
			if entryIdx < len(r.dirEntries) {
				copy(dirData[j*DIR_ENTRY_SIZE:], r.dirEntries[entryIdx].Bytes())
			}
		}
		copy(data[offset:], dirData)
	}

	// 6. Patch modified stream payloads
	if err := r.patchModifiedStreams(data); err != nil {
		return 0, err
	}

	// 7. Write final data to destination
	n, err := w.Write(data)
	return int64(n), err
}

// OpenRootStorage opens the root storage for navigation
func (r *Reader) OpenRootStorage() (*Storage, error) {
	if r.rootEntry == nil {
		return nil, ErrRootNotFound
	}

	return &Storage{
		reader: r,
		entry:  r.rootEntry,
	}, nil
}

// initFAT initializes the File Allocation Table
// This method will be implemented when fat.go is created
func (r *Reader) initFAT() error {
	// Placeholder - will be implemented in fat.go
	return r.loadFAT()
}

// initDirectories parses directory entries and builds the tree structure
// This method will be implemented when directory.go is created
func (r *Reader) initDirectories() error {
	// Placeholder - will be implemented in directory.go
	return r.loadDirectories()
}

// initMiniFAT initializes the Mini FAT and loads the mini stream container
// This method will be implemented when minifat.go is created
func (r *Reader) initMiniFAT() error {
	// Placeholder - will be implemented in minifat.go
	return r.loadMiniFAT()
}
