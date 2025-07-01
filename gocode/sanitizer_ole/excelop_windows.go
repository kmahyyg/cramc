package sanitizer_ole

import (
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/platform/windoge_utils"
	"cramc_go/telemetry"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"sync"
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
			common.Logger.Errorln(err)
		}
		_, err = oleutil.PutProperty(w.currentExcelObj, "DisplayAlerts", true)
		if err != nil {
			common.Logger.Errorln(err)
		}
		// boost runtime speed
		_, err = oleutil.PutProperty(w.currentExcelObj, "ScreenUpdating", true)
		if err != nil {
			common.Logger.Errorln(err)
		}
	} else {
		// security and ux optimize
		_, err = oleutil.PutProperty(w.currentExcelObj, "Visible", false)
		if err != nil {
			common.Logger.Errorln(err)
		}
		_, err = oleutil.PutProperty(w.currentExcelObj, "DisplayAlerts", false)
		if err != nil {
			common.Logger.Errorln(err)
		}
		// boost runtime speed
		_, err = oleutil.PutProperty(w.currentExcelObj, "ScreenUpdating", false)
		if err != nil {
			common.Logger.Errorln(err)
		}
	}
	// ignore remote dde update requests
	_, err = oleutil.PutProperty(w.currentExcelObj, "IgnoreRemoteRequests", true)
	if err != nil {
		telemetry.CaptureException(err, "Excel.Application.SetIgnoreRemoteRequests")
		common.Logger.Errorln(err)
	}
	// prevent async OLAP data queries from executing
	_, err = oleutil.PutProperty(w.currentExcelObj, "DeferAsyncQueries", true)
	if err != nil {
		common.Logger.Errorln(err)
	}
	// avoid any macro to execute
	_ = oleutil.MustPutProperty(w.currentExcelObj, "AutomationSecurity", MsoAutomationSecurityForceDisable)
	// also eliminate odbc query timeout
	_, err = oleutil.PutProperty(w.currentExcelObj, "ODBCTimeout", 10)
	if err != nil {
		telemetry.CaptureException(err, "Excel.Application.SetODBCTimeout10s")
		common.Logger.Errorln(err)
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
		common.Logger.Errorln(err)
		return err
	}
	w.inDbg = inDbg
	w.excelInstanceStartupConfig()
	common.Logger.Infoln("Excel.Application object initialized.")
	w.mu = &sync.Mutex{}
	return nil
}

func (w *ExcelWorker) Quit(isForced bool) {
	_, _ = w.currentExcelObj.CallMethod("Quit")
	w.currentExcelObj.Release()
	if isForced {
		_, _ = windoge_utils.KillAllOfficeProcesses()
		common.Logger.Infoln("ExcelWorker Force Terminated.")
	}
	w.currentExcelObj = nil
	w.workbooksHandle = nil
	common.Logger.Infoln("ExcelWorker Quit.")
	return
}

func (w *ExcelWorker) GetWorkbooks() error {
	if w.currentExcelObj == nil {
		common.Logger.Errorln(customerrs.ErrExcelWorkerUninitialized)
		return customerrs.ErrExcelWorkerUninitialized
	}
	w.workbooksHandle = oleutil.MustGetProperty(w.currentExcelObj, "Workbooks").ToIDispatch()
	common.Logger.Debugln("Workbooks handle requested.")
	return nil
}

