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

## Triplicate / TriState VBA rule research

The supplied text sample identifies itself as `Triplicate v0.21 /1nternal` and carries the marker `'<!--1nternal-->'`. F-Secure's [TriState family report](https://www.f-secure.com/v-descs/tristate) lists **Tristate, Triplicate, and Crown** as aliases, describes the three-Office-app infection behavior, and specifically associates **Tristate.C** with the visible text `Triplicate v0.21 /1nternal`. This matches the supplied sample's version string, rather than merely the generic word “tristate.” Microsoft's [Virus:O97M/Tristate.CO report](https://www.microsoft.com/en-us/wdsi/threats/malware-encyclopedia-description?Name=Virus:O97M/Tristate.CO) independently maps it to Kaspersky `Virus.MSOffice.Triplicate.c` and confirms the `<!--1nternal-->` infection marker, `Book1.xls` Excel startup persistence, `Triplicate` module in `Blank Presentation.pot`, invisible slide-sized shape, and `actionhook` activation. Kaspersky's [Triplicate technical report](https://threats.kaspersky.com/en/threat/Virus.MSOffice.Triplicate/) corroborates the Word/Excel/PowerPoint replication, `BOOK1` startup file, PowerPoint module and shape hook, and a one-in-seven activation chance.

**Useful family-specific VBA indicators:** the exact marker `'<!--1nternal-->'`; version string `Triplicate v0.21 /1nternal`; the procedure/signature `actionhook(tristate)`; and the coupled persistence/infection strings `Book1.` / `Book1.xls`, `Blank Presentation.pot`, and module name `Triplicate`. The sample also includes `Private Sub Document_Close()`, `Private Sub Workbook_Deactivate()`, the `NormalTemplate` infection path, registry value names `Options6` and `MacroVirusProtection`, and the generated `DisableAV()` routine. Prefer a conjunction of the marker or version string with multiple behavior strings (e.g. `actionhook(tristate)` plus `Blank Presentation.pot` or `Book1.`) to avoid matching on a generic identifier or a common Office event alone; treat `DisableAV` and `tristate` alone as weak. Signature scope should be **extracted VBA source text**, not raw compressed Office containers: the sample supplied is already text, and the cited vendor reports describe code-level strings/behavior, not a stable byte sequence in compressed Office storage.

The implemented rule is [`VirusO97MTristateTriplicate`](yrules/o97m-tristate-triplicate.yar). It requires a family marker/banner (or the `Triplicate` module name together with the `actionhook(tristate)` declaration), `.InsertLines`, an Excel/PowerPoint persistence path, and a protection-disabling indicator. Strings support ASCII and UTF-16LE, case-insensitively.

Smoke-checked with the installed YARA-X 1.22.1 CLI (`yr` on this workstation): the supplied sample and all three individual event procedures matched; all 24 temporary cases passed (17 positive, 7 negative), including case/line-ending changes, UTF-16LE, markerless fallback, benign automation, and missing required indicator groups. No malicious VBA was executed. To scan extracted VBA manually, use `yarax scan assets/yrules/o97m-tristate-triplicate.yar /path/to/extracted-vba.txt` (substitute `yr` if that is the installed executable name).