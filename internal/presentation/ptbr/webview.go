package ptbr

// WebViewTexts are what Windows shows when the WebView2 runtime the window
// needs is missing or fails. DownloadPage is followed by the minimum version.
type WebViewTexts struct {
	InstallationRequired, UpdateRequired, MissingRequirements, NotInstalled string
	Error, FailedToInstall, DownloadPage, PressOKToInstall, ContactAdmin    string
	InvalidFixed, Crashed                                                   string
}

// WebViewMessages words the WebView2 messages in the interface's language.
func WebViewMessages() WebViewTexts {
	return WebViewTexts{
		InstallationRequired: T("webview.installationRequired"),
		UpdateRequired:       T("webview.updateRequired"),
		MissingRequirements:  T("webview.missingRequirements"),
		NotInstalled:         T("webview.notInstalled"),
		Error:                T("webview.error"),
		FailedToInstall:      T("webview.failedToInstall"),
		DownloadPage:         T("webview.downloadPage") + " ",
		PressOKToInstall:     T("webview.pressOK"),
		ContactAdmin:         T("webview.contactAdmin"),
		InvalidFixed:         T("webview.invalidFixed"),
		Crashed:              T("webview.crashed"),
	}
}

// StartupError is the title and text of Windows' own box when the app cannot
// open, with the error as a detail for whoever helps.
func StartupError(err error) (title, text string) {
	return T("app.title"), T("error.startup", "reason", err.Error())
}
