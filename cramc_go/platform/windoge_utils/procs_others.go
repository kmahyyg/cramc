//go:build !windows

package windoge_utils

import "cramc_go/customerrs"

func CheckProcessElevated() (bool, error) {
	return false, customerrs.ErrUnsupportedPlatform
}
