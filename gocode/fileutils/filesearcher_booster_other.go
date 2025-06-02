//go:build !windows

package fileutils

import (
	"cramc_go/customerrs"
	"sync"
)

func CheckProcessElevated() (bool, error) {
	return false, customerrs.ErrUnsupportedPlatform
}

func IsDriveFileSystemNTFS(actionPath string) (bool, error) {
	return false, customerrs.ErrUnsupportedPlatform
}

func ExtractAndParseMFTThenSearch(actionPath string, allowedExts []string, outputChan chan string, wg *sync.WaitGroup) (int, error) {
	defer wg.Done()
	defer close(outputChan)
	return -1, customerrs.ErrUnsupportedPlatform
}

// generally, malicious office macro won't infect non-Windows system.
// thus, no modification could be detected and no hardening measure could be applied.
// on non-Windows system, it is expected to function only for scanner and sanitizer component.
// no privilege check will be performed.
