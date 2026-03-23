//go:build windows

package windoge_utils

import (
	"cramc_go/common"
	"cramc_go/customerrs"
	"unsafe"

	"golang.org/x/sys/windows"
)

func RetrieveACLOfFile(filep string) (any, error) {
	sddl, err := windows.GetNamedSecurityInfo(filep, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return nil, err
	}
	dacl, _, err := sddl.DACL()
	if err != nil {
		return nil, err
	}
	return dacl, nil
}

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
	return ownerSID.Copy()
}

func SetACLOfFile(filep string, acl any) error {
	var daclSddl = acl.(*windows.ACL)
	err := windows.SetNamedSecurityInfo(filep, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, daclSddl, nil)
	if err != nil {
		return err
	}
	return nil
}

func SetOwnerOfFile(filep string, owner any) (err error) {
	err = enableTakeOwnershipPrivilege()
	if err != nil {
		return err
	}
	common.Logger.Info("SeTakeOwnershipPrivilege enabled.")
	var ownerSID = owner.(*windows.SID)
	err = windows.SetNamedSecurityInfo(filep, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, ownerSID, nil, nil, nil)
	if err != nil {
		return err
	}
	return nil
}

func enableTakeOwnershipPrivilege() (err error) {
	var curTkn windows.Token
	err = windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &curTkn)
	if err != nil {
		return err
	}
	if curTkn.IsElevated() {
		var luid windows.LUID
		err = windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr("SeTakeOwnershipPrivilege"), &luid)
		if err != nil {
			return err
		}
		var newTknPrivs windows.Tokenprivileges
		newTknPrivs.PrivilegeCount = 1
		newTknPrivs.Privileges[0] = windows.LUIDAndAttributes{
			Luid:       luid,
			Attributes: windows.SE_PRIVILEGE_ENABLED,
		}
		err = windows.AdjustTokenPrivileges(curTkn, false, &newTknPrivs, uint32(unsafe.Sizeof(newTknPrivs)), nil, nil)
		if err != nil {
			return err
		}
	} else {
		return customerrs.ErrInsufficientPrivilege
	}
	return nil
}
