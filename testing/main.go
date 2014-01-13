package main

import (
	"encoding/json"
	"github.com/GeertJohan/go.airbrake"
	"log"
	"os"
)

// Config contains the project configuration used for the tests
type Config struct {
	ProjectID string `json:"projectID"`
	APIKey    string `json:"apiKey"`
}

var brake *airbrake.Brake

func initBrake() {
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
		log.Fatalf("error decoding configFile: %s\n", err)
	}

	// setup brake
	brake = airbrake.NewBrake(config.ProjectID, config.APIKey, "local-tester", nil)
}

func main() {
	initBrake()

	brake.Error("oops", "this is the first, and certainly not last, mistake.")

	largerStackError()
}

func largerStackError() {
	brake.Error("oops", "this error has a slightly larger stack-trace")
}
