package data

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
