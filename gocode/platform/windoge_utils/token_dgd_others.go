//go:build !windows

package windoge_utils

import "cramc_go/customerrs"

func CheckRunningUnderSYSTEM() (bool, error) {
	return false, customerrs.ErrUnsupportedPlatform
}

func ImpersonateCurrentInteractiveUserInThread(sessionID uint32) error {
	return customerrs.ErrUnsupportedPlatform
}

func PrepareForTokenImpersonation() error {
	return customerrs.ErrUnsupportedPlatform
}
