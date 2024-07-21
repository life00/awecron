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

// cron struct
type Cron struct {
	CfgDir      string
	CjDirs      []string
	CjSchedules []int
	Cfg
}

// global awecron config type
type Cfg struct {
	Max     int
	Min     int
	Timeout int
}

// cronjob struct
type Cronjob struct {
	*Cron
	Dir      string
	Schedule int
}

// sets the logging format
func SetLog() {
	var logPrefix string
	// get current user, or error
	if curUser, err := user.Current(); err == nil {
		logPrefix = fmt.Sprintf("awecron (%s) ", curUser.Username)
	} else {
		log.Printf("awecron [ERROR]: failed to get current user for logging")
		logPrefix = "awecron "
	}
	// set the logging prefix
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix(logPrefix)
}

// gets global configuration directory path
// HACK: this function may need improvements
func (c *Cron) GetCfgDir() {
	// config in $XDG_CONFIG_DIR/awecron or $HOME/.config/awecron
	// get user config directory, check if file/directory exists, check if its a directory
	if userCfgDir, err := os.UserConfigDir(); err == nil {
		if cfgDirInfo, err := os.Stat(userCfgDir + "/awecron"); err == nil {
			if cfgDirInfo.IsDir() {
				// return if successful
				c.CfgDir = userCfgDir + "/awecron"
				return
			} else {
				log.Fatalf("[FATAL]: global config directory %s is not a directory", userCfgDir+"/awecron")
			}
		}
	}
	// config in /etc/awecron
	// check if awecron file/directory exists, check if its a directory
	if cfgDirInfo, err := os.Stat("/etc/awecron"); err == nil {
		if cfgDirInfo.IsDir() {
			// return if successful
			c.CfgDir = "/etc/awecron"
			return
		} else {
			log.Fatalf("[FATAL]: global config directory %s is not a directory", "/etc/awecron")
		}
	}
	// could not find any matching directories
	log.Fatalf("[FATAL]: global config directory does not exist")
}

// gets global awecron configuration
func (c *Cron) GetCfg() {
	cfgData, err := os.ReadFile(c.CfgDir + "/cfg")
	if err != nil {
		log.Fatalf("[FATAL]: problem reading global config file CfgDir/cfg and saving as global config data cfgData")
	}
	err = toml.Unmarshal(cfgData, &c.Cfg)
	if err != nil {
		log.Fatalf("[FATAL]: problem unmarshalling global config data cfgData as struct cfg{}")
	}
	if c.Max <= 0 || c.Min <= 0 || c.Timeout <= 0 {
		log.Fatalf("[FATAL]: global config values cfg{} should be greater than zero")
	}
}

// gets cronjob directory paths
func (c *Cron) GetCjDirs() {
	cjTmrs, err := filepath.Glob(c.CfgDir + "/*/tmr")
	if err != nil {
		log.Fatalf("[FATAL]: problem matching CfgDir/*/tmr and getting an array of cronjob timers cjTmrs")
	}
	// removing the /tmr end
	for t := 0; t < len(cjTmrs); t++ {
		c.CjDirs = append(c.CjDirs, strings.TrimSuffix(cjTmrs[t], "/tmr"))
	}
}

// check if its time to run the cronjob
func (cj *Cronjob) Check() bool {
	// getting last modification date of tmr file
	cjTmrInfo, err := os.Stat(cj.Dir + "/tmr")
	if err != nil {
		log.Printf("[ERROR] {%s}: problem getting last modification date of dir/tmr file as file info cjTmrInfo", path.Base(cj.Dir))
		// the 0 returned for schedule is fixed later in main()
		// this also applies to all returns in run and schedule
		cj.Schedule = 0
		return false
	}
	schedule := cjTmrInfo.ModTime().Unix()
	// check if its time to run the cronjob
	if schedule < time.Now().Unix() {
		cj.Schedule = 0
		return true
	} else {
		cj.Schedule = int(schedule)
		return false
	}
}

// run the cronjob
func (cj *Cronjob) Run() bool {
	// remove tmr file to disable cronjob in case of errors
	err := os.Remove(cj.Dir + "/tmr")
	if err != nil {
		// fatal error because if it fails to disable the cronjob due to a problem then there may be an infinite loop
		log.Fatalf("[FATAL] {%s}: problem deleting dir/tmr file", path.Base(cj.Dir))
	}
	// declaring context timeout
	cjCtx, cjCtxCancel := context.WithTimeout(context.Background(), time.Duration(cj.Timeout)*time.Second)
	defer cjCtxCancel()
	// creating the cmd struct with context timeout
	cjCmd := exec.CommandContext(cjCtx, cj.Dir+"/run")
	// modifying function which will be used to stop the cronjob if it times out
	// so that it contains the log message that cronjob has timed out
	cjCmd.Cancel = func() (err error) {
		// stopping the cronjob
		err = cjCmd.Process.Kill()
		if err != nil {
			// non fatal error because if cjCmd.Process.Kill() will fail to stop the process
			// cjCmd.Run() will exit and forward this error, which will say that cronjob returned an error
			// so it won't reenable the cronjob and there is no persistent problem
			log.Printf("[ERROR] {%s}: failed to stop the timed out cronjob", path.Base(cj.Dir))
			return err
		}
		// log that the cronjob has timed out
		log.Printf("[INFO] {%s}: cronjob run has timed out, stopping", path.Base(cj.Dir))
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
		// log everything
		log.Printf("[INFO] {%s} [%d]: cronjob run is successful", path.Base(cj.Dir), cjCmd.ProcessState.ExitCode())
		return true
	} else {
		// log exit status
		log.Printf("[ERROR] {%s} [%d]: cronjob run returned an error", path.Base(cj.Dir), cjCmd.ProcessState.ExitCode())
		// log stderr if it is not empty
		if cjStderr.String() != "" {
			log.Printf("[INFO] {%s}: cronjob run stderr output:\n==========\n%s\n==========", path.Base(cj.Dir), cjStderr.String())
		}
		return false
	}
}

