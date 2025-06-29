//go:build windows

package windoge_utils

import (
	"cramc_go/common"
	"cramc_go/customerrs"
	"golang.org/x/sys/windows"
	"os/user"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	WELLKNOWN_SYSTEM_SID = "S-1-5-18"

	WTS_CURRENT_SERVER_HANDLE = windows.Handle(0)

	SE_TCB_NAME                = "SeTcbPrivilege"
	SE_ASSIGNPRIMARYTOKEN_NAME = "SeAssignPrimaryTokenPrivilege"
	SE_IMPERSONATE_NAME        = "SeImpersonatePrivilege"
)

type LSTATUS uint32

var (
	ERROR_SUCCESS LSTATUS = 0
)

// life saver!
// why do I need this? https://github.com/kmahyyg/cramc/issues/11

var (
	modAdvapi32 = syscall.NewLazyDLL("advapi32.dll")
	// https://learn.microsoft.com/en-us/windows/win32/secauthz/client-impersonation
	procRegDisablePredefinedCache   = modAdvapi32.NewProc("RegDisablePredefinedCache")
	procRegDisablePredefinedCacheEx = modAdvapi32.NewProc("RegDisablePredefinedCacheEx")

	modWtsapi32                    = syscall.NewLazyDLL("wtsapi32.dll")
	procWTSQuerySessionInformation = modWtsapi32.NewProc("WTSQuerySessionInformationA")

	modShlwapi = syscall.NewLazyDLL("shlwapi.dll")
	procIsOS   = modShlwapi.NewProc("IsOS")
)

func CheckRunningUnderSYSTEM() (bool, error) {
	curU, err := user.Current()
	if err != nil {
		common.Logger.Errorln(err)
		return false, err
	}
	if curU.Uid == WELLKNOWN_SYSTEM_SID {
		return true, nil
	}
	return false, nil
}

func getActiveWTSSessionID() (uint32, error) {
	// WTSGetActiveConsoleSessionId may return 0 if runs from SERVICE
	// https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-wtsgetactiveconsolesessionid
	// https://fleexlab.blogspot.com/2015/04/remote-desktop-surprise.html
	// if no session attached, may return 0xFFFFFFFF
	//
	// working way is https://stackoverflow.com/questions/8309043/wtsgetactiveconsolesessionid-returning-system-session
	//
	// session 0 for service is ignored, session 1 maybe console.
	// console may be in session 2+, depends on user logon order.
	// check: https://learn.microsoft.com/en-us/previous-versions/windows/hardware/design/dn653293(v=vs.85)?redirectedfrom=MSDN
	// check: https://www.brianbondy.com/blog/100/understanding-windows-at-a-deeper-level-sessions-window-stations-and-desktops
	//
	// precedence: console session > rdp session
	// active session > connected > disconnected > listening
	// if multiple active sessions are found, the first found session wins
	//
	var sessionCount uint32
	var sessionsInfo *windows.WTS_SESSION_INFO
	err := windows.WTSEnumerateSessions(WTS_CURRENT_SERVER_HANDLE, 0, 1, &sessionsInfo, &sessionCount)
	if err != nil {
		return 0, err
	}
	sessions := unsafe.Slice(sessionsInfo, sessionCount)
	defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(sessionsInfo)))
	var selectedSessionID uint32
	var sessionIDfound bool
	for _, ses := range sessions {
		// skip session ID 0 (service) & ID > 65530 special listening session
		if ses.SessionID == 0 || ses.SessionID > 65530 {
			continue
		}
		// check console session,
		sesName := windows.UTF16PtrToString(ses.WindowStationName)
		// active console first
		if sesName == "Console" && ses.State == windows.WTSActive {
			selectedSessionID = ses.SessionID
			sessionIDfound = true
			break
		}
		// if not, find the first active session
		// https://learn.microsoft.com/en-us/windows/win32/api/wtsapi32/ne-wtsapi32-wts_connectstate_class
		// WtsConnected may not indicate logged-in.
		if ses.State == windows.WTSActive {
			selectedSessionID = ses.SessionID
			sessionIDfound = true
			break
		}
		// if still not found, find the first disconnected session
		// determine if server edition first, if yes, result below is unreliable
		if isWindowsServer() {
			common.Logger.Warnln("[WARN] WTSActiveSession not found and running on server OS.")
		}
		// as MSFT stated: The WinStation is active but the client is disconnected. This state occurs when a user is signed in but not actively connected to the device, such as when the user has chosen to exit to the lock screen.
		// https://stackoverflow.com/questions/12063873/trying-to-interpret-user-session-states-on-windows-os
		var userN *uint16
		var userNsize uint32
		if ses.State == windows.WTSDisconnected {
			// for precaution, check session linked username, it should not be null
			err = wtsQuerySessionInformation(WTS_CURRENT_SERVER_HANDLE, ses.SessionID, WTSUserName, &userN, &userNsize)
			if err != nil {
				common.Logger.Errorln(err)
			}
			defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(userN)))
			curUserName := windows.UTF16PtrToString(userN)
			if len(curUserName) != 0 {
				selectedSessionID = ses.SessionID
				sessionIDfound = true
				break
			} else {
				continue
			}
		}
		// I don't want to fallback to Connected Session, as user may not be logged-in.
		// which I can't use for further token query and impersonation.
	}
	// if no active session found, means user logged-out or already disconnected.
	if sessionIDfound {
		common.Logger.Infof("Active WTS Session ID Found: %d", selectedSessionID)
		return selectedSessionID, nil
	} else {
		return 0, customerrs.ErrUnknownInternalError
	}
}

