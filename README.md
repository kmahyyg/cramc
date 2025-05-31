# CRAMC (CRAppy Macro Cleaner)

My Crappy Macro Cleaner - For Sanitizing Malicious Macro in Office Files when AV not work

> Today is 2025! Why are you still writing such a thing?
>> Because some "AI-Native" and "Advanced" EDR/NGAV we use can't prevent infection. Then file got blocked by our storage service provider make our operation a disaster.

# Components

- Scanner: Provided by Yara-X/Yara
- Cleaner: Written by myself using C-Sharp

# Usage

- At Least Windows 10.
- To ensure availbility, this program is recommended to run under Administrator and in path `%ProgramData%\CRAMC` 
- Do not put this program in cloud-storage folder.

I'm trying to build golang version for everything, using GRPC to communicate with csharp to interact with proprietary office files. This would greatly reduce my workload and better for maintenance.

# Disclaimer

Backup your data before you use it. No warranty at all.

# Developer Notes

- Only merged and compiled yara rule should be distributed
- Yara binary is always bundled
- `cramc_db.json` is k-v store, k should be rule name, v should be operation
- `cramc_conf.json` is generally for spawning runner and cleaner on the machine.
- Before remediation, file should always be backed-up.
- `databaseVersion` is for both cleanup db and yara rules.
- [Figma](https://www.figma.com/board/DGvlxo4XXQTZ8skqmJFFUh/CRAMC) link to control flow.

# License

For program: GNU AGPL v3

For yara rules: Licensed under CC BY-NC-SA 4.0 International

# Credit

Thanks to:
- https://github.com/VirusTotal/yara-x (BSD-3)
- https://github.com/VirusTotal/yara (BSD-3)

# Privacy Notice

To help us serve you better, we are collecting program crash and error context information using service provided by Sentry.io , their privacy prolicy could be find [here](https://sentry.io/privacy/) . Our team won't collect any information that could link to you.
