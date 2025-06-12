//go:build windows

package sanitizer_ole

import (
	"cramc_go/common"
	"cramc_go/platform/windoge_utils"
	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows/registry"
	"strings"
)

const (
	cleanupComment = `' Sanitized by CRAMC
Private Sub CRAMCPlaceholder()
    ' This ensures the comment above persists
End Sub
`
)

func StartSanitizer() error {
	// enable scripting access to VBAObject Model
	err := liftVBAScriptingAccess()
	if err != nil {
		return err
	}
	// kill all office processes, to avoid any potential file lock.
	_, _ = windoge_utils.KillAllOfficeProcesses()
	common.Logger.Infoln("Triggered M365 Office processes killer.")

	// prepare to call ole
	err = ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err != nil {
		return err
	}
	defer ole.CoUninitialize()

	// init excel, no exit before finish all files
	unknownObj, _ := oleutil.CreateObject("Excel.Application")
	excelObj, _ := unknownObj.QueryInterface(ole.IID_IDispatch)
	defer excelObj.Release()
	defer excelObj.CallMethod("Quit")
	// security and ux optimize
	_, err = oleutil.PutProperty(excelObj, "Visible", false)
	if err != nil {
		common.Logger.Errorln(err)
	}
	_, err = oleutil.PutProperty(excelObj, "DisplayAlerts", false)
	if err != nil {
		common.Logger.Errorln(err)
	}
	// ignore remote dde update requests
	_, err = oleutil.PutProperty(excelObj, "IgnoreRemoteRequests", true)
	if err != nil {
		common.Logger.Errorln(err)
	}
	// boost runtime speed
	_, err = oleutil.PutProperty(excelObj, "ScreenUpdating", false)
	if err != nil {
		common.Logger.Errorln(err)
	}
	// avoid any macro to execute
	_ = oleutil.MustPutProperty(excelObj, "AutomationSecurity", MsoAutomationSecurityForceDisable)
	if err != nil {
		common.Logger.Errorln(err)
	}
	// workbooks
	excelWbs := oleutil.MustGetProperty(excelObj, "Workbooks").ToIDispatch()
	for vObj := range common.SanitizeQueue {
		// change path separator, make sure consistent in os-level
		fPathNonVariant := strings.ReplaceAll(vObj.Path, "/", "\\")
		// backup file
		err = gzBakFile(fPathNonVariant)
		if err != nil {
			common.Logger.Errorln("Backup file failed:", err.Error())
		}
		switch vObj.Action {
		case "sanitize":
			// parse and take action
			func() {
				// always defer to save file
				thisWb := oleutil.MustCallMethod(excelWbs, "Open", fPathNonVariant).ToIDispatch()
				defer thisWb.Release()
				defer thisWb.CallMethod("Close", true)
				defer thisWb.CallMethod("Save")
				common.Logger.Infoln("Workbook Opened: ", fPathNonVariant)
				wbHasVBA := oleutil.MustGetProperty(thisWb, "HasVBProject").Value().(bool)
				if wbHasVBA {
					common.Logger.Infoln("Workbook has VBA Project, will be sanitized: ", fPathNonVariant)
					wbVbaProj := oleutil.MustGetProperty(thisWb, "VBProject").ToIDispatch()
					vbCompsInProj := oleutil.MustGetProperty(wbVbaProj, "VBComponents").ToIDispatch()
					vbCompsCount := (int)(oleutil.MustGetProperty(vbCompsInProj, "Count").Value().(int32))
					common.Logger.Debugln("VBComponents Count: ", vbCompsCount)
					for i := 1; i <= vbCompsCount; i++ {
						// yes, this bullsh*t index starts from 1...
						vbComp := oleutil.MustCallMethod(vbCompsInProj, "Item", i).ToIDispatch()
						vbCompName := oleutil.MustGetProperty(vbComp, "Name").Value().(string)
						if vbCompName == vObj.DestModule {
							common.Logger.Infoln("Sanitizing Matched VBA Component: ", vbCompName)
							// verified in powershell
							codeMod := oleutil.MustGetProperty(vbComp, "CodeModule").ToIDispatch()
							codeModLineCnt := (int)(oleutil.MustGetProperty(codeMod, "CountOfLines").Value().(int32))
							// remove all lines
							_, err = codeMod.CallMethod("DeleteLines", 1, codeModLineCnt)
							if err != nil {
								common.Logger.Errorln(err)
								continue
							}
							_, err = codeMod.CallMethod("AddFromString", cleanupComment)
							if err != nil {
								common.Logger.Errorln(err)
								continue
							}
							common.Logger.Infoln("Finished Sanitizing VBA Module: ", vbCompName)
						}
					}
				}
			}()
			// rename file and save to clean state cache of cloud-storage provider
			err = renameFileAndSave(fPathNonVariant)
			if err != nil {
				common.Logger.Errorln("Rename file failed:", err.Error())
			}
			common.Logger.Infoln("Workbook Sanitized: ", fPathNonVariant)
		default:
			common.Logger.Warnln("Unsupported action type: ", vObj.Action)
			continue
		}
	}
	common.Logger.Infoln("Sanitizer Finished.")
	return nil
}

func liftVBAScriptingAccess() error {
	// this fix COM API via OLE returned null on VBProject access
	regK, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Office\16.0\Common\Security`, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	err = regK.SetDWordValue("AccessVBOM", 1)
	if err != nil {
		return err
	}
	return nil
}

type MsoAutomationSecurity int

const (
	MsoAutomationSecurityLow = 1 + iota
	MsoAutomationSecurityByUI
	MsoAutomationSecurityForceDisable
)
