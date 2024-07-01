package main

import (
	"fmt"
	"log"
	"os"

	// "os/exec"
	// "os/user"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// global awecron config
var cfg struct {
	Max     int
	Min     int
	Timeout int
}

// minimal error check implementation
func check(err *error) {
	if *err != nil {
		log.Fatal(*err)
	}
}

// gets global awecron configuration
func getCfg(cfgDir *string) {
	cfgData, err := os.ReadFile(*cfgDir + "/cfg")
	check(&err)
	err = toml.Unmarshal(cfgData, &cfg)
	check(&err)
}

// gets cronjob directory paths
func getCjDirs(cfgDir *string) (cjDirs []string) {
	cjTmrs, err := filepath.Glob(*cfgDir + "/*/tmr")
	check(&err)
	// removing the /tmr end
	for i := 0; i < len(cjTmrs); i++ {
		cjDirs = append(cjDirs, strings.TrimSuffix(cjTmrs[i], "/tmr"))
	}
	return cjDirs
}

func main() {
	// TODO: implement awecron config directory detection
	cfgDir := "/tmp/awecron"

	// getting global awecron configuration
	getCfg(&cfgDir)

	// for testing
	fmt.Print("\nMax: ", cfg.Max, "\nMin: ", cfg.Min, "\nTimeout: ", cfg.Timeout, "\n")

	// getting cronjob directories
	var cjDirs []string = getCjDirs(&cfgDir)

	// for testing
	for i := 0; i < len(cjDirs); i++ {
		fmt.Println(cjDirs[i])
	}
	// ...
}
