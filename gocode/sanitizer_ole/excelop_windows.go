package sanitizer_ole

import (
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/platform/windoge_utils"
	"cramc_go/telemetry"
	"fmt"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"sync"
)

const (
	cleanupComment = `' Sanitized by CRAMC
Private Sub CRAMCPlaceholder()
    ' This ensures the comment above persists
End Sub
`
)

func createExcelInstance() (*ole.IDispatch, error) {
	// https://learn.microsoft.com/en-us/dotnet/api/microsoft.office.interop.excel?view=excel-pia
	// init excel, no exit before finish all files
	unknownObj, _ := oleutil.CreateObject("Excel.Application")
	excelObj, err := unknownObj.QueryInterface(ole.IID_IDispatch)
	return excelObj, err
}

func (w *ExcelWorker) excelInstanceStartupConfig() {
	if w.currentExcelObj == nil {
		panic("Excel instance is not initialized.")
	}
	// open gui for dbg
	var err error
	if w.inDbg {
		_, err = oleutil.PutProperty(w.currentExcelObj, "Visible", true)
		if err != nil {
			common.Logger.Error(err.Error())
		}
		_, err = oleutil.PutProperty(w.currentExcelObj, "DisplayAlerts", true)
		if err != nil {
			common.Logger.Error(err.Error())
		}
		// boost runtime speed
		_, err = oleutil.PutProperty(w.currentExcelObj, "ScreenUpdating", true)
		if err != nil {
			common.Logger.Error(err.Error())
		}
	} else {
		// security and ux optimize
		_, err = oleutil.PutProperty(w.currentExcelObj, "Visible", false)
		if err != nil {
			common.Logger.Error(err.Error())
		}
		_, err = oleutil.PutProperty(w.currentExcelObj, "DisplayAlerts", false)
		if err != nil {
			common.Logger.Error(err.Error())
		}
		// boost runtime speed
		_, err = oleutil.PutProperty(w.currentExcelObj, "ScreenUpdating", false)
		if err != nil {
			common.Logger.Error(err.Error())
		}
	}
	// ignore remote dde update requests
	_, err = oleutil.PutProperty(w.currentExcelObj, "IgnoreRemoteRequests", true)
	if err != nil {
		telemetry.CaptureException(err, "Excel.Application.SetIgnoreRemoteRequests")
		common.Logger.Error(err.Error())
	}
	// prevent async OLAP data queries from executing
	_, err = oleutil.PutProperty(w.currentExcelObj, "DeferAsyncQueries", true)
	if err != nil {
		common.Logger.Error(err.Error())
	}
	// avoid any macro to execute
	_ = oleutil.MustPutProperty(w.currentExcelObj, "AutomationSecurity", MsoAutomationSecurityForceDisable)
	// also eliminate odbc query timeout
	_, err = oleutil.PutProperty(w.currentExcelObj, "ODBCTimeout", 10)
	if err != nil {
		telemetry.CaptureException(err, "Excel.Application.SetODBCTimeout10s")
		common.Logger.Error(err.Error())
	}
	return
}

type ExcelWorker struct {
	currentExcelObj *ole.IDispatch
	workbooksHandle *ole.IDispatch
	currentWorkbook *ole.IDispatch
	mu              *sync.Mutex
	curFilePath     string
	inDbg           bool
}

func (w *ExcelWorker) Init(inDbg bool) error {
	var err error
	w.currentExcelObj, err = createExcelInstance()
	if err != nil {
		telemetry.CaptureException(err, "Excel.Application.Create")
		common.Logger.Error(err.Error())
		return err
	}
	w.inDbg = inDbg
	w.excelInstanceStartupConfig()
	common.Logger.Info("Excel.Application object initialized.")
	w.mu = &sync.Mutex{}
	return nil
}

func (w *ExcelWorker) Quit(isForced bool) {
	_, _ = w.currentExcelObj.CallMethod("Quit")
	w.currentExcelObj.Release()
	if isForced {
		_, _ = windoge_utils.KillAllOfficeProcesses()
		common.Logger.Info("ExcelWorker Force Terminated.")
	}
	w.currentExcelObj = nil
	w.workbooksHandle = nil
	common.Logger.Info("ExcelWorker Quit.")
	return
}

