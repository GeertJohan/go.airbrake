package main

import (
	"fmt"
	"github.com/GeertJohan/go.airbrake"
	"github.com/GeertJohan/go.linenoise"
	"github.com/foize/go.sgr"
	"os"
)

func main() {
	// get project id
	projectID, err := linenoise.Line("Please enter a project ID: ")
	if err != nil {
		fmt.Printf("error reading ID: %s\n", err)
		os.Exit(1)
	}

	// get api key
	apiKey, err := linenoise.Line("Please enter the api key: ")
	if err != nil {
		fmt.Printf("error reading api key: %s\n", err)
		os.Exit(1)
	}

	// get problem
	problem, err := linenoise.Line("What's your problem?!?: ")
	if err != nil {
		fmt.Printf("error reading user's problem: %s\n", err)
		os.Exit(1)
	}

	// create brake
	brake := airbrake.NewBrake(projectID, apiKey, "go.airbrake example", &airbrake.Config{
		InLog:  sgr.NewColorWriter(os.Stdout, sgr.FgYellow, true),
		OutLog: sgr.NewColorWriter(os.Stdout, sgr.FgBlue, true),
	})

	brake.Errorf("user-problem", "User has problem: %s", problem)
}
