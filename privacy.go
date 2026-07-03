package main

import (
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const privacyAcceptedKey = "privacy-accepted"

// showPrivacyConsentOnStart shows the privacy policy consent dialog once,
// after the application event loop has started (it must run on the main
// thread and after the window is shown, so OnStarted is used).
func showPrivacyConsentOnStart(a fyne.App, w fyne.Window) {
	if a.Preferences().Bool(privacyAcceptedKey) {
		return
	}

	a.Lifecycle().SetOnStarted(func() {
		if a.Preferences().Bool(privacyAcceptedKey) {
			return
		}
		showPrivacyConsent(a, w, func(accepted bool) {
			if !accepted {
				cleanup(w)
				os.Exit(0)
			}
			if privacyCheckSync != nil {
				privacyCheckSync()
			}
		})
	})
}

// policyURL returns the privacy policy URL with the requested language code
// appended as a query parameter (?lang=<code>) so the page shows that language.
func policyURL(code string) *url.URL {
	u, err := url.Parse(PrivacyPolicyURL)
	if err != nil {
		return nil
	}
	q := u.Query()
	q.Set("lang", code)
	u.RawQuery = q.Encode()
	return u
}

// showPrivacyConsent shows the multilingual privacy policy consent dialog.
// onResult is called exactly once with the user decision: true for Accept,
// false for Decline (or dismissing the dialog without a choice). The dialog
// does not exit/reset on its own — the caller decides what to do with it.
func showPrivacyConsent(a fyne.App, w fyne.Window, onResult func(bool)) {
	centered := func(o fyne.CanvasObject) fyne.CanvasObject {
		return container.New(layout.NewCenterLayout(), o)
	}
	var d dialog.Dialog
	var once sync.Once
	respond := func(accepted bool) {
		once.Do(func() { onResult(accepted) })
	}

	accept := widget.NewButtonWithIcon("", theme.ConfirmIcon(), func() {
		a.Preferences().SetBool(privacyAcceptedKey, true)
		respond(true)
		d.Hide()
	})
	decline := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		respond(false)
		d.Hide()
	})

	content := container.New(newTightVBox(),
		centered(widget.NewHyperlink("Privacy Policy", policyURL("en-US"))),
		centered(widget.NewHyperlink("Gizlilik Politikası", policyURL("tr-TR"))),
		centered(widget.NewHyperlink("プライバシーポリシー", policyURL("ja-JP"))),
		centered(widget.NewHyperlink("隐私政策", policyURL("zh-CN"))),
		centered(widget.NewHyperlink("Политика конфиденциальности", policyURL("ru-RU"))),
		centered(widget.NewLabel("")),
		centered(container.NewHBox(accept, decline)),
	)

	d = dialog.NewCustomWithoutButtons(lp("Accept"), content, w)

	d.SetOnClosed(func() {
		// Dismissed without a button (e.g. back gesture) → treat as decline.
		// no-op if the user already responded (sync.Once guard).
		respond(false)
	})

	d.Show()
}

// revokeConsent erases all application preferences by removing the Fyne
// preferences file and exits the app. Called when the user revokes consent
// via the About checkbox. Next launch will show the first-launch consent
// dialog again.
func revokeConsent(w fyne.Window) {
	cleanup(w)
	_ = os.Remove(filepath.Join(tempDir, "preferences.json"))
	os.Exit(0)
}