// schedule the next run of the cronjob
func (cj *Cronjob) GetSchedule() {
	// getting the plain text interval configuration
	// its also possible to do it with fmt.Fscanf, but I've chosen this option
	cjCfgData, err := os.ReadFile(cj.Dir + "/cfg")
	if err != nil {
		log.Printf("[ERROR] {%s}: problem reading cronjob config file dir/cfg and saving as cronjob config data cjCfgData", path.Base(cj.Dir))
		cj.Schedule = 0
		return
	}
	// conversion
	cjCfg, err := strconv.Atoi(strings.TrimSpace(string(cjCfgData)))
	if err != nil {
		log.Printf("[ERROR] {%s}: problem converting cronjob config data cjCfgData into cronjob config integer cjCfg", path.Base(cj.Dir))
		cj.Schedule = 0
		return
	}
	// make sure its greater than zero
	if cjCfg <= 0 {
		log.Printf("[ERROR] {%s}: cronjob config cjCfg should be greater than zero", path.Base(cj.Dir))
		cj.Schedule = 0
		return
	}
	// create tmr file again
	cjTmr, err := os.Create(cj.Dir + "/tmr")
	// all fatal errors because I am not risking with tmr file
	// because it might result in an infinite loop for whatever reason
	if err != nil {
		log.Fatalf("[FATAL] {%s}: problem creating dir/tmr file", path.Base(cj.Dir))
	}
	// closing cjTmr file
	err = cjTmr.Close()
	if err != nil {
		log.Fatalf("[FATAL] {%s}: problem closing tmr file cjTmr", path.Base(cj.Dir))
	}
	// get next run time
	schedule := time.Now().Unix() + int64(cjCfg)
	// set the next run time as last modification time
	err = os.Chtimes(cj.Dir+"/tmr", time.Time{}, time.Unix(schedule, int64(0)))
	if err != nil {
		log.Fatalf("[FATAL] {%s}: problem setting last modification time of tmr file", path.Base(cj.Dir))
	}
	cj.Schedule = int(schedule)
}

// get optimal sleep time until next cronjob
func (c Cron) GetSleepTime() (sleepTime int) {
	// if there is no cronjobs sleep max time
	if len(c.CjSchedules) == 0 {
		log.Printf("[INFO]: no enabled cronjobs found, sleeping max time")
		return c.Max
	}
	// get the smallest unix time stamp from cronjob schedules
	minCjSchedule := c.CjSchedules[0]
	for _, schedule := range c.CjSchedules[1:] {
		if schedule < minCjSchedule {
			minCjSchedule = schedule
		}
	}
	// get the sleep time
	sleepTime = minCjSchedule - int(time.Now().Unix())
	// apply the limits
	if sleepTime < c.Min {
		sleepTime = c.Min
	} else if sleepTime > c.Max {
		sleepTime = c.Max
	}
	// return the optimal sleep time
	return sleepTime
}

func main() {
	// setting the logging format
	SetLog()
	// getting the config directory
	var c Cron
	c.GetCfgDir()
	// getting global awecron configuration
	c.GetCfg()
	// infinite loop
	for {
		// getting cronjob directories
		c.GetCjDirs()
		// create mutex for managing above array inside of goroutines
		var cjMutex sync.Mutex
		// create wait group for goroutines
		var cjWG sync.WaitGroup
		// loop through CjDirs
		for i := range c.CjDirs {
			cj := Cronjob{&c, c.CjDirs[i], 0}
			// add one goroutine to wait group
			cjWG.Add(1)
			// initialize goroutine
			go func() {
				defer cjWG.Done()
				// in awecron.sh I run a separate function for dynamic sleep feature to determine how much for awecron to sleep, which is pretty inefficient
				// here instead I will make existing functions return necessary values
				// check if its necessary to run the cronjob
				if cj.Check() {
					// run the cronjob
					if cj.Run() {
						// schedule the cronjob for next run
						cj.GetSchedule()
					}
				}
				// if the function fails it has to return something as schedule, so it returns 0
				// so if its 0 it won't add it to the array of schedules at all
				if cj.Schedule != 0 {
					// mutex lock while appending schedule to an array, then unlock
					cjMutex.Lock()
					// append the next run time to the array of schedules
					c.CjSchedules = append(c.CjSchedules, cj.Schedule)
					cjMutex.Unlock()
				}
			}()
		}
		// wait until all cronjobs finish
		cjWG.Wait()
		// wipe CjDirs array
		c.CjDirs = nil
		// get optimal sleep time and sleep for that number of seconds
		time.Sleep(time.Duration(c.GetSleepTime()) * time.Second)
		// wipe CjSchedules array
		c.CjSchedules = nil
	}
}
