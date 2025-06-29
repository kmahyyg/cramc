//go:build windows

package windoge_utils

import (
	"cramc_go/common"
	"golang.org/x/sys/windows"
	"os/user"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	WELLKNOWN_SYSTEM_SID = "S-1-5-18"

	WTS_CURRENT_SERVER_HANDLE windows.Handle = 0
	NULLPTR                                  = windows.Handle(0)
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
		// if still not found, try first disconnected session with non-null username
		// it's failover, do not rely on this

	}
}

func ImpersonateCurrentInteractiveUserInThread(sessionID uint32) error {

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

func wtsQuerySessionInformation(hServer windows.Handle, sessionID uint32, wtsInfoClass uint32,
	ppBuffer *uint16, pBytesReturned uint32) bool {

}
