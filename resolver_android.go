//go:build android

package main

import (
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

func CanList(u fyne.URI) (bool, error) {
	if u == nil {
		return false, nil
	}

	if apiLevel() > 28 {
		return storage.CanList(u)
	}

	return callBooleanString("canListDirectory", u.String())
}

func uriBase(uri fyne.URI) string {
	name, err := callStringString("getFileName", uri.String())
	if err == nil && name != "" {
		return name
	}
	return base(uri.Path())
}

func base(path string) string {
	decoded, err := url.PathUnescape(path)
	if err != nil {
		decoded = strings.ReplaceAll(path, "%2F", "/")
	}

	lastSlash := strings.LastIndex(decoded, "/")
	if lastSlash < 0 {
		return replace(decoded)
	}

	return replace(decoded[lastSlash+1:])
}

func replace(s string) string {
	return strings.NewReplacer(
		"?", "_",
		":", "_",
	).Replace(s)
}
