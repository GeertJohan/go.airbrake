package main

import (
	"fmt"
	"github.com/GeertJohan/go.airbrake"
	"github.com/GeertJohan/go.linenoise"
	"os"
)

func main() {
	// get api key
	apiKey, err := linenoise.Line("Please enter an api key: ")
	if err != nil {
		fmt.Printf("error reading api key: %s\n", err)
		os.Exit(1)
	}

	// create brake
	brake := airbrake.NewBrake(apiKey, "go.airbrake example", &airbrake.Config{
		EnvironmentVersion: "0.1",
	})

	brake.Error("tipe", "message can be longer...")
}
