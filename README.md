# CRAMC (CRAppy Macro Cleaner)

My Crappy Macro Cleaner - For Sanitizing Malicious Macro in Office Files when AV not work

> Today is 2025! Why are you still writing such a thing?
>> Because some "AI-Native" and "Advanced" EDR/NGAV we use can't prevent infection. Then file got blocked by our storage service provider make our operation a disaster.

# Components

- Scanner: Provided by Yara-X/Yara
- Cleaner: Written by myself using C-Sharp

# Usage

- Make sure at least .NetFX4 installed on your machine.

# Developer Notes

- Only merged and compiled yara rule should be distributed
- Yara binary is always bundled
- `cramc_db.json` is k-v store, k should be rule name, v should be operation
- `cramc_conf.json` is generally for spawning runner and cleaner on the machine.
- Before remediation, file should always be backed-up.

# License

For program: GNU AGPL v3

For yara rules: Licensed under CC BY-NC-SA 4.0 International

# Credit

Thanks to:
- https://github.com/VirusTotal/yara-x (BSD-3)
- https://github.com/VirusTotal/yara (BSD-3)