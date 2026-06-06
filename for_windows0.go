//go:build !windows

package main

import "net/url"

func netUse(_ *url.URL, _ bool) error { return nil }
