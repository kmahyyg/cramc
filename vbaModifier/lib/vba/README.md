# lib/vba

`lib/vba` provides VBA-project-specific operations on top of CFB storage:

- Open VBA projects from `vbaProject.bin` bytes.
- Parse project metadata from the `PROJECT` stream.
- Parse module metadata from the compressed `dir` stream.
- Read module stream payloads.
- Compress/decompress MS-OVBA compressed containers.
- Patch module source text offsets in the `dir` stream.
- Remove `_VBA_PROJECT` performance cache payload on write.

## Package Scope

This package is not a full VBA interpreter. It focuses on stream-level parsing and rewriting utilities required for static extraction/sanitization workflows.

## Public API Highlights

Project reader:

- `OpenVBAProjectBytes(vbaProjectBin []byte, isLegacyFormat bool) (*Reader, error)`
- `(*Reader).ParseAllStreams() (*ProjectStreams, error)`
- `(*Reader).GetModuleContent(name string) ([]byte, error)`
- `(*Reader).Close() error`
- `OpenVBAStorage(root *cfbv3.Storage, isLegacyFormat bool) (*cfbv3.Storage, error)`

Compression and parsing helpers:

- `Compress(src []byte) ([]byte, error)`
- `Decompress(src []byte) ([]byte, error)`
- `PatchModuleTextOffsetInDirStream(rawCompressedDir []byte, moduleName string, newOffset uint32) ([]byte, []ModuleTextOffsetInfo, error)`
- `StripVBAProjectPerformanceCache(raw []byte) []byte`

Crypto/hash helpers:

- `XORCipher(data []byte, key []byte) []byte`
- `ContentHashSHA1(data []byte) [20]byte`
- `ContentHashHex(data []byte) string`

## Data Types

Important exported structures include:

- `ProjectStreams`
- `ProjectMetadata`
- `DirStream`
- `Module`
- `ModuleType`
- `ModuleTextOffsetInfo`

## Modern vs Legacy Inputs

The `isLegacyFormat` flag controls storage root handling:

- `false`: modern Office (`.xlsm`, `.xlsb`) where VBA lives directly under the CFB root in `vbaProject.bin`.
- `true`: legacy CFB workbook layout where project streams are under `_VBA_PROJECT_CUR`.

## Usage Example

```go
package main

import (
    "fmt"

    "vbaModifier/lib/vba"
)

func inspectVBA(vbaProjectBin []byte, legacy bool) error {
    rdr, err := vba.OpenVBAProjectBytes(vbaProjectBin, legacy)
    if err != nil {
        return err
    }
    defer rdr.Close()

    parsed, err := rdr.ParseAllStreams()
    if err != nil {
        return err
    }

    fmt.Printf("Project: %s\n", parsed.Project.Name)
    for _, m := range parsed.DirStream.Modules {
        rawModule, err := rdr.GetModuleContent(m.Name)
        if err != nil {
            return err
        }
        source, err := vba.Decompress(rawModule[int(m.TextOffset):])
        if err != nil {
            return err
        }
        fmt.Printf("Module=%s Type=%s Bytes=%d\n", m.Name, m.Type, len(source))
    }

    return nil
}
```

## Error Behavior

Common package errors:

- `ErrProjectStorageNotFound`
- `ErrProjectMetadataStreamMissing`
- `ErrUnsupportedEncryptedProject`
- `ErrDirStreamMalformed`
- `ErrModuleNotFound`
- `ErrInvalidSignature`
- `ErrCompressionFailed`
- `ErrDecompressionFailed`

Password-protected projects are currently rejected with `ErrUnsupportedEncryptedProject`.

## Compliance and Tests

The compression logic is tested for:

- Round-trip decompression correctness.
- Avoiding non-compliant short raw chunks in compressed containers.

Integration tests in `docparser` validate extraction and replacement behavior using real sample files.

## License

This package is part of the repository and is licensed under **AGPL-3.0-only**.

## References

- MS-OVBA openstandard: https://docs.microsoft.com/en-us/openspecs/office_file_formats/ms-ovba/
- MS-XLS openstandard: https://docs.microsoft.com/en-us/openspecs/office_file_formats/ms-xls/

## Attribution and Generation Note

Parts of this repository were generated with assistance from LLM tools. Review and test before production use.
