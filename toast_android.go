//go:build android

package main

import (
	"time"

	"fyne.io/fyne/v2"
)

type Toast struct {
	message string
}

const (
	ToastShort        = 3 * time.Second
	ToastLong         = 4 * time.Second
	DefaultPadding    = 10.0
	AnimationDuration = 300 * time.Millisecond
)

func NewToast(_ fyne.Window, message string) *Toast {
	return &Toast{message: message}
}

func (t *Toast) SetIcon(_ fyne.Resource) *Toast       { return t }
func (t *Toast) SetText(message string) *Toast         { t.message = message; return t }
func (t *Toast) SetTimeout(_ time.Duration) *Toast     { return t }
func (t *Toast) SetPadding(_ float32) *Toast           { return t }
func (t *Toast) SetAnimation(_ bool) *Toast            { return t }
func (t *Toast) Short() *Toast                         { return t }
func (t *Toast) Long() *Toast                          { return t }
func (t *Toast) Hide()                                 {}

func (t *Toast) Show() {
	callVoidString("showToast", t.message)
}
