package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// global awecron config
var cfg struct {
	Max     int
	Min     int
	Timeout int
}

// TODO: move this inside of the code
func panicErr(err *error) {
	if *err != nil {
		log.Panic(*err)
	}
}

// gets global awecron configuration
func getCfg(cfgDir *string) {
	cfgData, err := os.ReadFile(*cfgDir + "/cfg")
	panicErr(&err)
	err = toml.Unmarshal(cfgData, &cfg)
	panicErr(&err)
	if cfg.Max <= 0 || cfg.Min <= 0 || cfg.Timeout <= 0 {
		log.Panic("awecron error: global configuration values cfg{} should be greater than zero")
	}
}

// gets cronjob directory paths
func getCjDirs(cfgDir *string) (cjDirs []string) {
	cjTmrs, err := filepath.Glob(*cfgDir + "/*/tmr")
	panicErr(&err)
	// removing the /tmr end
	for t := 0; t < len(cjTmrs); t++ {
		cjDirs = append(cjDirs, strings.TrimSuffix(cjTmrs[t], "/tmr"))
	}
	return cjDirs
}

func checkCj(cjDir *string) bool {
	// getting last modification date of `tmr` file
	cjTmr, err := os.Stat(*cjDir + "/tmr")
	// panic error because it is not supposed to error usually (i.e. due to user interaction)
	panicErr(&err)
	// check if its time to run the cronjob
	if cjTmr.ModTime().Unix() < time.Now().Unix() {
		return true
	} else {
		return false
	}
}

func runCj(cjDir *string) {
	// remove tmr file to disable cronjob in case of errors
	err := os.Remove(*cjDir + "/tmr")
	// panic error because if it fails to disable then there may be infinite loop
	panicErr(&err)
	// get current running user (for future logs)
	curUser, err := user.Current()
	if err != nil {
		log.Println(err)
	}
	// running the executable
	cjCmd := exec.Command(*cjDir + "/run")
	err = cjCmd.Run()
	// if successful run
	if err == nil {
		// log everything
		log.Printf("awecron (%s) {%s} [%d]: cronjob run success", curUser.Username, path.Base(*cjDir), cjCmd.ProcessState.ExitCode())
		// getting the plain text interval configuration
		// its also possible to do it with fmt.Fscanf, but I've chosen this option
		cjCfgData, err := os.ReadFile(*cjDir + "/cfg")
		if err != nil {
			log.Println(err)
			return
		}
		// conversion
		cjCfg, err := strconv.Atoi(strings.TrimSpace(string(cjCfgData)))
		if err != nil {
			log.Println(err)
			return
		}
		// make sure its greater than zero
		if cjCfg <= 0 {
			log.Println("awecron error: cronjob config cjCfg should be greater than zero")
			return
		}
		// create tmr file again
		cjTmr, err := os.Create(*cjDir + "/tmr")
		// panic errors to avoid any possible infinite loops
		panicErr(&err)
		err = cjTmr.Close()
		panicErr(&err)
		// set the next run time as last modification time
		err = os.Chtimes(*cjDir+"/tmr", time.Time{}, time.Unix(time.Now().Unix()+int64(cjCfg), int64(0)))
		panicErr(&err)
	} else {
		// log everything
		log.Printf("awecron (%s) {%s} [%d]: cronjob run error", curUser.Username, path.Base(*cjDir), cjCmd.ProcessState.ExitCode())
	}
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

	for d := 0; d < len(cjDirs); d++ {
		if checkCj(&cjDirs[d]) {
			runCj(&cjDirs[d])
		}
	}
	// ...
}
