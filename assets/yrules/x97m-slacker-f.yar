rule VirusX97MSlackerF {
  meta:
    author     = "kmahyyg@GitHub"
    license    = "CC BY-NC-SA 4.0 International"
    confidence = "high"

  strings:
    $comment_m1           = "'Private Sub Workbook_BeforeSave(ByVal SaveAsUI As Boolean"
    $comment_m2           = "'If UCase(ThisWorkbook.Name"
    $comment_m3           = "'Application.Dialogs(xlDialogSaveAs).Show"
    $comment_m4           = "'OOO"
    $comment_m5           = ") = \"BOOK1\" Then"
    $code_takecare_of_mru = "AddToMru"
    $file_head            = "Excel.Application"
    $file_mod             = "ThisWorkbook"

  condition:
    all of them
}
