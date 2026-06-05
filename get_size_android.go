//go:build android

package main

import (
	"fmt"

	"fyne.io/fyne/v2"
)

func getSize(uri fyne.URI) (size int64, err error) {
	if uri == nil {
		return 0, fmt.Errorf("uri is nil")
	}

	size, err = callLongString("getSize", uri.String())
	if err != nil {
		return 0, fmt.Errorf("failed to get size: %w", err)
	}
	return size, nil
}
