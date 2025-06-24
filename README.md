# CRAMC (CRAppy Macro Cleaner)

My Crappy Macro Cleaner - For Sanitizing Malicious Macro in Office Files when AV not work

> Today is 2025! Why are you still writing such a thing?
>> Because some "AI-Native" and "Advanced" EDR/NGAV we use can't prevent infection. Then files got blocked by our storage service provider, which broke our operation.

# Usage

- At Least Windows 10.
- To ensure availability, this program is recommended to run under Administrator and in path `%ProgramData%\CRAMC`  (optional)
- Do not put this program in cloud-storage folder.

# Disclaimer

Backup your data before you use it. No warranty at all.

# Developer Notes

- Only merged and compiled yara rule should be distributed
- Yara is always bundled
- `cramc_db.json` is k-v store, k should be rule name, v should be operation
- Before remediation, original file should always be backed-up.
- `databaseVersion` is for both cleanup db and yara rules.
- [Figma](https://www.figma.com/board/DGvlxo4XXQTZ8skqmJFFUh/CRAMC) link to control flow.
- Always assume users are unprivileged, auto-request elevation based on sys manifest.
- On MacOS, `-static` extldflags won't work.
- `xl/vbaProject.bin` and `.xls` is OLE compound file (binary and proprietary format), format standard [here](https://learn.microsoft.com/en-us/openspecs/office_file_formats/MS-OFFFFLP/6ae2fd93-51fc-4e75-a54a-1b175c627b51) .
- Unfortunately, due to cost-effectiveness consideration and I'm developing this alone, I had to take COM+ API approach to sanitize infected files, which made this software completely rely on MS Excel executable and broke its cross-platform ability.

# Compile

- https://github.com/hillu/go-yara/blob/master/README.cross-building.md
- https://yara.readthedocs.io/en/v4.3.2/gettingstarted.html

# License

For program: GNU AGPL v3

For yara rules: Licensed under CC BY-NC-SA 4.0 International

# Credit

Thanks to:
- https://github.com/VirusTotal/yara (BSD-3)

# Privacy Notice

To help us serve you better, we are collecting program crash and error context information using service provided by BetterStack.com , their privacy policy could be found [here](https://betterstack.com/privacy) . Our team won't sell your information, collected information is only used for necessary troubleshooting purpose.
