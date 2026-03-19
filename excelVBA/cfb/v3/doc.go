// Package v3 implements a reader for OLE Compound File Binary Format (CFB) Version 3.
//
// # Overview
//
// The OLE Compound File format (documented in MS-CFB) is a file-system-within-a-file
// structure used by Microsoft Office applications to store structured data. This package
// specifically supports CFB Version 3, which uses 512-byte sectors.
//
// # Key Concepts
//
// - Storage: A directory-like container that can hold streams and other storages
// - Stream: A file-like object that contains actual data
// - Sector: Fixed-size (512-byte) blocks of data
// - FAT: File Allocation Table that tracks sector chains
// - MiniFAT: Special FAT for small streams (< 4096 bytes)
//
// # Basic Usage
//
// Opening a CFB file and reading a stream:
//
//	import cfbv3 "vbaModifier/lib/cfb/v3"
//
//	reader, err := cfbv3.Open("document.xls")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer reader.Close()
//
//	root, err := reader.OpenRootStorage()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	stream, err := root.OpenStream("Workbook")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer stream.Close()
//
//	data, err := io.ReadAll(stream)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Navigating the Directory Tree
//
// CFB files have a hierarchical structure similar to a file system:
//
//	root, _ := reader.OpenRootStorage()
//
//	// List all entries in root
//	entries := root.List()
//	fmt.Println("Root entries:", entries)
//
//	// Open a sub-storage
//	vbaStorage, err := root.OpenStorage("VBA")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// List streams in the VBA storage
//	streams := vbaStorage.ListStreams()
//	fmt.Println("VBA streams:", streams)
//
//	// Open a stream in the VBA storage
//	dirStream, err := vbaStorage.OpenStream("dir")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer dirStream.Close()
//
// # Stream Operations
//
// Streams implement io.Reader, io.Seeker, and io.Closer interfaces:
//
//	// Read entire stream
//	data, _ := io.ReadAll(stream)
//
//	// Seek to specific position
//	stream.Seek(100, io.SeekStart)
//
//	// Read in chunks
//	buffer := make([]byte, 512)
//	n, err := stream.Read(buffer)
//
//	// Get stream information
//	size := stream.Size()
//	name := stream.Name()
//
// # Version 3 Specifics
//
// This implementation only supports CFB Version 3, which has the following characteristics:
//
//   - Fixed 512-byte sectors
//   - Fixed 64-byte mini sectors
//   - 4096-byte mini stream cutoff (streams smaller than 4096 bytes use MiniFAT)
//   - Maximum sector size is 512 bytes (sector shift = 9)
//
// Files using CFB Version 4 (4096-byte sectors) are not supported and will be rejected
// during the Open() call with an ErrInvalidVersion error.
//
// # Error Handling
//
// The package defines several error types for different failure scenarios:
//
//   - ErrInvalidSignature: File is not a valid OLE file
//   - ErrInvalidVersion: File uses CFB version other than 3
//   - ErrInvalidSectorSize: Sector size is not 512 bytes
//   - ErrStreamNotFound: Requested stream does not exist
//   - ErrStorageNotFound: Requested storage does not exist
//   - ErrNotAStream: Entry is a storage, not a stream
//   - ErrNotAStorage: Entry is a stream, not a storage
//   - ErrCircularReference: Corrupted sector chain with circular reference
//   - ErrChainTooLong: Sector chain exceeds reasonable length
//
// # Thread Safety
//
// Reader instances are not thread-safe. Each Reader should be used by a single goroutine.
// If concurrent access is needed, create separate Reader instances for each goroutine.
//
// # Memory Considerations
//
// Streams are loaded entirely into memory when opened. For very large streams, this may
// consume significant memory. Consider the stream size before opening.
//
// # References
//
//   - MS-CFB: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-cfb/
//   - MS-OVBA: https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-ovba/
package v3
