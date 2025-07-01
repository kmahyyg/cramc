rule VirusX97MLarouxCF {
  meta:
    author     = "kmahyyg@GitHub"
    license    = "CC BY-NC-SA 4.0 International"
    confidence = "medium"

  strings:
    $comment_m1 = "donwload NEG!!! NoMercyExcelGenerator form NoMercyPage!"
    $comment_m2 = "foxz@usa.net"
    $comment_m3 = "infected by NEG"
    $comment_m4 = "check_files"
    $comment_m5 = "Module=foxz"
    $comment_m6 = "NEGS.XLS"

  condition:
    all of them
}
