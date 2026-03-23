// internal/initialize/steps/dependencies.go
package steps

import (
	"os"
	"os/exec"
	"wfkit/internal/utils"
)

func InstallDependencies(pkgMgr string) error {
	utils.CPrint("Installing dependencies...", "blue")
	cmd := exec.Command(pkgMgr, "install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
