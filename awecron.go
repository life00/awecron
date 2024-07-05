package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// global awecron config type
type cfgType struct {
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
func getCfg(cfgDir *string, cfg *cfgType) {
	cfgData, err := os.ReadFile(*cfgDir + "/cfg")
	if err != nil {
		curUser, _ := user.Current()
		log.Fatalf("awecron fatal (%s): problem reading global config file cfgDir/cfg and saving as global config data cfgData", curUser.Username)
	}
	err = toml.Unmarshal(cfgData, cfg)
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
func checkCj(cjDir *string) (bool, int) {
	// getting last modification date of tmr file
	cjTmrInfo, err := os.Stat(*cjDir + "/tmr")
	if err != nil {
		curUser, _ := user.Current()
		log.Printf("awecron error (%s) {%s}: problem getting last modification date of cjDir/tmr file as file info cjTmrInfo", curUser.Username, path.Base(*cjDir))
		// the 0 returned for cjSchedule is fixed later in main()
		// this also applies to all returns in runCj and scheduleCj
		return false, 0
	}
	cjSchedule := cjTmrInfo.ModTime().Unix()
	// check if its time to run the cronjob
	if cjSchedule < time.Now().Unix() {
		return true, 0
	} else {
		return false, int(cjSchedule)
	}
}

// run the cronjob
func runCj(cjDir *string, cjTimeout *int) bool {
	// remove tmr file to disable cronjob in case of errors
	err := os.Remove(*cjDir + "/tmr")
	if err != nil {
		curUser, _ := user.Current()
		// fatal error because if it fails to disable the cronjob due to a problem then there may be an infinite loop
		log.Fatalf("awecron fatal (%s) {%s}: problem deleting cjDir/tmr file", curUser.Username, path.Base(*cjDir))
	}
	// declaring context timeout
	cjCtx, cjCtxCancel := context.WithTimeout(context.Background(), time.Duration(*cjTimeout)*time.Second)
	defer cjCtxCancel()
	// creating the cmd struct with context timeout
	cjCmd := exec.CommandContext(cjCtx, *cjDir+"/run")
	// modifying function which will be used to stop the cronjob if it times out
	// so that it contains the log message that cronjob has timed out
	cjCmd.Cancel = func() (err error) {
		// stopping the cronjob
		err = cjCmd.Process.Kill()
		if err != nil {
			curUser, _ := user.Current()
			// non fatal error because if cjCmd.Process.Kill() will fail to stop the process
			// cjCmd.Run() will exit and forward this error, which will say that cronjob returned an error
			// so it won't reenable the cronjob and there is no persistent problem
			log.Printf("awecron error (%s) {%s}: failed to stop the timed out cronjob", curUser.Username, path.Base(*cjDir))
			return err
		}
		// log that the cronjob has timed out
		curUser, _ := user.Current()
		log.Printf("awecron info (%s) {%s}: cronjob run has timed out, stopping", curUser.Username, path.Base(*cjDir))
		return nil
	}
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
func scheduleCj(cjDir *string) int {
	// getting the plain text interval configuration
	// its also possible to do it with fmt.Fscanf, but I've chosen this option
	cjCfgData, err := os.ReadFile(*cjDir + "/cfg")
	if err != nil {
		curUser, _ := user.Current()
		log.Printf("awecron error (%s) {%s}: problem reading cronjob config file cjDir/cfg and saving as cronjob config data cjCfgData", curUser.Username, path.Base(*cjDir))
		return 0
	}
	// conversion
	cjCfg, err := strconv.Atoi(strings.TrimSpace(string(cjCfgData)))
	if err != nil {
		curUser, _ := user.Current()
		log.Printf("awecron error (%s) {%s}: problem converting cronjob config data cjCfgData into cronjob config integer cjCfg", curUser.Username, path.Base(*cjDir))
		return 0
	}
	// make sure its greater than zero
	if cjCfg <= 0 {
		curUser, _ := user.Current()
		log.Printf("awecron error (%s) {%s}: cronjob config cjCfg should be greater than zero", curUser.Username, path.Base(*cjDir))
		return 0
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
	// get next run time
	cjSchedule := time.Now().Unix() + int64(cjCfg)
	// set the next run time as last modification time
	err = os.Chtimes(*cjDir+"/tmr", time.Time{}, time.Unix(cjSchedule, int64(0)))
	if err != nil {
		curUser, _ := user.Current()
		log.Fatalf("awecron fatal (%s) {%s}: problem setting last modification time of tmr file", curUser.Username, path.Base(*cjDir))
	}
	return int(cjSchedule)
}

// get optimal sleep time until next cronjob
func getSleepTime(cjSchedules []int, cfg *cfgType) (sleepTime int) {
	// get the smallest unix time stamp from cronjob schedules
	minCjSchedule := cjSchedules[0]
	for _, cjSchedule := range cjSchedules[1:] {
		if cjSchedule < minCjSchedule {
			minCjSchedule = cjSchedule
		}
	}
	// get the sleep time
	sleepTime = minCjSchedule - int(time.Now().Unix())
	// apply the limits
	if sleepTime < cfg.Min {
		sleepTime = cfg.Min
	} else if sleepTime > cfg.Max {
		sleepTime = cfg.Max
	}
	// return the optimal sleep time
	return sleepTime
}

func main() {
	// getting the config directory
	cfgDir := getCfgDir()
	// global awecron config
	var cfg cfgType
	// getting global awecron configuration
	getCfg(&cfgDir, &cfg)
	// infinite loop
	for {
		// getting cronjob directories
		cjDirs := getCjDirs(&cfgDir)
		// array of unix time stamps until next cronjob run
		var cjSchedules []int
		// create mutex for managing above array inside of goroutines
		var cjMutex sync.Mutex
		// create wait group for goroutines
		var cjWG sync.WaitGroup
		// TODO: implement timeout
		for _, cjDir := range cjDirs {
			// add one goroutine to wait group
			cjWG.Add(1)
			// initialize goroutine
			go func() {
				defer cjWG.Done()
				// in awecron.sh I run a separate function for dynamic sleep feature to determine how much for awecron to sleep, which is pretty inefficient
				// here instead I will make existing functions return necessary values
				var cjSchedule int
				var checkCjReturn bool
				// check if its necessary to run the cronjob
				if checkCjReturn, cjSchedule = checkCj(&cjDir); checkCjReturn {
					// run the cronjob
					if runCj(&cjDir, &cfg.Timeout) {
						// schedule the cronjob for next run
						cjSchedule = scheduleCj(&cjDir)
					}
				}
				// if the function fails it has to return something as cjSchedule, so it returns 0
				// so if its 0 it won't add it to the array of schedules at all
				if cjSchedule != 0 {
					// mutex lock while appending cjSchedule to an array, then unlock
					cjMutex.Lock()
					// append the next run time to the array of schedules
					cjSchedules = append(cjSchedules, cjSchedule)
					cjMutex.Unlock()
				}
			}()
		}
		// wait until all cronjobs finish
		cjWG.Wait()
		// get optimal sleep time and sleep for that number of seconds
		time.Sleep(time.Duration(getSleepTime(cjSchedules, &cfg)) * time.Second)
	}
}