func (w *ExcelWorker) GetWorkbooks() error {
	if w.currentExcelObj == nil {
		common.Logger.Error(customerrs.ErrExcelWorkerUninitialized.Error())
		return customerrs.ErrExcelWorkerUninitialized
	}
	w.workbooksHandle = oleutil.MustGetProperty(w.currentExcelObj, "Workbooks").ToIDispatch()
	common.Logger.Debug("Workbooks handle requested.")
	return nil
}

func (w *ExcelWorker) OpenWorkbook(fPath string) error {
	if w.workbooksHandle == nil {
		common.Logger.Error(customerrs.ErrExcelWorkbooksUnable2Fetch.Error())
		return customerrs.ErrExcelWorkbooksUnable2Fetch
	}
	//
	// https://learn.microsoft.com/en-us/dotnet/api/microsoft.office.interop.excel.workbooks.open?view=excel-pia#microsoft-office-interop-excel-workbooks-open(system-string-system-object-system-object-system-object-system-object-system-object-system-object-system-object-system-object-system-object-system-object-system-object-system-object-system-object-system-object)
	// Microsoft.Office.Interop.Excel.Workbook Open(string Filename, object UpdateLinks, object ReadOnly,
	// 	object Format, object Password, object WriteResPassword, object IgnoreReadOnlyRecommended,
	// 	object Origin, object Delimiter, object Editable, object Notify, object Converter,
	// 	object AddToMru, object Local, object CorruptLoad);
	//
	// https://stackoverflow.com/questions/14908372/how-to-suppress-update-links-warning
	currentWorkbook, err := oleutil.CallMethod(w.workbooksHandle, "Open", fPath, 0)
	if err != nil {
		telemetry.CaptureException(err, "Excel.Application.Workbooks.Open")
		return err
	}
	w.currentWorkbook = currentWorkbook.ToIDispatch()
	w.curFilePath = fPath
	common.Logger.Info("Workbook currently opened: " + fPath)
	// try to eliminate slow workbook
	_, err = oleutil.PutProperty(w.currentWorkbook, "ForceFullCalculation", false)
	if err != nil {
		telemetry.CaptureException(err, "Excel.ThisWorkbook.SetForceFullCalculationFalse")
		common.Logger.Error(err.Error())
	}
	// disable update embedded ole links
	_, err = oleutil.PutProperty(w.currentWorkbook, "UpdateLinks", XlUpdateLinksNever)
	if err != nil {
		telemetry.CaptureException(err, "Excel.Workbook.SetUpdateLinksNever")
		common.Logger.Error(err.Error())
	}
	// disable update remote ref in workbook
	_, err = oleutil.PutProperty(w.currentWorkbook, "UpdateRemoteReferences", false)
	if err != nil {
		telemetry.CaptureException(err, "Excel.Workbook.SetUpdateRemoteReferencesFalse")
		common.Logger.Error(err.Error())
	}
	// in rare cases, e.g. a ~30M xlsm file may fail to open in 1 min, thus timed out.
	// try to eliminate such cases, these two properties must be set after opening any workbook and
	// will only affect the current instance
	_, err = oleutil.PutProperty(w.currentExcelObj, "Calculation", XlCalculationManual)
	if err != nil {
		telemetry.CaptureException(err, "Excel.Workbook.ParentApp.SetCalculationManual")
		common.Logger.Error(err.Error())
	}
	_, err = oleutil.PutProperty(w.currentExcelObj, "CalculateBeforeSave", false)
	if err != nil {
		telemetry.CaptureException(err, "Excel.Workbook.ParentApp.SetCalculateBeforeSaveFalse")
		common.Logger.Error(err.Error())
	}
	// also do not prompt for format conversion
	// https://learn.microsoft.com/en-us/dotnet/api/microsoft.office.tools.excel.workbook.donotpromptforconvert
	_, err = oleutil.PutProperty(w.currentWorkbook, "DoNotPromptForConvert", false)
	if err != nil {
		common.Logger.Error(err.Error())
	}
	_, err = oleutil.PutProperty(w.currentWorkbook, "CheckCompatibility", false)
	if err != nil {
		common.Logger.Error(err.Error())
	}
	common.Logger.Info("Workbook slowness workarounds applied.")
	return nil

}

