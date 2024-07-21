package main

import (
	"awecron/cron"
	"awecron/cronjob"
	"sync"
	"time"
)

func main() {
	// setting the logging format
	cron.SetLog()
	// getting the config directory
	var c cron.Cron
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
			cj := cronjob.Cronjob{&c, c.CjDirs[i], 0}
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
