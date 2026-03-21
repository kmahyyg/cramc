# Developer Notes

- Only merged and compiled yara rules should be distributed
- Yara-X is always bundled
- Before remediation, original file should always be backed-up.
- `databaseVersion` is for both cleanup db and yara rules.
- [Figma](https://www.figma.com/board/DGvlxo4XXQTZ8skqmJFFUh/CRAMC) link to control flow.
- Always assume users are unprivileged, auto-request elevation based on sys manifest.
- `xl/vbaProject.bin` and `.xls` is OLE compound file (binary and proprietary format), format standard [here](https://learn.microsoft.com/en-us/openspecs/office_file_formats/MS-OFFFFLP/6ae2fd93-51fc-4e75-a54a-1b175c627b51) .
- Unfortunately, due to cost-effectiveness consideration and I'm developing this alone, I had to take COM+ API approach to sanitize infected files, which made this software completely rely on MS Excel executable and broke its cross-platform ability.
- Maybe worth a read: `https://attack.mitre.org/techniques/T1564/007/`

Since Yara-X introduced more strict rule syntax verifier, we use git pre-commit hook to format your rules:
```bash
# install yara-x before you do anything
cp ./assets/pre-commit-hooks.sh ./.git/hooks/pre-commit
chmod +x ./.git/hooks/pre-commit
```