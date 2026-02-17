//go:build !windows

package handy

import "errors"

func newWinListener() (winListener, error) {
	return nil, errors.New("handy: windows listener unavailable on this platform")
}
