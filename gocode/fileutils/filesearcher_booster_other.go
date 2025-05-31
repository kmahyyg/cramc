//go:build !windows

package fileutils

import "cramc_go/customerrs"

func CheckProcessElevated() (bool, error) {
	return false, customerrs.ErrUnsupportedPlatform
}

func IsDriveFileSystemNTFS() (bool, error) {
	return false, customerrs.ErrUnsupportedPlatform
}

func ExtractAndParseMFT(actionPath []string, allowedExts []string, outputChan chan string) (bool, error) {
	return false, customerrs.ErrUnsupportedPlatform
}

// generally, malicious office macro won't infect non-Windows system.
// thus, no modification could be detected and no hardening measure could be applied.
// on non-Windows system, it is expected to function only for scanner and sanitizer component.
// no privilege check will be performed.
