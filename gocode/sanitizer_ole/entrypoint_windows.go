//go:build windows

package sanitizer_ole

import (
	"context"
	"cramc_go/common"
	"cramc_go/platform/windoge_utils"
	"cramc_go/telemetry"
	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	modOle32                 = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeSecurity = modOle32.NewProc("CoInitializeSecurity")
	nullptr                  = uintptr(0)
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
	err := LiftVBAScriptingAccess("16.0", "Excel")
	if err != nil {
		return err
	}
	// kill all office processes, to avoid any potential file lock.
	_, _ = windoge_utils.KillAllOfficeProcesses()
	common.Logger.Infoln("Triggered M365 Office processes killer.")

	// check if run as system
	runAsSystem, _ := windoge_utils.CheckRunningUnderSYSTEM()
	if runAsSystem {
		_ = windoge_utils.PrepareForTokenImpersonation(false) // os-thread locked
		defer windoge_utils.PrepareForTokenImpersonation(true)
	}

	// prepare to call ole
	err = ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err != nil {
		return err
	}
	defer ole.CoUninitialize()

	// try to impersonate while calling COM API
	// if anything wrong related to SYSTEM impersonation, you may try:
	// https://learn.microsoft.com/en-us/windows/win32/com/setting-processwide-security-with-coinitializesecurity
	// calling: HRESULT CoInitializeSecurity(NULL, -1, NULL, NULL, RPC_C_AUTHN_LEVEL_NONE, RPC_C_IMP_LEVEL_IMPERSONATE,
	// 											NULL, EOAC_NONE, NULL);
	//
	if runAsSystem {
		//
		// no effect on final result
		//cAuthSvc := (int32)(-1)
		//err = CoInitializeSecurity(0, &cAuthSvc, nil, 0, RPC_C_AUTHN_LEVEL_NONE, RPC_C_IMP_LEVEL_IMPERSONATE, nil, EOAC_NONE, 0)
		//if err != nil {
		//	common.Logger.Errorln("Failed to call CoInitializeSecurity:", err)
		//}
		//
		// after handling COM API security, call impersonation on current thread.
		// for now, as prepare for token impersonation is already called, it should be bounded to a single OS thread
		// so we shouldn't make ourselves mess around.
		impToken, err2 := windoge_utils.ImpersonateCurrentInteractiveUserInThread()
		if impToken != 0 {
			defer (windows.Token)(impToken).Close()
			common.Logger.Infoln("Current interactive user impersonation token created.")
		}
		if err2 != nil {
			common.Logger.Errorln("Failed to impersonate current interactive user:", err2)
			return err2
		}
		common.Logger.Infoln("Current interactive user impersonated.")
	}

	// new approach: bundled
	inDebugging := false
	if data, ok := os.LookupEnv("RunEnv"); ok {
		if data == "DEBUG" {
			inDebugging = true
		}
	}
	eWorker := &ExcelWorker{}
	err = eWorker.Init(inDebugging)
	if err != nil {
		common.Logger.Errorln("Failed to initialize excel worker:", err)
		return err
	}
	defer eWorker.Quit(false)
	err = eWorker.GetWorkbooks()
	if err != nil {
		common.Logger.Errorln("Failed to get workbooks:", err)
		return err
	}
	common.Logger.Infoln("Excel.Application worker initialized.")

	wg2 := &sync.WaitGroup{}
	// iterate through workbooks
	for vObj := range common.SanitizeQueue {
		common.Logger.Debugln("Sanitizer Queue Received a New File.")
		// change path separator, make sure consistent in os-level
		fPathNonVariant, err2 := filepath.Abs(vObj.Path)
		if err2 != nil {
			common.Logger.Errorln("Failed to get absolute path:", err2)
			continue
		}
		// backup file
		err = gzBakFile(fPathNonVariant)
		if err != nil {
			common.Logger.Errorln("Backup file failed:", err.Error())
		}
		common.Logger.Infoln("Original file backup succeeded: ", vObj.Path)
		// sleep 1 second to leave space for saving
		time.Sleep(1 * time.Second)
		switch vObj.Action {
		case "sanitize":
			// parse and take action
			func(eWorker *ExcelWorker) {
				// 60 seconds should be sufficient for opening and sanitizing a single normal doc
				//
				// unfortunately, in some rare cases, it cost around 109 seconds for open.
				// in case of such a sucking document, have to change timeout to 180s
				ctx, cancelF := context.WithTimeout(context.TODO(), 180*time.Second)
				defer cancelF()
				// notice if finished earlier
				doneC := make(chan struct{}, 1)
				wg2.Add(1)
				common.Logger.Debugln("Sanitize workbook started, wg2 += 1")
				go func() {
					defer wg2.Done()
					// lock to ensure only single doc at a time
					eWorker.Lock()
					// must unlock whatever happened
					defer eWorker.Unlock()
					// open workbook
					common.Logger.Infoln("Opening workbook in sanitizer: ", fPathNonVariant)
					err3 := eWorker.OpenWorkbook(fPathNonVariant)
					if err3 != nil {
						common.Logger.Errorln("Failed to open workbook in sanitizer:", err3)
						doneC <- struct{}{}
						return
					}
					common.Logger.Debugln("Workbook opened: ", fPathNonVariant)
					defer func() {
						err4 := eWorker.SaveAndCloseWorkbook()
						if err != nil {
							common.Logger.Errorln("Failed to save and close workbook in defer Sanitizer:", err4)
						}
						time.Sleep(1 * time.Second)
						// rename file and save to clean state cache of cloud-storage provider
						err4 = renameFileAndSave(fPathNonVariant)
						if err4 != nil {
							common.Logger.Errorln("Rename file failed in sanitizer:", err4.Error())
						}
						common.Logger.Infoln("Workbook Sanitized: ", fPathNonVariant)
					}()
					// sanitize
					common.Logger.Debugln("Sanitize Workbook VBA Module now.")
					err3 = eWorker.SanitizeWorkbook(vObj.DestModule)
					if err3 != nil {
						common.Logger.Errorln("Failed to sanitize workbook:", err3)
						doneC <- struct{}{}
						return
					}
					common.Logger.Debugln("Sanitize Workbook VBA Module finished, doneC returned.")
					doneC <- struct{}{}
				}()
				select {
				case <-doneC:
					// properly remediated
					// go ahead
					common.Logger.Debugln("Sanitize workbook finished, doneC returned correctly.")
					return
				case <-ctx.Done():
					// timed out or error
					err5 := ctx.Err()
					if err5 != nil {
						telemetry.CaptureException(err5, "SanitizeWorkbookTimedOut")
						common.Logger.Errorln("Failed to sanitize workbook, timed out:", err5)
					}
					common.Logger.Infoln("Sanitize workbook timed out, ctx.Done() returned, go to force clean.")
					// for GC, cleanup and rebuild excel instance
					originalDbgStatus := eWorker.inDbg
					eWorker.Quit(true)
					// safely ignore errors as it's already built correctly before
					_ = eWorker.Init(originalDbgStatus)
					_ = eWorker.GetWorkbooks()
				}
			}(eWorker)
		default:
			common.Logger.Warnln("Unsupported action type: ", vObj.Action)
			continue
		}
	}
	wg2.Wait()
	common.Logger.Infoln("Sanitizer Finished.")
	return nil
}

