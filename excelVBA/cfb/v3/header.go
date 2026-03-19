package v3

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Header represents the 512-byte header of a CFB Version 3 file
type Header struct {
	Signature        [8]byte     // Must be: 0xD0CF11E0A1B11AE1
	ReservedCLSID    [16]byte    // Class ID (typically zeros)
	MinorVersion     uint16      // Minor version (0x003E)
	MajorVersion     uint16      // Must be 3 for version 3
	ByteOrder        uint16      // Byte order identifier (0xFFFE for little-endian)
	SectorSize       uint16      // Must be 9 (2^9 = 512 bytes)
	MiniSectorSize   uint16      // Typically 6 (2^6 = 64 bytes)
	Reserved1        [6]byte     // Reserved, must be zero
	NumDirSectors    uint32      // Reserved, must be zero (v3)
	NumFATSectors    uint32      // Number of FAT sectors
	FirstDirSector   uint32      // First directory sector
	TransactionSig   uint32      // Transaction signature number
	MiniStreamCutoff uint32      // Mini stream cutoff (typically 4096)
	FirstMiniFAT     uint32      // First MiniFAT sector
	NumMiniFAT       uint32      // Number of MiniFAT sectors
	FirstDIFAT       uint32      // First DIFAT sector
	NumDIFAT         uint32      // Number of DIFAT sectors
	DIFAT            [109]uint32 // First 109 FAT sector positions
}

// readHeader reads and validates the CFB header from the given reader
func readHeader(r io.ReaderAt) (*Header, error) {
	// Read the 512-byte header
	headerBytes := make([]byte, HEADER_SIZE)
	n, err := r.ReadAt(headerBytes, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}
	if n != HEADER_SIZE {
		return nil, fmt.Errorf("incomplete header read: got %d bytes, expected %d", n, HEADER_SIZE)
	}

	// Parse header structure
	header := &Header{}
	buf := bytes.NewReader(headerBytes)

	// Read header fields using binary.Read for proper byte order handling
	if err := binary.Read(buf, binary.LittleEndian, &header.Signature); err != nil {
		return nil, fmt.Errorf("failed to read signature: %w", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.ReservedCLSID); err != nil {
		return nil, fmt.Errorf("failed to read CLSID: %w", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.MinorVersion); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.MajorVersion); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.ByteOrder); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.SectorSize); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.MiniSectorSize); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.Reserved1); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.NumDirSectors); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.NumFATSectors); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.FirstDirSector); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.TransactionSig); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.MiniStreamCutoff); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.FirstMiniFAT); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.NumMiniFAT); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.FirstDIFAT); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.NumDIFAT); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.DIFAT); err != nil {
		return nil, err
	}

	// Validate header
	if err := header.validate(); err != nil {
		return nil, err
	}

	return header, nil
}

// validate checks if the header is valid for CFB Version 3
func (h *Header) validate() error {
	// Validate signature
	if !bytes.Equal(h.Signature[:], OLE_SIGNATURE[:]) {
		return ErrInvalidSignature
	}

	// Validate byte order (little-endian)
	if h.ByteOrder != 0xFFFE {
		return ErrInvalidByteOrder
	}

	// Validate major version (must be 3)
	if h.MajorVersion != 3 {
		return fmt.Errorf("%w: got version %d", ErrInvalidVersion, h.MajorVersion)
	}

	// Validate sector size (must be 9 for 512-byte sectors)
	if h.SectorSize != 9 {
		return fmt.Errorf("%w: got sector shift %d (expected 9 for 512 bytes)", ErrInvalidSectorSize, h.SectorSize)
	}

	// Validate mini sector size (typically 6 for 64-byte sectors)
	if h.MiniSectorSize < 6 || h.MiniSectorSize > 9 {
		return fmt.Errorf("invalid mini sector size: %d (expected 6-9)", h.MiniSectorSize)
	}

	// Version 3 specific: NumDirSectors must be zero
	if h.NumDirSectors != 0 {
		return fmt.Errorf("version 3 requires NumDirSectors to be 0, got %d", h.NumDirSectors)
	}

	return nil
}

// GetSectorSize returns the actual sector size in bytes (2^SectorSize)
func (h *Header) GetSectorSize() int {
	return 1 << h.SectorSize
}

// GetMiniSectorSize returns the actual mini sector size in bytes (2^MiniSectorSize)
func (h *Header) GetMiniSectorSize() int {
	return 1 << h.MiniSectorSize
}

// Bytes returns the 512-byte header as a byte slice
func (h *Header) Bytes() []byte {
	buf := new(bytes.Buffer)
	// Write header fields in little-endian order
	binary.Write(buf, binary.LittleEndian, h.Signature)
	binary.Write(buf, binary.LittleEndian, h.ReservedCLSID)
	binary.Write(buf, binary.LittleEndian, h.MinorVersion)
	binary.Write(buf, binary.LittleEndian, h.MajorVersion)
	binary.Write(buf, binary.LittleEndian, h.ByteOrder)
	binary.Write(buf, binary.LittleEndian, h.SectorSize)
	binary.Write(buf, binary.LittleEndian, h.MiniSectorSize)
	binary.Write(buf, binary.LittleEndian, h.Reserved1)
	binary.Write(buf, binary.LittleEndian, h.NumDirSectors)
	binary.Write(buf, binary.LittleEndian, h.NumFATSectors)
	binary.Write(buf, binary.LittleEndian, h.FirstDirSector)
	binary.Write(buf, binary.LittleEndian, h.TransactionSig)
	binary.Write(buf, binary.LittleEndian, h.MiniStreamCutoff)
	binary.Write(buf, binary.LittleEndian, h.FirstMiniFAT)
	binary.Write(buf, binary.LittleEndian, h.NumMiniFAT)
	binary.Write(buf, binary.LittleEndian, h.FirstDIFAT)
	binary.Write(buf, binary.LittleEndian, h.NumDIFAT)
	binary.Write(buf, binary.LittleEndian, h.DIFAT)

	return buf.Bytes()
}

// String returns a human-readable representation of the header
func (h *Header) String() string {
	return fmt.Sprintf("CFB Header: Version %d.%d, Sector Size: %d bytes, Mini Sector Size: %d bytes, FAT Sectors: %d, MiniFAT Sectors: %d",
		h.MajorVersion, h.MinorVersion, h.GetSectorSize(), h.GetMiniSectorSize(), h.NumFATSectors, h.NumMiniFAT)
}
