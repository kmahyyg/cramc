# AGENTS.md — CRAMC Codebase Guide

## Overview
CRAMC (CRAppy Macro Cleaner) is a Windows-only Office macro sanitizer. It scans Excel files with YARA-X rules, extracts/replaces malicious VBA code in-place, and backs up originals before remediation. The Go workspace (`go.work`) contains two modules: `cramc_go` (all binaries and logic) and `vbaModifier` (vendored CFB v3/VBA parsing library).

## Module & Binary Layout
| Module | Binary | CGO | Target |
|---|---|---|---|
| `cramc_go/cmd/aioagent` | `cramc_aio.exe` | Required (yara-x) | Windows/amd64 |
| `cramc_go/cmd/bakrestorer` | `bakrestorer.exe` | No | Windows/amd64 |
| `cramc_go/cmd/devreleaser` | `devreleaser` | Required (yara-x) | Linux/amd64 (build tool only) |

`vbaModifier` is a local module; its library code lives under `vbaModifier/lib/cfb/v3` and `vbaModifier/lib/vba` (vendored, not on pkg.go.dev). `docparser` imports it directly as `vbaModifier/lib/cfb/v3` and `vbaModifier/lib/vba`. 

`vbaModifier/lib/vba` focused on parsing / extracting / replacing VBA binary data inside CFB stream, also support in-memory data as underlying data storage. This library relies on `vbaModifier/lib/cfb/v3` for reading and writing underlying CFB storage file. All `vbaModifier` code is pure Go without any CGo or non-official dependency.

## Build System
All production builds run on Ubuntu via `assets/build.sh` (driven by GitHub Actions). Local builds on macOS are **not supported** for the Windows target.

Key build flags that must always be set:
```bash
# VersionStr must be injected at link time — it is empty string at runtime otherwise
-ldflags "-X \"cramc_go/common.VersionStr=$(git describe --long --dirty --tags)\""

# CGO cross-compile for Windows
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc
PKG_CONFIG_PATH=<yara-x win prefix>/lib/pkgconfig

# Static link tag must accompany static extldflags
-tags static_link -extldflags "-static -lm -static-libgcc -static-libstdc++"

# Debug builds add (no stripping, goroutine leak profiling)
GOEXPERIMENT=goroutineleakprofile -gcflags 'all=-N -l'
```

Before building `aioagent`, generate the Windows resource file:
```bash
cd cramc_go/cmd/aioagent
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go-winres make --product-version=git-tag --file-version=git-tag
```

The YARA-X C library must be compiled from the Rust source (`cargo cinstall -p yara-x-capi --release --crt-static --library-type staticlib`) before any CGO build. A linker workaround file `assets/linkerdeps/lib/libwindows.0.53.0.a` must be copied into the yara-x prefix lib directory for Windows cross-compilation.

## YARA Rules Workflow
1. Place/edit `.yar` files under `assets/yrules/`.
2. Install the `yara-x` CLI and set up the pre-commit hook to auto-format rules:
   ```bash
   cp ./assets/pre-commit-hooks.sh ./.git/hooks/pre-commit && chmod +x ./.git/hooks/pre-commit
   ```
3. During CI, `devreleaser -compile` merges and compiles all rules → `yrules/bin/unified.yar`.
4. `devreleaser -enc=true -in=./yrules/bin/unified.yar -out=../bin/unified.yar.bin` encrypts the compiled binary with the hardcoded key.
5. `cramc_aio.exe` expects `unified.yar.bin` **in the same directory as the executable** at runtime (fix #9).

## Encryption & Backup Format
- All encrypted files (YARA rules, config, backups) use **XChacha20-Poly1305** via `cramc_go/cryptutils/xchacha20.go`.
- Key is hardcoded at `common.HexEncryptionPassword` — this is intentional and acknowledged as weak.
- Backup files are written as `<originalpath>.zst.ebak`: zstd-compressed then encrypted, with the original file path as the additional authenticated data (AMAD).
- The binary layout is documented in `assets/crypt-bakrestorer.md`. Schema version constant: `cryptutils.AddiMsgVersionInAssoData = 1`.

## Processing Pipeline (`cmd/aioagent`)
```
GeneralWalkthroughSearch → searcherOptChan (cap 50)
  → searchConsumer (checks on-disk size) → scanIptChan (cap 50)
    → SanitizeFilesWithYara (YARA scan + VBA extract + ReplaceMaliciousCode)
```
All stages run concurrently under a single `sync.WaitGroup`. `DryRunOnly` is checked by every callsite that mutates files.

## File Format Handling
- `.xlsx`, `.xlsm`, `.xlsb`: treated as ZIP archives; `xl/vbaProject.bin` is extracted (`fileutils.DecompressMacroBin`), parsed as CFBv3 file, VBA modules sanitized, then the ZIP is rewritten (`fileutils.ReplaceXLVBAProjectBin`).
- `.xls`: native CFBv3 file, read directly without ZIP extraction (`isLegacyFormat = true`).
- VBA sanitization replaces module body with `' Sanitized by CRAMC v0.6.0\r\n` while preserving `Attribute` header lines, removes `PerformanceCache` section from corresponding module binary stream and `_VBA_PROJECT` stream, patch module `TextOffset` recorded in `dir` stream to zero, then `__SRP_*` streams are removed from the CFB storage.
- CFB storage is following [MS-CFB standard](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-cfb/), VBA project structure follows [MS-OVBA standard](https://learn.microsoft.com/en-us/openspecs/office_file_formats/ms-ovba/). `vbaModifier` library implements custom parsing/writing logic for both formats, with a focus on preserving all non-malicious content and metadata.

## Version & Update Check
- `common.ProgramRev` (integer) must match `programRevision` in `assets/latest_version.json` (fetched from GitHub at startup). Mismatch causes hard exit — **bump `ProgramRev` in `common/shared.go` and `latest_version.json` together** on every release.
- `VersionStr` is empty at compile time; always set via ldflags.

## Runtime Environment
- **Windows only** for `cramc_aio.exe`; privilege elevation is handled via `app.manifest` (UAC).
- Set `RunEnv=DEBUG` or `RunEnv=NOSPAWN` for debug-level JSON logging; any non-empty `RunEnv` also skips killing Office processes (useful for local testing).
- Log output goes to both stdout and a `.log` file created in the working directory (JSON `slog` format with custom `TRACE`/`FATAL` levels).
- `common.Logger` is a package-level `*slog.Logger`; initialize it before calling any other package function.

## Key Files
| Path | Purpose                                                |
|---|--------------------------------------------------------|
| `cramc_go/common/shared.go` | Global state, hardcoded encryption key, `ProgramRev`   |
| `cramc_go/common/datamodels.go` | `YaraScanResult`, `ExtractedVBAModule` shared structs  |
| `cramc_go/docparser/vba_ops.go` | VBA extract & replace logic, imports `vbaModifier`     |
| `cramc_go/cryptutils/xchacha20.go` | Encrypt/decrypt with embedded AMAD layout              |
| `assets/build.sh` | Authoritative build script; CI entry point             |
| `assets/crypt-bakrestorer.md` | Binary layout spec for `.zst.ebak` files               |
| `assets/latest_version.json` | Remote version manifest checked at startup             |
| `assets/testdata/*.xl*` | Test files for various Excel formats, some with macros |

