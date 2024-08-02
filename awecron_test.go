package main

// I will say right away that I will not be testing log output. It
// would require capturing stdout and stderr, and then parsing it.
// I believe its too complicated and it would be sufficient to only
// test the actual runtime environment and results.

// The tests cover all the expected functionality and reasonably possible errors,
// except for fatal errors. I will not do fatal error handling because
// it would require somehow overwriting the log.Fatalf function or
// os.Exit, but this seems to be overly complicated. Ideally I was
// supposed to not use fatal errors at all and instead return errors up
// the functions, but here we are. This means that only non fatal
// options will be tested.  TODO: return errors to main instead of log.Fatal()

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"reflect"
	"testing"
	"time"
)

// awecron config directory for all tests
var CfgDir string

// this function is not testing the main function
// it controls how the tests are run
func TestMain(m *testing.M) {
	var err error
	// create a temporary config directory for all tests
	CfgDir, err = os.MkdirTemp("", "awecron")
	if err != nil {
		panic("Failed to create temporary directory")
	}
	fmt.Printf("performing tests inside %s temporary directory\n", CfgDir)
	// running the actual tests
	exitCode := m.Run()
	// cleanup
	os.RemoveAll(CfgDir)
	// return exit code
	os.Exit(exitCode)
}

// test SetLog()
func TestSetLog(t *testing.T) {
	SetLog()
	curUser, err := user.Current()
	if err != nil {
		t.Errorf("failed to get current user for testing\n")
	}
	expected := fmt.Sprintf("awecron (%s) ", curUser.Username)
	result := log.Prefix()
	if result != expected {
		t.Errorf("logging format did not match expected logging format")
	}
}

// test GetCfgDir()
// it is difficult to test this function because it cannot be
// isolated from the actual OS
// func TestGetCfgDir(t *testing.T) {
// 	result := GetCfgDir()
// 	t.Log(result)
// }

// test GetCfg()
func TestGetCfg(t *testing.T) {
	t.Cleanup(func() {
		os.Remove(CfgDir + "/cfg")
	})
	// writing config file
	input := []byte("# comment\nmin = 5\n\ntimeout = 10\nmax = 600\n")
	err := os.WriteFile(CfgDir+"/cfg", input, 0644)
	if err != nil {
		t.Errorf("failed to create global config file for testing\n")
	}
	expected := CfgType{
		Max:     600,
		Min:     5,
		Timeout: 10,
	}
	var result CfgType
	GetCfg(&CfgDir, &result)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("value in struct cfg did not match expected value\n")
	}
}

// test GetCjDirs()
func TestGetCjDirs(t *testing.T) {
	t.Cleanup(func() {
		for i := 1; i < 6; i++ {
			os.RemoveAll(fmt.Sprintf("%s/cronjob%d", CfgDir, i))
		}
	})
	// creating 5 cronjob{1,2,3,4,5}/tmr
	for i := 1; i < 6; i++ {
		cjDir := fmt.Sprintf("%s/cronjob%d", CfgDir, i)
		err := os.Mkdir(cjDir, 0755)
		if err != nil {
			t.Errorf("failed to create cronjob directory for testing\n")
		}
		f, err := os.Create(cjDir + "/tmr")
		if err != nil {
			t.Errorf("failed to create cronjob tmr file for testing\n")
		}
		f.Close()
	}
	expected := []string{CfgDir + "/cronjob1", CfgDir + "/cronjob2", CfgDir + "/cronjob3", CfgDir + "/cronjob4", CfgDir + "/cronjob5"}
	result := GetCjDirs(&CfgDir)
	if len(result) != len(expected) {
		t.Errorf("length of array cjDirs did not match expected length\n")
	}
	for i := range result {
		if result[i] != expected[i] {
			t.Errorf("value of index %d in array cjDirs did not match expected value\n", i)
		}
	}
}

