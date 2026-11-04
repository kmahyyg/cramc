/*
 * Detects the supplied Triplicate v0.21 /1nternal (TriState) VBA source.
 * Scan extracted/decompressed VBA, not compressed Office containers.
 * https://www.f-secure.com/v-descs/tristate (v0.21 is documented as Tristate.C)
 * https://threats.kaspersky.com/en/threat/Virus.MSOffice.Triplicate/
 */
rule VirusO97MTristateTriplicate {
  meta:
    author      = "kmahyyg@GitHub"
    license     = "CC BY-NC-SA 4.0 International"
    description = "Triplicate/TriState cross-application VBA replication"
    confidence  = "high"

  strings:
    $family_marker = "<!--1nternal-->" ascii wide nocase
    $family_banner = "Triplicate v0.21 /1nternal" ascii wide nocase
    $family_module = "\"Triplicate\"" ascii wide nocase
    $family_hook   = /\bSub[ \t]+actionhook[ \t]*\([ \t]*tristate[ \t]*\)/ ascii wide nocase

    $inject = ".InsertLines" ascii wide nocase

    $persist_excel      = "\\Book1." ascii wide nocase
    $persist_powerpoint = "\\Templates\\Blank Presentation.pot" ascii wide nocase

    $disable_word       = "Normal.ThisDocument.DisableAV" ascii wide nocase
    $disable_powerpoint = "MacroVirusProtection" ascii wide nocase

  condition:
    (any of ($family_marker, $family_banner) or ($family_module and $family_hook)) and
    $inject and any of ($persist_*) and any of ($disable_*)
}
