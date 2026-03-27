//go:build windows

package windoge_utils

import (
	"cramc_go/common"
	"cramc_go/customerrs"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var privOnce = &sync.Once{}

func RetrieveOwnerOfFile(filep string) (any, error) {
	// return windows file object SDDL
	sddl, err := windows.GetNamedSecurityInfo(filep, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return nil, err
	}
	ownerSID, _, err := sddl.Owner()
	if err != nil {
		return nil, err
	}
	// return *windows.SID, err
	copiedOwner, err := ownerSID.Copy()
	if err != nil {
		return nil, err
	}
	return copiedOwner, nil
}

func SetOwnerOfFile(filep string, owner any) (err error) {
	err = enableTakeOwnershipPrivilege()
	if err != nil {
		return err
	}
	common.Logger.Info("required privilege for taking ownership enabled.")
	var ownerSID = owner.(*windows.SID)
	err = windows.SetNamedSecurityInfo(filep, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, ownerSID, nil, nil, nil)
	if err != nil {
		return err
	}
	return nil
}

func enableTakeOwnershipPrivilege() (err error) {
	var requiredPrivileges = []string{"SeTakeOwnershipPrivilege", "SeRestorePrivilege", "SeSecurityPrivilege"}
	var lastErr error
	privOnce.Do(func() {
		for _, priv := range requiredPrivileges {
			err = enableSinglePrivilege(priv)
			if err != nil {
				lastErr = err
				common.Logger.Error("Failed to enable privilege: " + priv + ", error: " + err.Error())
				return
			} else {
				common.Logger.Info("Privilege enabled: " + priv)
			}
		}
	})
	return lastErr
}

func enableSinglePrivilege(privilegeName string) (err error) {
	var curTkn windows.Token
	err = windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &curTkn)
	if err != nil {
		return err
	}
	if curTkn.IsElevated() {
		var luid windows.LUID
		err = windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr(privilegeName), &luid)
		if err != nil {
			return err
		}
		var ntkn windows.Tokenprivileges
		ntkn.PrivilegeCount = 1
		ntkn.Privileges[0] = windows.LUIDAndAttributes{
			Luid:       luid,
			Attributes: windows.SE_PRIVILEGE_ENABLED,
		}
		err = windows.AdjustTokenPrivileges(curTkn, false, &ntkn, uint32(unsafe.Sizeof(ntkn)), nil, nil)
		if err != nil {
			return err
		}
		return nil
	} else {
		return customerrs.ErrInsufficientPrivilege
	}
}
