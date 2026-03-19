//go:build !windows

package windoge_utils

import "cramc_go/customerrs"

func CheckRunningUnderSYSTEM() (bool, error) {
	return false, customerrs.ErrUnsupportedPlatform
}

func GetLoggedInUserToken(tokenType uint32) (uintptr, error) {
	return 0, customerrs.ErrUnsupportedPlatform
}

func PrepareForTokenImpersonation() error {
	return customerrs.ErrUnsupportedPlatform
}
