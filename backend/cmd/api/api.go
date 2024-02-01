package main

import (
	"fmt"
	"os"

	"github.com/veops/oneterm/cmd/api/app"
)

func main() {
	command := app.NewServerCommand()

	if err := command.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
