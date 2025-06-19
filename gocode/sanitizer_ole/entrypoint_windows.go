//go:build windows

package sanitizer_ole

import (
	"context"
	"cramc_go/common"
	"cramc_go/platform/windoge_utils"
	"github.com/getsentry/sentry-go"
	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows/registry"
	"strings"
	"time"
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

	// new approach: bundled
	eWorker := &ExcelWorker{}
	err = eWorker.Init()
	if err != nil {
		sentry.CaptureException(err)
		common.Logger.Errorln("Failed to initialize excel worker:", err)
		return err
	}
	defer eWorker.Quit(false)
	err = eWorker.GetWorkbooks()
	if err != nil {
		sentry.CaptureException(err)
		common.Logger.Errorln("Failed to get workbooks:", err)
		return err
	}
	common.Logger.Infoln("Excel.Application worker initialized.")

	// iterate through workbooks
	for vObj := range common.SanitizeQueue {
		// change path separator, make sure consistent in os-level
		fPathNonVariant := strings.ReplaceAll(vObj.Path, "/", "\\")
		// backup file
		err = gzBakFile(fPathNonVariant)
		if err != nil {
			common.Logger.Errorln("Backup file failed:", err.Error())
		}
		common.Logger.Infoln("Original file backup succeeded.")
		// sleep 1 second to leave space for saving
		time.Sleep(1 * time.Second)
		switch vObj.Action {
		case "sanitize":
			// parse and take action
			func(eWorker *ExcelWorker) {
				// 60 seconds should be sufficient for open and sanitize a single doc
				ctx, cancelF := context.WithTimeout(context.TODO(), 60*time.Second)
				defer cancelF()
				// notice if finished earlier
				doneC := make(chan struct{}, 1)
				go func() {
					// lock to ensure only single doc at a time
					eWorker.Lock()
					// must unlock whatever happened
					defer eWorker.Unlock()
					// open workbook
					err := eWorker.OpenWorkbook(fPathNonVariant)
					if err != nil {
						common.Logger.Errorln("Failed to open workbook:", err)
						sentry.CaptureMessage("Failed Document: " + fPathNonVariant)
						doneC <- struct{}{}
						return
					}
					defer func() {
						err = eWorker.SaveAndCloseWorkbook()
						if err != nil {
							common.Logger.Errorln("Failed to save and close workbook:", err)
						}
						time.Sleep(1 * time.Second)
						// rename file and save to clean state cache of cloud-storage provider
						err = renameFileAndSave(fPathNonVariant)
						if err != nil {
							common.Logger.Errorln("Rename file failed:", err.Error())
						}
					}()
					// sanitize
					err = eWorker.SanitizeWorkbook(vObj.DestModule)
					if err != nil {
						common.Logger.Errorln("Failed to sanitize workbook:", err)
						sentry.CaptureMessage("Failed Document: " + fPathNonVariant)
						doneC <- struct{}{}
						return
					}
					doneC <- struct{}{}
				}()
				select {
				case <-doneC:
					// properly remediated
					// go ahead
					common.Logger.Debugln("Sanitize workbook finished, doneC returned correctly.")
				case <-ctx.Done():
					// timed out or error, send log to sentry
					err := ctx.Err()
					if err != nil {
						sentry.CaptureException(err)
						common.Logger.Errorln("Failed to sanitize workbook, timed out:", err)
					}
					// for GC, cleanup and rebuild excel instance
					eWorker.Quit(true)
					// safely ignore errors as it's already built correctly before
					_ = eWorker.Init()
					_ = eWorker.GetWorkbooks()
				}
				common.Logger.Infoln("Workbook Sanitized: ", fPathNonVariant)
			}(eWorker)
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
