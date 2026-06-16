package main

import (
	"github.com/fikriaf/ngodeai-cli/cmd"
	"github.com/fikriaf/ngodeai-cli/internal/logging"
)

func main() {
	defer logging.RecoverPanic("main", func() {
		logging.Error("Application terminated due to unhandled panic")
	})

	cmd.Execute()
}
