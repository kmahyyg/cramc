//go:build !windows

package sanitizer_ole

import "cramc_go/customerrs"

func StartSanitizer() error {
	return customerrs.ErrUnsupportedPlatform
}