// test CheckCj()
func TestCheckCj(t *testing.T) {
	t.Cleanup(func() {
		os.RemoveAll(CfgDir + "/ready")
		os.RemoveAll(CfgDir + "/not_ready")
	})
	// type for storing expected data and results
	type tests struct {
		ready bool
		time  int
	}
	// create ready cronjob and test it
	t.Run("ready", func(t *testing.T) {
		cjDir := CfgDir + "/ready"
		err := os.Mkdir(cjDir, 0755)
		if err != nil {
			t.Errorf("failed to create cronjob directory for testing\n")
		}
		cjTmr, err := os.Create(cjDir + "/tmr")
		if err != nil {
			t.Errorf("failed to create cronjob tmr file for testing\n")
		}
		cjTmr.Close()
		// change last modification time to zero
		err = os.Chtimes(cjDir+"/tmr", time.Time{}, time.Unix(0, 0))
		if err != nil {
			t.Errorf("failed to create change tmr file last modification time for testing\n")
		}
		expected := tests{
			true,
			0,
		}
		var result tests
		result.ready, result.time = CheckCj(&cjDir)
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("values of bool if cronjob is ready and integer unix timestamp did not match expected value\n")
		}
	})
	// create not_ready cronjob and test it
	t.Run("not ready", func(t *testing.T) {
		cjDir := CfgDir + "/not_ready"
		err := os.Mkdir(cjDir, 0755)
		if err != nil {
			t.Errorf("failed to create cronjob directory for testing\n")
		}
		cjTmr, err := os.Create(cjDir + "/tmr")
		if err != nil {
			t.Errorf("failed to create cronjob tmr file for testing\n")
		}
		cjTmr.Close()
		// change last modification time to zero
		cjSchedule := time.Now().Unix() + 60
		err = os.Chtimes(cjDir+"/tmr", time.Time{}, time.Unix(cjSchedule, 0))
		if err != nil {
			t.Errorf("failed to create change tmr file last modification time for testing\n")
		}
		expected := tests{
			false,
			int(time.Now().Unix() + 60),
		}
		var result tests
		result.ready, result.time = CheckCj(&cjDir)
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("values bool if cronjob is ready and integer unix timestamp did not match expected value\n")
		}
	})
}

// test RunCj()
func TestRunCj(t *testing.T) {
	t.Cleanup(func() {
		os.RemoveAll(CfgDir + "/success")
		os.RemoveAll(CfgDir + "/fail")
		os.RemoveAll(CfgDir + "/timeout")
	})
	// create success cronjob and test it
	t.Run("success", func(t *testing.T) {
		cjDir := CfgDir + "/success"
		err := os.Mkdir(cjDir, 0755)
		if err != nil {
			t.Errorf("failed to create cronjob directory for testing\n")
		}
		cjTmr, err := os.Create(cjDir + "/tmr")
		if err != nil {
			t.Errorf("failed to create cronjob tmr file for testing\n")
		}
		cjTmr.Close()
		run := []byte("#!/bin/sh\n\necho 'hello world'\nexit 0\n")
		err = os.WriteFile(cjDir+"/run", run, 0755)
		if err != nil {
			t.Errorf("failed to create run file for testing\n")
		}
		cjTimeout := 10
		result := RunCj(&cjDir, &cjTimeout)
		if result != true {
			t.Errorf("value of bool if cronjob run was successful did not match expected value\n")
		}
	})
	// create fail cronjob and test it
	t.Run("fail", func(t *testing.T) {
		cjDir := CfgDir + "/false"
		err := os.Mkdir(cjDir, 0755)
		if err != nil {
			t.Errorf("failed to create cronjob directory for testing\n")
		}
		cjTmr, err := os.Create(cjDir + "/tmr")
		if err != nil {
			t.Errorf("failed to create cronjob tmr file for testing\n")
		}
		cjTmr.Close()
		run := []byte("#!/bin/sh\n\necho 'hello world'\nexit 1\n")
		err = os.WriteFile(cjDir+"/run", run, 0755)
		if err != nil {
			t.Errorf("failed to create run file for testing\n")
		}
		cjTimeout := 10
		result := RunCj(&cjDir, &cjTimeout)
		if result != false {
			t.Errorf("value of bool if cronjob run was successful did not match expected value\n")
		}
	})
	// create timeout cronjob and test it
	t.Run("timeout", func(t *testing.T) {
		cjDir := CfgDir + "/timeout"
		err := os.Mkdir(cjDir, 0755)
		if err != nil {
			t.Errorf("failed to create cronjob directory for testing\n")
		}
		cjTmr, err := os.Create(cjDir + "/tmr")
		if err != nil {
			t.Errorf("failed to create cronjob tmr file for testing\n")
		}
		cjTmr.Close()
		run := []byte("#!/bin/sh\n\necho 'hello world'\nsleep 2\nexit 0\n")
		err = os.WriteFile(cjDir+"/run", run, 0755)
		if err != nil {
			t.Errorf("failed to create run file for testing\n")
		}
		cjTimeout := 1
		result := RunCj(&cjDir, &cjTimeout)
		if result != false {
			t.Errorf("value of bool if cronjob run was successful did not match expected value\n")
		}
	})
}

