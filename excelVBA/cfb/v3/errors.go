package v3

import "errors"

// Error types for CFB file handling
var (
	// File format errors
	ErrInvalidSignature  = errors.New("invalid OLE signature")
	ErrInvalidVersion    = errors.New("only CFB version 3 is supported")
	ErrInvalidSectorSize = errors.New("invalid sector size, must be 512 bytes for version 3")
	ErrInvalidByteOrder  = errors.New("invalid byte order identifier")

	// Structure errors
	ErrCorruptedFAT       = errors.New("corrupted FAT structure")
	ErrCorruptedMiniFAT   = errors.New("corrupted MiniFAT structure")
	ErrCorruptedDIFAT     = errors.New("corrupted DIFAT structure")
	ErrCorruptedDirectory = errors.New("corrupted directory structure")

	// Sector errors
	ErrInvalidSector     = errors.New("invalid sector reference")
	ErrCircularReference = errors.New("circular sector reference detected")
	ErrChainTooLong      = errors.New("sector chain exceeds maximum length")
	ErrSectorOutOfBounds = errors.New("sector position out of file bounds")

	// Entry errors
	ErrStreamNotFound   = errors.New("stream not found")
	ErrStorageNotFound  = errors.New("storage not found")
	ErrNotAStream       = errors.New("entry is not a stream")
	ErrNotAStorage      = errors.New("entry is not a storage")
	ErrInvalidEntryType = errors.New("invalid entry type")
	ErrInvalidEntryName = errors.New("invalid entry name encoding")

	// Directory tree errors
	ErrRootNotFound         = errors.New("root entry not found")
	ErrTooManyEntries       = errors.New("too many directory entries")
	ErrInvalidTreeStructure = errors.New("invalid directory tree structure")

	// Stream errors
	ErrStreamClosed            = errors.New("stream is closed")
	ErrInvalidSeekOffset       = errors.New("invalid seek offset")
	ErrStreamTooLarge          = errors.New("stream size exceeds reasonable limit")
	ErrStreamCapacityExceeded  = errors.New("replacement stream exceeds allocated capacity")
	ErrUnsupportedReallocation = errors.New("stream reallocation is not supported")
)
