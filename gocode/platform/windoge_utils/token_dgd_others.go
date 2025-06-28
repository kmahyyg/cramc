//go:build !windows

package windoge_utils

import "cramc_go/customerrs"

func CheckRunningUnderSYSTEM() (bool, error) {
	return false, customerrs.ErrUnsupportedPlatform
}

func GetActiveConsoleSessionId() (int, error) {
	return -1, customerrs.ErrUnsupportedPlatform
}

func ImpersonateCurrentInteractiveUserInThread() error {
	return customerrs.ErrUnsupportedPlatform
}

func PrepareForTokenOperation() error {
	return customerrs.ErrUnsupportedPlatform
}
