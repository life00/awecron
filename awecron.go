package main

import (
	"bytes"
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

// gets global configuration directory path
// HACK: this function may need improvements
func getCfgDir() string {
	// config in $XDG_CONFIG_DIR/awecron or $HOME/.config/awecron
	// get user config directory, check if file/directory exists, check if its a directory
	if userCfgDir, err := os.UserConfigDir(); err == nil {
		if cfgDirInfo, err := os.Stat(userCfgDir + "/awecron"); err == nil {
			if cfgDirInfo.IsDir() {
				// return if successful
				return userCfgDir + "/awecron"
			} else {
				curUser, _ := user.Current()
				log.Fatalf("awecron fatal (%s): global config directory %s is not a directory", curUser.Username, userCfgDir+"/awecron")
			}
		}
	}
	// config in /etc/awecron
	// check if awecron file/directory exists, check if its a directory
	if cfgDirInfo, err := os.Stat("/etc/awecron"); err == nil {
		if cfgDirInfo.IsDir() {
			// return if successful
			return "/etc/awecron"
		} else {
			curUser, _ := user.Current()
			log.Fatalf("awecron fatal (%s): global config directory %s is not a directory", curUser.Username, "/etc/awecron")
		}
	}
	// could not find any matching directories
	curUser, _ := user.Current()
	log.Fatalf("awecron fatal (%s): global config directory does not exist", curUser.Username)
	return ""
}

// gets global awecron configuration
func getCfg(cfgDir *string) {
	cfgData, err := os.ReadFile(*cfgDir + "/cfg")
	if err != nil {
		curUser, _ := user.Current()
		log.Fatalf("awecron fatal (%s): problem reading global config file cfgDir/cfg and saving as global config data cfgData", curUser.Username)
	}
	err = toml.Unmarshal(cfgData, &cfg)
	if err != nil {
		curUser, _ := user.Current()
		log.Fatalf("awecron fatal (%s): problem unmarshalling global config data cfgData as struct cfg{}", curUser.Username)
	}
	if cfg.Max <= 0 || cfg.Min <= 0 || cfg.Timeout <= 0 {
		curUser, _ := user.Current()
		log.Fatalf("awecron fatal (%s): global config values cfg{} should be greater than zero", curUser.Username)
	}
}

// gets cronjob directory paths
func getCjDirs(cfgDir *string) (cjDirs []string) {
	cjTmrs, err := filepath.Glob(*cfgDir + "/*/tmr")
	if err != nil {
		curUser, _ := user.Current()
		log.Fatalf("awecron fatal (%s): problem matching cfgDir/*/tmr and getting an array of cronjob timers cjTmrs", curUser.Username)
	}
	// removing the /tmr end
	for t := 0; t < len(cjTmrs); t++ {
		cjDirs = append(cjDirs, strings.TrimSuffix(cjTmrs[t], "/tmr"))
	}
	return cjDirs
}

// check if its time to run the cronjob
func checkCj(cjDir *string) bool {
	// getting last modification date of tmr file
	cjTmrInfo, err := os.Stat(*cjDir + "/tmr")
	if err != nil {
		curUser, _ := user.Current()
		log.Printf("awecron error (%s) {%s}: problem getting last modification date of cjDir/tmr file as file info cjTmrInfo", curUser.Username, path.Base(*cjDir))
		return false
	}
	// check if its time to run the cronjob
	if cjTmrInfo.ModTime().Unix() < time.Now().Unix() {
		return true
	} else {
		return false
	}
}

// run the cronjob
func runCj(cjDir *string) bool {
	// remove tmr file to disable cronjob in case of errors
	err := os.Remove(*cjDir + "/tmr")
	if err != nil {
		curUser, _ := user.Current()
		// fatal error because if it fails to disable the cronjob due to a problem then there may be an infinite loop
		log.Fatalf("awecron fatal (%s) {%s}: problem deleting cjDir/tmr file", curUser.Username, path.Base(*cjDir))
	}
	// creating the cmd struct
	cjCmd := exec.Command(*cjDir + "/run")
	// recording stderr
	// I could've used cjCmd.CombinedOutput() but I am not interested in recording stdout
	var cjStderr bytes.Buffer
	cjCmd.Stderr = &cjStderr
	// running the executable
	err = cjCmd.Run()
	// if successful run
	if err == nil {
		curUser, _ := user.Current()
		// log everything
		log.Printf("awecron info (%s) {%s} [%d]: cronjob run is successful", curUser.Username, path.Base(*cjDir), cjCmd.ProcessState.ExitCode())
		return true
	} else {
		curUser, _ := user.Current()
		// log exit status
		log.Printf("awecron error (%s) {%s} [%d]: cronjob run returned an error", curUser.Username, path.Base(*cjDir), cjCmd.ProcessState.ExitCode())
		// log stderr if it is not empty
		if cjStderr.String() != "" {
			log.Printf("awecron info (%s) {%s}: cronjob run stderr output:\n==========\n%s\n==========", curUser.Username, path.Base(*cjDir), cjStderr.String())
		}
		return false
	}
}

// schedule the next run of the cronjob
func scheduleCj(cjDir *string) {
	// getting the plain text interval configuration
	// its also possible to do it with fmt.Fscanf, but I've chosen this option
	cjCfgData, err := os.ReadFile(*cjDir + "/cfg")
	if err != nil {
		curUser, _ := user.Current()
		log.Printf("awecron error (%s) {%s}: problem reading cronjob config file cjDir/cfg and saving as cronjob config data cjCfgData", curUser.Username, path.Base(*cjDir))
		return
	}
	// conversion
	cjCfg, err := strconv.Atoi(strings.TrimSpace(string(cjCfgData)))
	if err != nil {
		curUser, _ := user.Current()
		log.Printf("awecron error (%s) {%s}: problem converting cronjob config data cjCfgData into cronjob config integer cjCfg", curUser.Username, path.Base(*cjDir))
		return
	}
	// make sure its greater than zero
	if cjCfg <= 0 {
		curUser, _ := user.Current()
		log.Printf("awecron error (%s) {%s}: cronjob config cjCfg should be greater than zero", curUser.Username, path.Base(*cjDir))
		return
	}
	// create tmr file again
	cjTmr, err := os.Create(*cjDir + "/tmr")
	// all fatal errors because I am not risking with tmr file
	// because it might result in an infinite loop for whatever reason
	if err != nil {
		curUser, _ := user.Current()
		log.Fatalf("awecron fatal (%s) {%s}: problem creating cjDir/tmr file", curUser.Username, path.Base(*cjDir))
	}
	// closing cjTmr file
	err = cjTmr.Close()
	if err != nil {
		curUser, _ := user.Current()
		log.Fatalf("awecron fatal (%s) {%s}: problem closing tmr file cjTmr", curUser.Username, path.Base(*cjDir))
	}
	// set the next run time as last modification time
	err = os.Chtimes(*cjDir+"/tmr", time.Time{}, time.Unix(time.Now().Unix()+int64(cjCfg), int64(0)))
	if err != nil {
		curUser, _ := user.Current()
		log.Fatalf("awecron fatal (%s) {%s}: problem setting last modification time of tmr file", curUser.Username, path.Base(*cjDir))
	}
}

func main() {
	cfgDir := getCfgDir()

	// getting global awecron configuration
	getCfg(&cfgDir)

	// for testing
	fmt.Print("\nMax: ", cfg.Max, "\nMin: ", cfg.Min, "\nTimeout: ", cfg.Timeout, "\n")

	// getting cronjob directories
	var cjDirs []string = getCjDirs(&cfgDir)

	// TODO: implement parallelism
	for d := 0; d < len(cjDirs); d++ {
		// TODO: implement timeout here, or for cjCmd
		if checkCj(&cjDirs[d]) {
			// if cronjob run is successful
			if runCj(&cjDirs[d]) {
				// schedule for next run
				scheduleCj(&cjDirs[d])
			}
		}
		// TODO: dynamic sleep here
	}
	// ...
}
