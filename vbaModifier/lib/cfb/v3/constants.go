package v3

// CFB Version 3 Constants
const (
	// Sector sizes for CFB Version 3
	HEADER_SIZE        = 512
	SECTOR_SIZE        = 512
	MINI_SECTOR_SIZE   = 64
	MINI_STREAM_CUTOFF = 4096

	// Directory entry constants
	DIR_ENTRY_SIZE         = 128
	ENTRIES_PER_SECTOR     = 4   // 512 / 128
	FAT_ENTRIES_PER_SECTOR = 128 // 512 / 4

	// DIFAT array size in header
	DIFAT_ARRAY_SIZE = 109

	// Sanity limits
	MAX_DIR_ENTRIES = 100000   // Maximum directory entries to prevent infinite loops
	MAX_SECTORS     = 0x100000 // Maximum sectors in a chain

	// Special sector values
	MAXREGSECT = 0xFFFFFFFA // Maximum regular sector ID
	DIFSECT    = 0xFFFFFFFC // DIFAT sector
	FATSECT    = 0xFFFFFFFD // FAT sector
	ENDOFCHAIN = 0xFFFFFFFE // End of sector chain
	FREESECT   = 0xFFFFFFFF // Free sector

	// Directory entry special values
	NOSTREAM = 0xFFFFFFFF // No stream ID
)

// OLE Signature - First 8 bytes of a valid OLE file
var OLE_SIGNATURE = [8]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// EntryType represents the type of a directory entry
type EntryType byte

const (
	EntryTypeInvalid EntryType = 0 // Unknown or unallocated
	EntryTypeStorage EntryType = 1 // Storage object (directory)
	EntryTypeStream  EntryType = 2 // Stream object (file)
	EntryTypeRoot    EntryType = 5 // Root storage object
)

// String returns a string representation of the entry type
func (e EntryType) String() string {
	switch e {
	case EntryTypeInvalid:
		return "Unknown"
	case EntryTypeStorage:
		return "Storage"
	case EntryTypeStream:
		return "Stream"
	case EntryTypeRoot:
		return "Root"
	default:
		return "Invalid"
	}
}

// ColorFlag represents the color of a directory entry in the red-black tree
type ColorFlag byte

const (
	ColorRed   ColorFlag = 0 // Red node
	ColorBlack ColorFlag = 1 // Black node
)

// String returns a string representation of the color flag
func (c ColorFlag) String() string {
	switch c {
	case ColorRed:
		return "Red"
	case ColorBlack:
		return "Black"
	default:
		return "Invalid"
	}
}
