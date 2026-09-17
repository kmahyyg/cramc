/*
 * https://www.trendmicro.com/vinfo/us/threat-encyclopedia/malware/Virus.Win32.NAPWHICH.C
 * https://x.com/TheDFIRReport/status/1329123402922201089
 * https://www.esentire.com/blog/xred-backdoor-the-hidden-threat-in-trojanized-programs
 * being detected as: Virus.Win32.NAPWHICH.C or  Win32/Delf.NBX or  Win32:Zorex-E or Trojan-PSW.Win32.Cryptnot or Trojan.Win32.XRed.azn
 * Multiple samples are including PE files and Excel/Word documents with macros. This rule only applies to macro code.
 */

rule TrojanNapwhichXred {
  meta:
    author     = "kmahyyg@GitHub"
    license    = "CC BY-NC-SA 4.0 International"
    confidence = "medium"

  strings:
    $b_download_url  = "0BxsMXGfPIZfSVzUyaHFYVkQxeFk"
    $b_download_url2 = "zhp1b06imehwylq/Synaptics.rar"

    $c_dest_file         = "\\Synaptics\\Synaptics.exe"
    $c_dest_file_attr    = "vbHidden + vbSystem"
    $c_dest_path         = "\\~$cache1"
    $c_loose_restriction = "\\Security\\VBAWarnings\", 1, \"REG_DWORD\""

  condition:
    (all of ($c_*)) or (any of ($b_*))
}