func ImpersonateCurrentInteractiveUserInThread() (uintptr, error) {
	// to ensure cross-platform compatibility, returned value should be Windows.Token, use uintptr to prevent further issue
	// get sessionID from getActiveWTSSessionID()
	sessID, err := getActiveWTSSessionID()
	if err != nil {
		common.Logger.Errorln("Cannot determine current active session, abort with error: ", err)
		return 0, err
	}
	if sessID == 0 {
		common.Logger.Errorln("Unexpected Session ID returned, must fail.")
		return 0, customerrs.ErrUnknownInternalError
	}
	// now query logged-in session
	var sessUserToken windows.Token
	err = windows.WTSQueryUserToken(sessID, &sessUserToken)
	if err != nil {
		common.Logger.Errorln("Cannot retrieve primary user token from given session ID: ", sessID, "with Error: ", err)
		return 0, err
	}
	defer sessUserToken.Close()
	// enable priv
	err = enableNecessaryPrivilege()
	if err != nil {
		common.Logger.Errorln("Cannot enable necessary privilege: ", err)
		return 0, err
	}
	// token retrieved, thread locked, now time to duplicate a primary token as an impersonation token,
	// this impersonation token cannot be used to CreateProcessAsUser and should be freed after use.
	var impUserToken windows.Token
	err = windows.DuplicateTokenEx(sessUserToken, windows.TOKEN_ALL_ACCESS, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &impUserToken)
	if err != nil {
		common.Logger.Errorln("Cannot duplicate token from given session ID: ", sessID, "with Error: ", err)
		return 0, err
	}
	// set to impersonation token in the current thread
	err = windows.SetThreadToken(nil, impUserToken)
	if err != nil {
		return (uintptr)(impUserToken), err
	}
	return (uintptr)(impUserToken), nil
}

func PrepareForTokenImpersonation(isReverse bool) error {
	if isReverse {
		err := windows.RevertToSelf()
		if err != nil {
			common.Logger.Errorln(err)
		}
		runtime.UnlockOSThread()
		return err
	}
	// must lock and bind to a single specialized os thread before continue
	runtime.LockOSThread()
	// disable registry cache for precaution, if called from service context, it's recommended by MSFT
	err := regDisablePredefinedCache()
	if err != nil {
		// safe to ignore and go next
		common.Logger.Warnln("RegDisablePredefinedCacheEx returned err: ", err.Error())
		return err
	}
	return nil
}

