# CRAMC (CRAppy Macro Cleaner)

My Crappy Macro Cleaner - For Sanitizing Malicious Macro in Office Files when AV not work

> Today is 2025! Why are you still writing such a thing?
>> Because some "AI-Native" and "Advanced" EDR/NGAV we use can't prevent infection. Then files got blocked by our storage service provider, which broke our operation.

# Usage

- You must run at least Windows 10.
- (optional) To ensure availability, this program is recommended to run under Administrator and in path `%ProgramData%\CRAMC` .
- DO NOT put this program in cloud-storage folder.
- Please whitelist its path in CrowdStrike due to false-positive ML detection.
- It's designed to run under either privileged or unprivileged situation. You don't have to manually elevate.
- Please always try to run the latest version of the program.
- It's recommended to run [cleanup_stale.ps1](./assets/cleanup_stale.ps1) with unprivileged target user before starting the program.
- Download `cramc_go_{numeric ID}.zip` and extract all files to a new empty folder, run `cramc_aio.exe`.
- Files under `C:\TMP` and `%AppData%\Microsoft\Excel` and `%LocalAppData%\Microsoft\Windows\INetCache` will be removed.

# Disclaimer

Backup your data before you use it. No warranty at all.

# Compile

- https://virustotal.github.io/yara-x/docs/api/c/c-/

Since Yara-X introduced more strict rule syntax verifier, we use git pre-commit hook to format your rules:
```bash
# install yara-x before you do anything
cp ./assets/pre-commit-hooks.sh ./.git/hooks/pre-commit
chmod +x ./.git/hooks/pre-commit
```

For more detailed instruction about compiling on Ubuntu (or any Debian-based distros), please refer to [CI configuration](./.github/workflows/v4_rustbuild.yml) .

# License

For program: GNU AGPL v3

For yara rules: Licensed under CC BY-NC-SA 4.0 International

# Credit

Thanks to those libraries:

BSD-3-Clause:
- github.com/VirusTotal/yara-x
- github.com/google/uuid
- github.com/klauspost/compress/internal/snapref
- github.com/shirou/gopsutil/v4
- `golang.org/x/*`
- google.golang.org/protobuf

Apache-2.0:
- github.com/klauspost/compress
- google.golang.org/genproto/googleapis/rpc/status
- google.golang.org/grpc
- www.velocidex.com/golang/go-ntfs/parser

MIT:
- github.com/klauspost/compress/zstd/internal/xxhash
- github.com/Microsoft/go-winio
- github.com/yusufpapurcu/wmi

Others:
- github.com/davecgh/go-spew (ISC)

Mixed:
- github.com/microsoft/windows-rs (Apache-2.0, MIT)

# Privacy Notice

To help us serve you better, we are collecting program crash and error context information using service provided by BetterStack.com , their privacy policy could be found [here](https://betterstack.com/privacy) . Our team won't sell your information, collected information is only used for necessary troubleshooting purpose.
