package main

import (
	"encoding/json"
	"fmt"
	"github.com/GeertJohan/go.airbrake"
	"github.com/GeertJohan/go.linenoise"
	"github.com/foize/go.sgr"
	"log"
	"os"
)

// Config contains the project configuration used for the tests
type Config struct {
	ProjectID string `json:"projectID"`
	APIKey    string `json:"apiKey"`
}

var brake *airbrake.Brake

func main() {
	var err error

	// open config file
	configFile, err := os.Open("./config.json")
	if err != nil {
		log.Fatalf("error opening config file: %s\n", err)
	}
	defer configFile.Close()

	// decode config file
	config := &Config{}
	err = json.NewDecoder(configFile).Decode(config)
	if err != nil {
		if err == os.ErrNotExist {
		}
		log.Fatalf("error decoding configFile: %s\n", err)
	}

	if len(config.ProjectID) == 0 {
		// get project id
		config.ProjectID, err = linenoise.Line("Please enter a project ID: ")
		if err != nil {
			fmt.Printf("error reading ID: %s\n", err)
			os.Exit(1)
		}
	}

	if len(config.APIKey) == 0 {
		// get api key
		config.APIKey, err = linenoise.Line("Please enter the api key: ")
		if err != nil {
			fmt.Printf("error reading api key: %s\n", err)
			os.Exit(1)
		}
	}

	// get problem
	problem, err := linenoise.Line("What's your problem?!?: ")
	if err != nil {
		fmt.Printf("error reading user's problem: %s\n", err)
		os.Exit(1)
	}

	// create brake
	brake = airbrake.NewBrake(config.ProjectID, config.APIKey, "go.airbrake example", &airbrake.Config{
		DebugLogIn:  sgr.NewColorWriter(os.Stdout, sgr.FgYellow, true),
		DebugLogOut: sgr.NewColorWriter(os.Stdout, sgr.FgBlue, true),
		URLService:  airbrake.URLService_Airbat,
	})

	brake.Errorf("user-problem", "User has problem: %s", problem)
}
