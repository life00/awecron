package cron

import (
	"awecron/data"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Cron data.Cron

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
