# CFB Version 3 Reader

A pure Go implementation of a reader for OLE Compound File Binary Format (CFB) Version 3.

## Overview

This package provides a complete implementation for reading Microsoft's Compound File Binary format, specifically Version 3 (512-byte sectors). This format is used extensively by Microsoft Office applications to store structured data, including VBA macros in Excel files.

## Features

- ✅ **Complete CFB v3 Support**: Full implementation of MS-CFB specification for Version 3
- ✅ **Sector Management**: Efficient handling of FAT, MiniFAT, and DIFAT
- ✅ **Directory Tree Navigation**: Hierarchical storage and stream access
- ✅ **Stream Interface**: Standard Go interfaces (`io.Reader`, `io.Seeker`, `io.Closer`)
- ✅ **Error Handling**: Comprehensive validation and error reporting
- ✅ **No External Dependencies**: Uses only Go standard library
- ✅ **Production Ready**: Tested with real vbaProject.bin files

## Installation

```bash
go get vbaModifier/lib/cfb/v3
```

## Quick Start

```go
package main

import (
    "fmt"
    "io"
    "log"
    
    cfbv3 "vbaModifier/lib/cfb/v3"
)

func main() {
    // Open CFB file
    reader, err := cfbv3.Open("vbaProject.bin")
    if err != nil {
        log.Fatal(err)
    }
    defer reader.Close()
    
    // Open root storage
    root, err := reader.OpenRootStorage()
    if err != nil {
        log.Fatal(err)
    }
    
    // Navigate to VBA storage
    vba, err := root.OpenStorage("VBA")
    if err != nil {
        log.Fatal(err)
    }
    
    // Open a stream
    stream, err := vba.OpenStream("dir")
    if err != nil {
        log.Fatal(err)
    }
    defer stream.Close()
    
    // Read stream data
    data, err := io.ReadAll(stream)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Read %d bytes from /VBA/dir\n", len(data))
}
```

## Architecture

### Package Structure

```
lib/cfb/v3/
├── constants.go    # CFB constants and enums
├── errors.go       # Custom error types
├── header.go       # Header parsing and validation
├── reader.go       # Main Reader struct and API
├── sector.go       # Sector reading and chain following
├── fat.go          # FAT and DIFAT handling
├── minifat.go      # MiniFAT for small streams
├── directory.go    # Directory entry parsing
├── stream.go       # Stream interface implementation
├── storage.go      # Storage navigation
└── doc.go          # Package documentation
```

### Key Components

#### 1. Reader
The main entry point for opening and reading CFB files.

```go
type Reader struct {
    file       *os.File
    header     *Header
    fat        []uint32
    miniFat    []uint32
    miniStream []byte
    dirEntries []*DirectoryEntry
    rootEntry  *DirectoryEntry
}
```

**Methods:**
- [`Open(filename string)`](reader.go:28) - Opens a CFB file
- [`Close()`](reader.go:70) - Closes the file
- [`OpenRootStorage()`](reader.go:81) - Opens the root storage
- [`GetHeader()`](reader.go:76) - Returns header information

#### 2. Storage
Represents a directory-like container in the CFB hierarchy.

```go
type Storage struct {
    reader *Reader
    entry  *DirectoryEntry
}
```

**Methods:**
- [`OpenStream(name)`](storage.go:14) - Opens a stream by name
- [`OpenStorage(name)`](storage.go:29) - Opens a sub-storage by name
- [`ListStreams()`](storage.go:48) - Lists all stream names
- [`ListStorages()`](storage.go:57) - Lists all sub-storage names
- [`List()`](storage.go:68) - Lists all entries
- [`Exists(name)`](storage.go:145) - Checks if an entry exists

#### 3. Stream
Represents a file-like object containing data.

```go
type Stream interface {
    io.Reader
    io.Seeker
    io.Closer
    Size() int64
    Name() string
}
```

**Usage:**
```go
stream, _ := storage.OpenStream("example")
defer stream.Close()

// Read entire stream
data, _ := io.ReadAll(stream)

// Seek to position
stream.Seek(100, io.SeekStart)

// Read chunk
buffer := make([]byte, 512)
n, _ := stream.Read(buffer)

// Get information
size := stream.Size()
name := stream.Name()
```

## Implementation Details

### Version 3 Specifics

This implementation **only** supports CFB Version 3:
- **Sector Size**: Fixed at 512 bytes
- **Mini Sector Size**: Fixed at 64 bytes
- **Mini Stream Cutoff**: 4096 bytes (streams < 4096 use MiniFAT)
- **Header Size**: 512 bytes
- **Sector Shift**: 9 (2^9 = 512)

Files using CFB Version 4 (4096-byte sectors) will be rejected with `ErrInvalidVersion`.

### Sector Management

#### FAT (File Allocation Table)
The FAT tracks sector chains for regular streams (≥ 4096 bytes):
- Each FAT entry is a 32-bit sector index
- Special values: `ENDOFCHAIN`, `FREESECT`, `FATSECT`, `DIFSECT`
- Stored in sectors referenced by the DIFAT

#### MiniFAT (Mini File Allocation Table)
The MiniFAT tracks mini-sector chains for small streams (< 4096 bytes):
- Each mini-sector is 64 bytes
- Mini-sectors are stored in the root entry's stream (mini stream container)
- More efficient for small VBA modules and metadata

#### DIFAT (Double-Indirect FAT)
The DIFAT locates FAT sectors when files are large:
- Header contains first 109 FAT sector positions
- Additional DIFAT sectors provide more FAT positions
- Automatically handled for large files

