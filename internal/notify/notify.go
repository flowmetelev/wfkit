package notify

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

var ErrUnsupported = errors.New("desktop notifications are not available")

func Success(title, body string) error {
	return send(title, body)
}

func Failure(title, body string) error {
	return send(title, body)
}

func send(title, body string) error {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if title == "" || body == "" {
		return nil
	}

	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("osascript"); err != nil {
			return ErrUnsupported
		}
		script := fmt.Sprintf("display notification %s with title %s", strconv.Quote(body), strconv.Quote(title))
		return exec.Command("osascript", "-e", script).Run()
	case "linux":
		if _, err := exec.LookPath("notify-send"); err != nil {
			return ErrUnsupported
		}
		return exec.Command("notify-send", title, body).Run()
	case "windows":
		if _, err := exec.LookPath("powershell"); err != nil {
			return ErrUnsupported
		}
		command := fmt.Sprintf(
			`Add-Type -AssemblyName System.Windows.Forms; Add-Type -AssemblyName System.Drawing; $n=New-Object System.Windows.Forms.NotifyIcon; $n.Icon=[System.Drawing.SystemIcons]::Information; $n.BalloonTipTitle=%s; $n.BalloonTipText=%s; $n.Visible=$true; $n.ShowBalloonTip(5000); Start-Sleep -Seconds 6; $n.Dispose()`,
			powerShellQuoted(title),
			powerShellQuoted(body),
		)
		return exec.Command("powershell", "-NoProfile", "-Command", command).Run()
	default:
		return ErrUnsupported
	}
}

func powerShellQuoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