func (w *ExcelWorker) SaveAndCloseWorkbook() error {
	if w.currentWorkbook == nil {
		common.Logger.Error(customerrs.ErrExcelCurrentWorkbookNullPtr.Error())
		return customerrs.ErrExcelCurrentWorkbookNullPtr
	}
	_, _ = w.currentWorkbook.CallMethod("Save")
	_, _ = w.currentWorkbook.CallMethod("Close", true)
	w.currentWorkbook.Release()
	common.Logger.Info("Workbook save and closed.")
	w.currentWorkbook = nil
	return nil
}

func (w *ExcelWorker) SanitizeWorkbook(targetOp string, destModuleName string) error {
	if w.currentWorkbook == nil {
		common.Logger.Error(customerrs.ErrExcelCurrentWorkbookNullPtr.Error())
		return customerrs.ErrExcelCurrentWorkbookNullPtr
	}
	wbHasVBA := oleutil.MustGetProperty(w.currentWorkbook, "HasVBProject").Value().(bool)
	if wbHasVBA {
		common.Logger.Info("Workbook has VBProject.")
		wbVbaProjRes, err := oleutil.GetProperty(w.currentWorkbook, "VBProject")
		if err != nil {
			telemetry.CaptureException(err, "Excel.Workbook.VBProject.VBOMAccess")
			return err
		}
		wbVbaProj := wbVbaProjRes.ToIDispatch()
		vbCompsInProj := oleutil.MustGetProperty(wbVbaProj, "VBComponents").ToIDispatch()
		vbCompsCount := (int)(oleutil.MustGetProperty(vbCompsInProj, "Count").Value().(int32))
		common.Logger.Debug(fmt.Sprintf("VBComponents Count: %d", vbCompsCount))
		for i := 1; i <= vbCompsCount; i++ {
			// yes, this bullsh*t index starts from 1...
			vbComp := oleutil.MustCallMethod(vbCompsInProj, "Item", i).ToIDispatch()
			vbCompName := oleutil.MustGetProperty(vbComp, "Name").Value().(string)
			common.Logger.Debug("Current VBComponent Name in iteration: " + vbCompName)
			if vbCompName == destModuleName {
				switch targetOp {
				case "remediate":
					common.Logger.Info("Remediating Matched VBA Component: " + vbCompName)
					// verified in powershell
					codeMod := oleutil.MustGetProperty(vbComp, "CodeModule").ToIDispatch()
					codeModLineCnt := (int)(oleutil.MustGetProperty(codeMod, "CountOfLines").Value().(int32))
					// remove all lines
					_, err := codeMod.CallMethod("DeleteLines", 1, codeModLineCnt)
					if err != nil {
						telemetry.CaptureException(err, "Excel.WorkbookVBACodeModule.DeleteLines_"+w.curFilePath)
						common.Logger.Error(err.Error())
						return err
					}
					_, err = codeMod.CallMethod("AddFromString", cleanupComment)
					if err != nil {
						common.Logger.Error(err.Error())
						return err
					}
					common.Logger.Info("Finished Remediating VBA Module: " + vbCompName)
					return nil
				case "rm_module":
					common.Logger.Info("Removing VBA Module: " + vbCompName)
					_, err = oleutil.CallMethod(vbCompsInProj, "Remove", vbComp)
					if err != nil {
						telemetry.CaptureException(err, "Excel.WorkbookVBAComponents.RemoveModule")
						common.Logger.Error(err.Error())
						return err
					}
					common.Logger.Info("Finished Removal of malicious VBA Module: " + vbCompName)
					return nil
				default:
					common.Logger.Error("Unknown target operation: " + targetOp)
					return customerrs.ErrUnknownInternalError
				}
			}
		}
		return nil
	}
	return customerrs.ErrExcelNoMacroFound
}

func (w *ExcelWorker) Lock() {
	w.mu.Lock()
}

func (w *ExcelWorker) Unlock() {
	w.mu.Unlock()
}