func enableNecessaryPrivilege() error {
	curProcHnd := windows.CurrentProcess()
	var curProcToken windows.Token
	err := windows.OpenProcessToken(curProcHnd, windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &curProcToken)
	if err != nil {
		return err
	}
	defer curProcToken.Close()
	// lookupLuID
	curSys := uint16(0)
	checkLUIDUnderPrivAndAdjust := func(privName string, tkn windows.Token) error {
		var privLUID windows.LUID
		namePtr, err2 := windows.UTF16PtrFromString(privName)
		if err2 != nil {
			return err2
		}
		err2 = windows.LookupPrivilegeValue(&curSys, namePtr, &privLUID)
		if err2 != nil {
			return err2
		}
		tknPriv := &windows.Tokenprivileges{
			PrivilegeCount: 1,
			Privileges: [1]windows.LUIDAndAttributes{
				{
					Luid:       privLUID,
					Attributes: windows.SE_PRIVILEGE_ENABLED,
				},
			},
		}
		err2 = windows.AdjustTokenPrivileges(tkn, false, tknPriv, 0, nil, nil)
		if err2 != nil {
			return err2
		}
		return nil
	}

	for _, priv := range []string{SE_TCB_NAME, SE_ASSIGNPRIMARYTOKEN_NAME, SE_IMPERSONATE_NAME} {
		err3 := checkLUIDUnderPrivAndAdjust(priv, curProcToken)
		if err3 != nil {
			common.Logger.Errorln("Adjusting Token Privilege Error: ", err3, " When processing:", priv)
		}
	}
	return nil
}

// -------- Windows Internal System API definition -------- //
// for func (*Proc) Call:
// The returned error is always non-nil, constructed from the result of GetLastError.
// Callers must inspect the primary return value to decide whether an error occurred (according to the semantics
// of the specific function being called) before consulting the error. The error always has type Errno.

// regDisablePredefinedCache from MSDN
// https://learn.microsoft.com/en-us/windows/win32/api/winreg/nf-winreg-regdisablepredefinedcache
// definition: LSTATUS RegDisablePredefinedCache();
func regDisablePredefinedCache() error {
	ret, _, err := procRegDisablePredefinedCache.Call()
	if LSTATUS(ret) != ERROR_SUCCESS {
		common.Logger.Errorf("RegDisablePredefinedCache failed, with error code: %d", ret)
		return err
	}
	return nil
}

// regDisablePredefinedCacheEx from MSDN
// https://learn.microsoft.com/en-us/windows/win32/api/winreg/nf-winreg-regdisablepredefinedcacheex
// definition: LSTATUS RegDisablePredefinedCacheEx();
func regDisablePredefinedCacheEx() error {
	ret, _, _ := procRegDisablePredefinedCacheEx.Call()
	if LSTATUS(ret) != ERROR_SUCCESS {
		common.Logger.Errorf("RegDisablePredefinedCache failed, with error code: %d", ret)
		return syscall.Errno(ret)
	}
	return nil
}

type WtsInfoClass uint8

const (
	WTSUserName WtsInfoClass = 5
)

// wtsQuerySessionInformation (ANSI version):
// from MSDN: https://learn.microsoft.com/en-us/windows/win32/api/wtsapi32/nf-wtsapi32-wtsquerysessioninformationa
// https://learn.microsoft.com/en-us/windows/win32/api/wtsapi32/ne-wtsapi32-wts_info_class
func wtsQuerySessionInformation(hServer windows.Handle, sessionID uint32,
	klass WtsInfoClass, ppBuffer **uint16, pBytesReturned *uint32) error {
	ret, _, err := syscall.SyscallN(procWTSQuerySessionInformation.Addr(), 5, uintptr(hServer), uintptr(sessionID), uintptr(klass), uintptr(unsafe.Pointer(ppBuffer)), uintptr(unsafe.Pointer(pBytesReturned)))
	if ret == 0 {
		return err
	}
	return nil
}

func isWindowsServer() bool {
	var OS_ANYSERVER uint32 = 29
	ret, _, _ := syscall.SyscallN(procIsOS.Addr(), 1, uintptr(OS_ANYSERVER))
	return ret != 0
}
