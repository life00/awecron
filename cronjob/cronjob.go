package cronjob

import (
	"awecron/data"
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

type (
	Cronjob data.Cronjob
	Cron    data.Cron
)

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