func (w *ExcelWorker) OpenWorkbook(fPath string) error {
	if w.workbooksHandle == nil {
		common.Logger.Errorln(customerrs.ErrExcelWorkbooksUnable2Fetch)
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
	common.Logger.Infoln("Workbook currently opened: ", fPath)
	// try to eliminate slow workbook
	_, err = oleutil.PutProperty(w.currentWorkbook, "ForceFullCalculation", false)
	if err != nil {
		telemetry.CaptureException(err, "Excel.ThisWorkbook.SetForceFullCalculationFalse")
		common.Logger.Errorln(err)
	}
	// disable update embedded ole links
	_, err = oleutil.PutProperty(w.currentWorkbook, "UpdateLinks", XlUpdateLinksNever)
	if err != nil {
		telemetry.CaptureException(err, "Excel.Workbook.SetUpdateLinksNever")
		common.Logger.Errorln(err)
	}
	// disable update remote ref in workbook
	_, err = oleutil.PutProperty(w.currentWorkbook, "UpdateRemoteReferences", false)
	if err != nil {
		telemetry.CaptureException(err, "Excel.Workbook.SetUpdateRemoteReferencesFalse")
		common.Logger.Errorln(err)
	}
	// in rare cases, e.g. a ~30M xlsm file may fail to open in 1 min, thus timed out.
	// try to eliminate such cases, these two properties must be set after opening any workbook and
	// will only affect the current instance
	_, err = oleutil.PutProperty(w.currentExcelObj, "Calculation", XlCalculationManual)
	if err != nil {
		telemetry.CaptureException(err, "Excel.Workbook.ParentApp.SetCalculationManual")
		common.Logger.Errorln(err)
	}
	_, err = oleutil.PutProperty(w.currentExcelObj, "CalculateBeforeSave", false)
	if err != nil {
		telemetry.CaptureException(err, "Excel.Workbook.ParentApp.SetCalculateBeforeSaveFalse")
		common.Logger.Errorln(err)
	}
	// also do not prompt for format conversion
	// https://learn.microsoft.com/en-us/dotnet/api/microsoft.office.tools.excel.workbook.donotpromptforconvert
	_, err = oleutil.PutProperty(w.currentWorkbook, "DoNotPromptForConvert", false)
	if err != nil {
		common.Logger.Errorln(err)
	}
	_, err = oleutil.PutProperty(w.currentWorkbook, "CheckCompatibility", false)
	if err != nil {
		common.Logger.Errorln(err)
	}
	common.Logger.Infoln("Workbook slowness workarounds applied.")
	return nil

}

func (w *ExcelWorker) SaveAndCloseWorkbook() error {
	if w.currentWorkbook == nil {
		common.Logger.Errorln(customerrs.ErrExcelCurrentWorkbookNullPtr)
		return customerrs.ErrExcelCurrentWorkbookNullPtr
	}
	_, _ = w.currentWorkbook.CallMethod("Save")
	_, _ = w.currentWorkbook.CallMethod("Close", true)
	w.currentWorkbook.Release()
	common.Logger.Infoln("Workbook save and closed.")
	w.currentWorkbook = nil
	return nil
}

func (w *ExcelWorker) SanitizeWorkbook(destModuleName string) error {
	if w.currentWorkbook == nil {
		common.Logger.Errorln(customerrs.ErrExcelCurrentWorkbookNullPtr)
		return customerrs.ErrExcelCurrentWorkbookNullPtr
	}
	wbHasVBA := oleutil.MustGetProperty(w.currentWorkbook, "HasVBProject").Value().(bool)
	if wbHasVBA {
		common.Logger.Infoln("Workbook has VBProject.")
		wbVbaProjRes, err := oleutil.GetProperty(w.currentWorkbook, "VBProject")
		if err != nil {
			telemetry.CaptureException(err, "Excel.Workbook.VBProject.VBOMAccess")
			return err
		}
		wbVbaProj := wbVbaProjRes.ToIDispatch()
		vbCompsInProj := oleutil.MustGetProperty(wbVbaProj, "VBComponents").ToIDispatch()
		vbCompsCount := (int)(oleutil.MustGetProperty(vbCompsInProj, "Count").Value().(int32))
		common.Logger.Debugln("VBComponents Count: ", vbCompsCount)
		for i := 1; i <= vbCompsCount; i++ {
			// yes, this bullsh*t index starts from 1...
			vbComp := oleutil.MustCallMethod(vbCompsInProj, "Item", i).ToIDispatch()
			vbCompName := oleutil.MustGetProperty(vbComp, "Name").Value().(string)
			common.Logger.Debugln("Current VBComponent Name in iteration: ", vbCompName)
			if vbCompName == destModuleName {
				common.Logger.Infoln("Sanitizing Matched VBA Component: ", vbCompName)
				// verified in powershell
				codeMod := oleutil.MustGetProperty(vbComp, "CodeModule").ToIDispatch()
				codeModLineCnt := (int)(oleutil.MustGetProperty(codeMod, "CountOfLines").Value().(int32))
				// remove all lines
				_, err := codeMod.CallMethod("DeleteLines", 1, codeModLineCnt)
				if err != nil {
					telemetry.CaptureException(err, "Excel.WorkbookVBACodeModule.DeleteLines_"+w.curFilePath)
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