// test ScheduleCj()
func TestScheduleCj(t *testing.T) {
	t.Cleanup(func() {
		os.RemoveAll(CfgDir + "/cronjob")
	})
	// instead of creating new cronjobs for each subtest
	// I modify existing environment
	cjDir := CfgDir + "/cronjob"
	err := os.Mkdir(cjDir, 0755)
	if err != nil {
		t.Errorf("failed to create cronjob directory for testing\n")
	}
	// test when there is no file
	t.Run("no file", func(t *testing.T) {
		result := ScheduleCj(&cjDir)
		if result != 0 {
			t.Errorf("value of integer cjSchedule did not match expected value\n")
		}
		if _, err := os.Stat(cjDir + "/tmr"); err == nil {
			t.Errorf("tmr file should not exist\n")
		}
	})
	// test when there is no value
	t.Run("no value", func(t *testing.T) {
		cfg := []byte("\n")
		err = os.WriteFile(cjDir+"/cfg", cfg, 0644)
		if err != nil {
			t.Errorf("failed to create cfg file for testing\n")
		}
		result := ScheduleCj(&cjDir)
		if result != 0 {
			t.Errorf("value of integer cjSchedule did not match expected value\n")
		}
		if _, err := os.Stat(cjDir + "/tmr"); err == nil {
			t.Errorf("tmr file should not exist\n")
		}
	})
	// test when there is invalid value
	t.Run("invalid value", func(t *testing.T) {
		cfg := []byte("0\n")
		err = os.WriteFile(cjDir+"/cfg", cfg, 0644)
		if err != nil {
			t.Errorf("failed to create cfg file for testing\n")
		}
		result := ScheduleCj(&cjDir)
		if result != 0 {
			t.Errorf("value of integer cjSchedule did not match expected value\n")
		}
		if _, err := os.Stat(cjDir + "/tmr"); err == nil {
			t.Errorf("tmr file should not exist\n")
		}
	})
	// test when there is a valid value
	t.Run("valid", func(t *testing.T) {
		cfg := []byte("10\n")
		err = os.WriteFile(cjDir+"/cfg", cfg, 0644)
		if err != nil {
			t.Errorf("failed to create cfg file for testing\n")
		}
		expected := int(time.Now().Unix() + 10)
		result := ScheduleCj(&cjDir)
		if result != expected {
			t.Errorf("value of integer cjSchedule did not match expected value\n")
		}
		cjTmrInfo, err := os.Stat(cjDir + "/tmr")
		if err != nil {
			t.Errorf("failed to get cronjob tmr file info\n")
		}
		if cjTmrInfo.ModTime().Unix() != int64(expected) {
			t.Errorf("last modification time of tmr file did not match expected value\n")
		}
	})
}

// test GetSleepTime()
func TestGetSleepTime(t *testing.T) {
	cfg := CfgType{
		Max:     600,
		Min:     5,
		Timeout: 10,
	}
	// test if there is no cronjobs
	t.Run("no cronjobs", func(t *testing.T) {
		expected := cfg.Max
		cjSchedules := []int{}
		result := GetSleepTime(&cjSchedules, &cfg)
		if result != expected {
			t.Errorf("value of integer sleepTime did not match expected value\n")
		}
	})
	// test if schedule does not exceed limits
	t.Run("does not exceed limits", func(t *testing.T) {
		expected := 6
		curTime := int(time.Now().Unix())
		cjSchedules := []int{curTime + 31, curTime + 6, curTime + 84}
		result := GetSleepTime(&cjSchedules, &cfg)
		if result != expected {
			t.Errorf("value of integer sleepTime did not match expected value\n")
		}
	})
	// test if schedule exceeds higher limit
	t.Run("exceeds higher limit", func(t *testing.T) {
		expected := cfg.Max
		curTime := int(time.Now().Unix())
		cjSchedules := []int{curTime + 631, curTime + 606, curTime + 684}
		result := GetSleepTime(&cjSchedules, &cfg)
		if result != expected {
			t.Errorf("value of integer sleepTime did not match expected value\n")
		}
	})
	// test if schedule exceeds lower limit
	t.Run("exceeds lower limit", func(t *testing.T) {
		expected := 5
		curTime := int(time.Now().Unix())
		cjSchedules := []int{curTime + 31, curTime + 4, curTime + 84}
		result := GetSleepTime(&cjSchedules, &cfg)
		if result != expected {
			t.Errorf("value of integer sleepTime did not match expected value\n")
		}
	})
}
