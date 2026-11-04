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
    $family_marker = "<!--1nternal-->"
    $family_banner = "Triplicate v0.21 /1nternal"
    $family_module = "\"Triplicate\""
    $family_hook   = /\bSub[ \t]+actionhook[ \t]*\([ \t]*tristate[ \t]*\)/

    $disable_word       = "Normal.ThisDocument.DisableAV"
    $disable_powerpoint = "MacroVirusProtection"

  condition:
    (any of ($family_marker, $family_banner) or ($family_module and $family_hook)) and any of ($disable_*)
}
