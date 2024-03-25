package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// config of individual cronjobs
var cfg struct {
	User string
	Name string
	Time int
}

// minimal error check implementation
func check(err *error) {
	if *err != nil {
		log.Fatal(*err)
	}
}

// running a cronjob
func run(cronjobPath *string) {
	// reading config
	cfgData, err := os.ReadFile(*cronjobPath + "/cfg")
	check(&err)

	// unmarshal and write config to struct
	err = toml.Unmarshal(cfgData, &cfg)
	check(&err)
	cfgData = nil                          // wipe unneeded variable
	cfg.Name = filepath.Base(*cronjobPath) // set cfg.Name from the directory name

	log.Print(cfg.Name, " ", cfg.User)
	// ...
}

func main() {
	// TODO: detection of cronjobs and loop
	cronjobPath := "/tmp/cronjob/"
	// ...
	run(&cronjobPath)
	log.Print("success")
	// ...
}