func LiftVBAScriptingAccess(versionStr string, componentStr string) error {
	// this fix COM API via OLE returned null on VBProject access
	regK, openedExists, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Office\`+versionStr+`\`+componentStr+`\Security`, registry.ALL_ACCESS)
	if err != nil {
		common.Logger.Errorln("Failed to create registry key to lift VBOM restriction:", err)
		return err
	}
	if openedExists {
		common.Logger.Debugln("Registry key already exists, opened existing one.")
	}
	common.Logger.Debugln("Registry key Opened.")
	defer regK.Close()
	err = regK.SetDWordValue("AccessVBOM", (uint32)(1))
	if err != nil {
		common.Logger.Errorln("Failed to set registry value to lift VBOM restriction:", err)
		return err
	}
	common.Logger.Infoln("Registry value set to 1 for AccessVBOM.")
	return nil
}

// -------- Excel COM API Call Constants Enum -------- //

type MsoAutomationSecurity int

const (
	MsoAutomationSecurityLow = 1 + iota
	MsoAutomationSecurityByUI
	MsoAutomationSecurityForceDisable
)

type XlCalculation int

const (
	XlCalculationManual        = -4135
	XlCalculationAutomatic     = -4105
	XlCalculationSemiautomatic = 2
)

type XlUpdateLinks int

const (
	XlUpdateLinksUserSetting = 1 + iota
	XlUpdateLinksNever
	XlUpdateLinksAlways
)

// -------- Windows COM API Impersonation Enum -------- //

type RpcCallAuthenticationLevel uint32

const (
	RPC_C_AUTHN_LEVEL_DEFAULT RpcCallAuthenticationLevel = iota
	RPC_C_AUTHN_LEVEL_NONE
	RPC_C_AUTHN_LEVEL_CONNECT
	RPC_C_AUTHN_LEVEL_CALL
	RPC_C_AUTHN_LEVEL_PKT
	RPC_C_AUTHN_LEVEL_PKT_INTEGRITY
	RPC_C_AUTHN_LEVEL_PKT_PRIVACY
)

type RpcCallImpersonationLevel uint32

const (
	RPC_C_IMP_LEVEL_DEFAULT RpcCallImpersonationLevel = iota
	RPC_C_IMP_LEVEL_ANONYMOUS
	RPC_C_IMP_LEVEL_IDENTIFY
	RPC_C_IMP_LEVEL_IMPERSONATE
	RPC_C_IMP_LEVEL_DELEGATE
)

// https://learn.microsoft.com/en-us/windows/win32/api/objidl/ne-objidl-eole_authentication_capabilities
type EOLEAuthenticationCapabilities uint32

const (
	EOAC_NONE              EOLEAuthenticationCapabilities = 0
	EOAC_MUTUAL_AUTH                                      = 1
	EOAC_STATIC_CLOAKING                                  = 0x20
	EOAC_DYNAMIC_CLOAKING                                 = 0x40
	EOAC_ANY_AUTHORITY                                    = 0x80
	EOAC_MAKE_FULLSIC                                     = 0x100
	EOAC_DEFAULT                                          = 0x800
	EOAC_SECURE_REFS                                      = 0x2
	EOAC_ACCESS_CONTROL                                   = 0x4
	EOAC_APPID                                            = 0x8
	EOAC_DYNAMIC                                          = 0x10
	EOAC_REQUIRE_FULLSIC                                  = 0x200
	EOAC_AUTO_IMPERSONATE                                 = 0x400
	EOAC_DISABLE_AAA                                      = 0x1000
	EOAC_NO_CUSTOM_MARSHAL                                = 0x2000
	EOAC_RESERVED1                                        = 0x4000
)

type HRESULT int32

// SOLEAuthenticationService: https://learn.microsoft.com/en-us/windows/win32/api/objidl/ns-objidl-sole_authentication_service
type SOLEAuthenticationService struct {
	DwAuthnSvc     uint32
	DwAuthzSvc     uint32
	PPrincipalName *uint16 //OLECHAR* (utf-16 string)
	Hr             HRESULT
}

type SOLEAuthenticationInfo struct {
	DwAuthnSvc uint32
	DwAuthzSvc uint32
	PAuthInfo  uintptr
}

// CoInitializeSecurity from MSDN
// https://learn.microsoft.com/en-us/windows/win32/api/combaseapi/nf-combaseapi-coinitializesecurity
// why I need this? https://learn.microsoft.com/en-us/windows/win32/com/setting-processwide-security-with-coinitializesecurity
//
// Data Type: https://learn.microsoft.com/en-us/cpp/cpp/data-type-ranges?view=msvc-170
// https://learn.microsoft.com/en-us/windows/win32/winprog/windows-data-types
// https://learn.microsoft.com/en-us/windows/win32/seccrypto/common-hresult-values
//
// HRESULT CoInitializeSecurity(
//
//	[in, optional] PSECURITY_DESCRIPTOR        pSecDesc,  // PVOID
//	[in]           LONG                        cAuthSvc,  // uint32, count of asAuthSvc
//	[in, optional] SOLE_AUTHENTICATION_SERVICE *asAuthSvc,  // struct
//	[in, optional] void                        *pReserved1,
//	[in]           DWORD                       dwAuthnLevel,
//	[in]           DWORD                       dwImpLevel,
//	[in, optional] void                        *pAuthList,  // a pointer to SOLE_AUTHENTICATION_LIST, is an array of SOLEAuthenticationInfo
//	[in]           DWORD                       dwCapabilities,
//	[in, optional] void                        *pReserved3
//
// );
func CoInitializeSecurity(psecDesc uintptr, cAuthsvc *int32, asAuthSvc *SOLEAuthenticationService,
	pReserved1 uintptr, dwAuthnLevel RpcCallAuthenticationLevel, dwImpLevel RpcCallImpersonationLevel,
	pAuthList **SOLEAuthenticationInfo, dwCapabilities EOLEAuthenticationCapabilities,
	pReserved3 uintptr) error {
	ret, _, _ := syscall.SyscallN(procCoInitializeSecurity.Addr(), 9,
		psecDesc, uintptr(unsafe.Pointer(cAuthsvc)), uintptr(unsafe.Pointer(asAuthSvc)),
		nullptr, uintptr(dwAuthnLevel), uintptr(dwImpLevel), uintptr(unsafe.Pointer(pAuthList)),
		uintptr(dwCapabilities), nullptr)
	if uint32(ret) == 0 {
		// S_OK, Operation Successful == 0
		return nil
	}
	return syscall.Errno(ret)
}
