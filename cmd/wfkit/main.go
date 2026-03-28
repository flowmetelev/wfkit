package main

import (
	"os"

	"wfkit/internal/utils"
)

func main() {
	app := buildApp()

	if err := app.Run(os.Args); err != nil {
		notifyFailure(notificationsEnabled(os.Args[1:]), "wfkit failed", notificationBody(err, "The command finished with an error."))
		hints := errorHints(err, app.Version, os.Args[1:])
		utils.PrintErrorScreen(
			"wfkit failed",
			err,
			hints...,
		)
		os.Exit(1)
	}
}
