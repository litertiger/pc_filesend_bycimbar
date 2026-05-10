package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// notifySuccess sends a desktop notification and prints to console.
func notifySuccess(title, body string) {
	fmt.Printf("\n[OK] %s\n     %s\n", title, body)
	sendDesktopNotification(title, body, false)
}

// notifyError sends a desktop notification and prints to console.
func notifyError(title, body string) {
	fmt.Printf("\n[ERR] %s\n      %s\n", title, body)
	sendDesktopNotification(title, body, true)
}

func sendDesktopNotification(title, body string, isError bool) {
	switch runtime.GOOS {
	case "windows":
		sendWindowsNotification(title, body, isError)
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		_ = exec.Command("osascript", "-e", script).Run()
	default: // linux and others
		urgency := "normal"
		if isError {
			urgency = "critical"
		}
		_ = exec.Command("notify-send", "-u", urgency, "--", title, body).Run()
	}
}

// sendWindowsNotification shows a Windows 10 system-tray balloon tip.
//
// Title and body are passed via environment variables to avoid any PowerShell
// injection issues with special characters in filenames.
// The PowerShell process is started in the background (Start, not Run) so the
// main program is not blocked while the balloon is visible.
func sendWindowsNotification(title, body string, isError bool) {
	iconKind := "Information"
	if isError {
		iconKind = "Error"
	}

	// Use a here-string and env vars so that filenames with quotes, brackets,
	// dollar signs, etc. never break the PowerShell script.
	script := `
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$n = New-Object System.Windows.Forms.NotifyIcon
$n.Icon = [System.Drawing.SystemIcons]::` + iconKind + `
$n.BalloonTipTitle = $env:NOTIF_TITLE
$n.BalloonTipText  = $env:NOTIF_BODY
$n.BalloonTipIcon  = [System.Windows.Forms.ToolTipIcon]::` + iconKind + `
$n.Visible = $true
$n.ShowBalloonTip(8000)
Start-Sleep -Milliseconds 8500
$n.Dispose()
`
	cmd := exec.Command("powershell.exe",
		"-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden",
		"-Command", script)
	cmd.Env = append(os.Environ(),
		"NOTIF_TITLE="+title,
		"NOTIF_BODY="+body,
	)
	// Fire-and-forget: do not wait for the balloon to expire.
	_ = cmd.Start()
}

// termBell prints an ASCII bell character to the terminal.
func termBell() {
	fmt.Print("\a")
}
