# lib/cfb/v3

`lib/cfb/v3` is a minimal mutable Compound File Binary (CFB/OLE) reader-writer used by this repository to edit VBA project streams in memory.

## What It Provides

- Open a CFB document from bytes.
- Traverse storages and streams by name.
- Read stream payloads.
- Replace or remove stream payloads.
- Serialize the modified CFB back to bytes.

This package is focused on the subset of CFB behavior needed for Office VBA container rewriting.

## Public API Snapshot

Core types:

- `Reader`
- `Storage`

Key functions and methods:

- `OpenBytes(data []byte) (*Reader, error)`
- `(*Reader).OpenRootStorage() (*Storage, error)`
- `(*Reader).WriteTo(w io.Writer) (int64, error)`
- `(*Reader).Close() error`
- `(*Storage).OpenStorage(name string) (*Storage, error)`
- `(*Storage).OpenStream(name string) (io.ReadCloser, error)`
- `(*Storage).ReplaceStream(name string, data []byte) error`
- `(*Storage).RemoveStream(name string) error`
- `(*Storage).StreamExists(name string) bool`
- `(*Storage).StorageExists(name string) bool`
- `(*Storage).ListStreams() []string`
- `(*Storage).ListStorages() []string`

## Typical Usage

```go
package main

import (
    "bytes"
    "io"
    "os"

    cfbv3 "vbaModifier/lib/cfb/v3"
)

func main() {
    input, err := os.ReadFile("vbaProject.bin")
    if err != nil {
        panic(err)
    }

    reader, err := cfbv3.OpenBytes(input)
    if err != nil {
        panic(err)
    }
    defer reader.Close()

    root, err := reader.OpenRootStorage()
    if err != nil {
        panic(err)
    }

    // Example: open VBA storage and read the compressed dir stream.
    vbaStorage, err := root.OpenStorage("VBA")
    if err != nil {
        panic(err)
    }

    dirStream, err := vbaStorage.OpenStream("dir")
    if err != nil {
        panic(err)
    }
    dirRaw, err := io.ReadAll(dirStream)
    _ = dirStream.Close()
    if err != nil {
        panic(err)
    }

    // Replace stream bytes.
    if err := vbaStorage.ReplaceStream("dir", dirRaw); err != nil {
        panic(err)
    }

    // Write modified CFB bytes.
    var out bytes.Buffer
    if _, err := reader.WriteTo(&out); err != nil {
        panic(err)
    }
    if err := os.WriteFile("vbaProject.modified.bin", out.Bytes(), 0o644); err != nil {
        panic(err)
    }
}
```

## Notes and Constraints

- Name matching is case-sensitive for storage/stream lookup.
- Serialization currently supports layouts that fit within the DIFAT entries embedded in the header (up to 109 FAT sectors).
- Small streams are handled through the mini-stream/mini-FAT path; larger streams use regular FAT chains.
- This package does not expose an API for creating entirely new streams/storages; it is currently optimized for opening and mutating existing entries.

## Where It Is Used

- `lib/vba` uses this package to access `PROJECT`, `dir`, and module streams.
- `docparser` uses it for rewriting stream content and re-serializing patched VBA projects.

## License

This package is part of the repository and is licensed under **AGPL-3.0-only**.

## Reference

MS-CFB openstandard: https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-cfb/

## Attribution and Generation Note

Parts of this repository were generated with assistance from LLM tools. Review and test before production use.
