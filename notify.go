package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

// notifySuccess sends a desktop notification and prints to console.
func notifySuccess(title, body string) {
	fmt.Printf("\n✓ %s\n  %s\n", title, body)
	sendDesktopNotification(title, body, "normal")
}

// notifyError sends a desktop notification and prints to console.
func notifyError(title, body string) {
	fmt.Printf("\n✗ %s\n  %s\n", title, body)
	sendDesktopNotification(title, body, "critical")
}

func sendDesktopNotification(title, body, urgency string) {
	switch runtime.GOOS {
	case "linux":
		// notify-send is part of libnotify-bin (Freedesktop notifications).
		args := []string{"-u", urgency, "--", title, body}
		_ = exec.Command("notify-send", args...).Run()
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		_ = exec.Command("osascript", "-e", script).Run()
	case "windows":
		// PowerShell toast notification (Windows 10+).
		ps := fmt.Sprintf(
			`[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null;`+
				`$xml = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02);`+
				`$xml.GetElementsByTagName("text")[0].AppendChild($xml.CreateTextNode('%s')) | Out-Null;`+
				`$xml.GetElementsByTagName("text")[1].AppendChild($xml.CreateTextNode('%s')) | Out-Null;`+
				`[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('cimbar-recv').Show($xml)`,
			title, body)
		_ = exec.Command("powershell", "-Command", ps).Run()
	}
}

// termBell prints an ASCII bell character to the terminal.
func termBell() {
	fmt.Print("\a")
}
