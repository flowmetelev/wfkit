package sound

import (
	"os/exec"
	"runtime"
)

func PlaySuccessSound() error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("afplay", "sounds/publish.mp3")
	case "windows":
		cmd = exec.Command("powershell", "-c", "(New-Object Media.SoundPlayer 'sounds/publish.mp3').PlaySync()")
	case "linux":
		cmd = exec.Command("mpg123", "-q", "sounds/publish.mp3")
	default:
		return nil
	}
	return cmd.Run()
}
