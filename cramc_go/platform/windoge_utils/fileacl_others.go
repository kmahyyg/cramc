//go:build !windows

package windoge_utils

import "cramc_go/customerrs"

func RetrieveACLOfFile(filep string) (any, error) {
	return nil, customerrs.ErrUnsupportedPlatform
}

func RetrieveOwnerOfFile(filep string) (any, error) {
	return nil, customerrs.ErrUnsupportedPlatform
}

func SetACLOfFile(filep string, acl any) error {
	return nil, customerrs.ErrUnsupportedPlatform
}

func SetOwnerOfFile(filep string, owner any) error {
	return nil, customerrs.ErrUnsupportedPlatform
}