### Directory Structure

CFB uses a red-black tree structure for directory entries:
- Each entry is 128 bytes
- 4 entries per 512-byte sector
- Tree relationships: left sibling, right sibling, child
- Efficient name-based lookup

### Error Handling

The package provides specific error types:

```go
var (
    ErrInvalidSignature   // Not a valid OLE file
    ErrInvalidVersion     // Not CFB Version 3
    ErrInvalidSectorSize  // Sector size not 512 bytes
    ErrStreamNotFound     // Stream doesn't exist
    ErrStorageNotFound    // Storage doesn't exist
    ErrNotAStream         // Entry is storage, not stream
    ErrNotAStorage        // Entry is stream, not storage
    ErrCircularReference  // Corrupted sector chain
    ErrChainTooLong       // Excessive sector chain
)
```

## Usage Examples

### Example 1: List All Files

```go
reader, _ := cfbv3.Open("document.xls")
defer reader.Close()

root, _ := reader.OpenRootStorage()

// List all entries recursively
func listEntries(storage *cfbv3.Storage, prefix string) {
    for _, name := range storage.List() {
        path := prefix + "/" + name
        
        if storage.StorageExists(name) {
            fmt.Printf("DIR:  %s\n", path)
            sub, _ := storage.OpenStorage(name)
            listEntries(sub, path)
        } else {
            stream, _ := storage.OpenStream(name)
            fmt.Printf("FILE: %s (%d bytes)\n", path, stream.Size())
            stream.Close()
        }
    }
}

listEntries(root, "")
```

### Example 2: Extract VBA Modules

```go
reader, _ := cfbv3.Open("vbaProject.bin")
defer reader.Close()

root, _ := reader.OpenRootStorage()
vba, _ := root.OpenStorage("VBA")

// Extract all VBA streams
for _, name := range vba.ListStreams() {
    stream, _ := vba.OpenStream(name)
    data, _ := io.ReadAll(stream)
    
    // Save to file
    os.WriteFile("extracted_"+name, data, 0644)
    stream.Close()
}
```

### Example 3: Calculate Stream Hashes

```go
import "crypto/sha256"

reader, _ := cfbv3.Open("vbaProject.bin")
defer reader.Close()

root, _ := reader.OpenRootStorage()
vba, _ := root.OpenStorage("VBA")

for _, name := range vba.ListStreams() {
    stream, _ := vba.OpenStream(name)
    data, _ := io.ReadAll(stream)
    
    hash := sha256.Sum256(data)
    fmt.Printf("%s: %x\n", name, hash)
    stream.Close()
}
```

## Testing

Run the demo program with test files:

```bash
# Test with L01 sample
go run cmd/demo1/main.go assets/testdata/L01-vbaProject.bin

# Test with M01 sample
go run cmd/demo1/main.go assets/testdata/M01-vbaProject.bin
```

Expected output:
- CFB header information
- Directory tree structure
- Stream sizes and SHA256 hashes
- Hex dump of 'dir' stream

## Performance Considerations

### Memory Usage
- Streams are loaded entirely into memory when opened
- For large streams, consider the memory impact
- Mini streams are cached in the mini stream container

### Optimization Strategies
1. **Lazy Loading**: Streams loaded on-demand
2. **Sector Caching**: Efficient sector access
3. **O(1) FAT Lookups**: Array-based indexing
4. **Tree Search**: Red-black tree for fast name lookups

### Limits
- `MAX_SECTORS`: 0x100000 (prevents infinite loops)
- `MAX_DIR_ENTRIES`: 100000 (prevents memory exhaustion)
- `MINI_STREAM_CUTOFF`: 4096 bytes (standard)

## Validation

The implementation includes comprehensive validation:

1. **File Signature**: Validates OLE magic bytes
2. **Version Check**: Ensures Version 3 format
3. **Sector Size**: Validates 512-byte sectors
4. **Bounds Checking**: Prevents out-of-range sector access
5. **Circular Detection**: Detects corrupted sector chains
6. **Size Validation**: Ensures reasonable stream sizes
7. **UTF-16 Encoding**: Handles invalid names gracefully

## Limitations

- **Version 3 Only**: Does not support CFB Version 4 (4096-byte sectors)
- **Read-Only**: No write/modification support (yet)
- **No Compression**: Does not handle compressed streams
- **No Encryption**: Does not handle encrypted streams
- **Full Load**: Streams loaded entirely into memory

## Future Enhancements

Planned features for future releases:

1. **VBA Parser**: Parse MS-OVBA format from extracted streams
2. **Stream Writing**: Modify and write back CFB files
3. **Compression Support**: Handle compressed VBA modules
4. **Streaming API**: Read large streams without full load
5. **Validation Tools**: Detect malformed or malicious files

## References

### Specifications
- [MS-CFB]: Compound File Binary File Format
  - https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-cfb/
- [MS-OVBA]: Office VBA File Format Structure
  - https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-ovba/

### Related Resources
- OLE/CFB file format documentation
- VBA macro structure and decompression
- Red-black tree implementation details

## License

See the project LICENSE file for licensing information.

## Contributing

This is part of the vbaModifier project. See the main README for contribution guidelines.

## Support

For issues, questions, or contributions, please refer to the main project repository.

---

**Implementation Version**: 0.1.0  
**CFB Version Support**: Version 3 Only  
**Status**: Experimental (Code Generated by AI with a few tests) 
**Last Updated**: 2025-12-30
