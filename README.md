# Awesome Cron .go

## Introduction

Awecron is a small and simple custom cron written in Go. The aim of this project is to create a minimal cron with a special scheduling design for desktop / laptop users.

Originally awecron was written in POSIX shell script and it still can be found in [awecron.sh repository](https://github.com/life00/awecron.sh), however users are encouraged to use the awecron implementation in Go instead as it is arguably more suitable for the application, has better error checks and appears to have better performance. It is _fully_^[Except the global awecron config [./cfg](./cfg) in awecron.go is a TOML file, while in awecron.sh it is a shell script file. This however is not a problem because as long as you keep it simple like in the example provided (i.e. only have static configuration) it is cross compatible. This is the reason why TOML was chosen for awecron.go.] cross compatible with the existing awecron.sh configuration.

Most of the following documentation was taken from existing [awecron.sh repository](https://github.com/life00/awecron.sh).

### Features

- uses the [special cronjob scheduling design](#scheduling-design) for desktop / laptop users
- very minimal and small
- portability
  - it is possible to build and run awecron on any linux distribution
  - it may be possible to build and run awecron.go on FreeBSD, OpenBSD, and macOS, however it is not officially supported
- runs cronjobs concurrently
- always performs data validation checks and proper error handling
- has timeout feature
  - will force stop a cronjob if it exceeds the set time limit
- has dynamic sleep feature
  - sleeps for the exact time needed until next cronjob
- if cronjob errors then it is automatically disabled

## Installation

### Compatibility

Should work on any POSIX compliant system. Tested on:

- Alpine Linux
- Fedora Linux

### Setup

1. clone the repo: `git clone https://github.com/life00/awecron`
2. delete all unnecessary files: `rm -rfv ./awecron/.git`
   - you may also remove the rest later
3. compile awecron and move the binary: `go build ./awecron/awecron.go; mv ./awecron /usr/local/bin/`
4. move the config directory to an appropriate location: `mv ./awecron /etc/`
5. ensure that the permissions are set appropriately: `chown root:root /usr/local/bin/awecron /etc/awecron; ...`
6. verify validity of all files
7. configure awecron (see [configuring awecron section](#configuring-awecron))
8. run awecron (see [running awecron section](#running-awecron))

#### Configuring awecron

When the awecron is run it first tries to check if the configuration directory is in `$XDG_CONFIG_DIR/awecron/` or `$HOME/.config/awecron/`, then it checks the global configuration in `/etc/awecron/`. The former should be used when running awecron as non root user (see below).

The global configuration of awecron is a TOML file located in [./cfg](./cfg). See the comments there for details.

There is a simple cronjob configuration example in [./ex/](./ex/). It includes the following files:

- `run` is a binary or a shell script that supposed to run
- `tmr` is an empty file; its last modification time is used to determine the next run time of a cronjob
  - without the file awecron will ignore the directory
- `cfg` contains the interval the cronjob should run at

#### Running awecron

You may use the following simple examples of init service configuration for awecron (see [./sf/](./sf/) directory):

- [OpenRC](./sf/openrc/awecron)
- [runit](./sf/runit/)
- [systemd](./sf/systemd/awecron.service)

Awecron runs all the cronjobs as its current user. As previously mentioned it is possible to have the configuration of awecron in the local user environment. This way you may have multiple instances of awecron running as different users without interference.

## Design choices

### Scheduling design

As it was already stated the design has desktop / laptop users in mind. The problem with these platforms may be that they could be offline most of the time, and as the result the cronjob schedules are inconsistent and may be missed regularly. When using crontab the issue may be that the cronjob is skipped at that specific time (e.g. 12:00) because the device could be offline. In these cases anacron is suggested, however it still has a similar problem that the next scheduled time might be the time when the device will be offline, thus skipping the cronjob.

Similarly to anacron, awecron also periodically runs cronjobs, however awecron solves the above-mentioned problem by running skipped cronjobs as soon as possible instead of waiting for the next scheduled time, then rescheduling them based on the interval. This is the key difference of awecron and why I believe it is most suitable for desktop / laptop users.

### Implementation design

I have initially chosen to write it in POSIX shell script as a way to improve my shell script knowledge, and similarly I have written it in Go to learn Go. Awecron was strongly inspired by the runit init system and its design choices. It is similarly very minimal and has similar features of handling runtime resources and configuration files.

Awecron program will first determine its config directory where all the runtime files and configuration are stored. Then it will run through all directories in the config directory which contain `tmr` file (`cfgDir/*/tmr`) which it assumes are cronjobs.

All cronjobs run concurrently as separate goroutines. Awecron checks if it is necessary to run the cronjob and if yes then it runs the executable (`cfgDir/*/run`) and ensures the cronjob does not exceed the time limit (configured in global awecron config).

After the cronjob is successful the next run time is calculated from the cronjob interval configuration (`cfgDir/*/cfg`) and saved as last modification time of `tmr` file (`cfgDir/*/tmr`).

In case the cronjob fails the `tmr` file is not created and so the cronjob is disabled. This may especially be useful when manually disabling a cronjob (e.g. `rm "cfgDir/ex/tmr"`) or making it run as soon as possible (e.g. `touch "cfgDir/ex/tmr"; systemctl restart awecron`).

When a cronjob is run the appropriate logs are outputed containing the user that runs awecron (and the cronjob), type of log, name of the cronjob (directory), exit code, and log message.

Afterwards, awecron determines the necessary amount of time to sleep until the next cronjob. This allows awecron to be very efficient and mostly be in the background. It is possible to also configure the maximum and minimum time limits of sleep in global awecron config. This may be useful to reduce possible overhead of small interval cronjobs (minumum limit) and make awecron check for newly added or updated cronjobs more frequently (maximum limit). In between sleep intervals awecron runs and checks all the necessary cronjobs and reevaluates its next optimal sleep time.

The whole program is implemented with error safety in mind. All data is validated and errors are handled properly. There are three types of logs that could be outputted. Logs containing INFO tag indicate important events that went as planned without errors. Logs containing ERROR tag indicate that there was a non-fatal error likely due to misconfiguration of a cronjob which will be disable, and such an error does not impact the functionality of the whole cron. Logs containing FATAL tag indicate that there was a fatal error that may impact the functionality of the whole cron which will result in awecron exiting.

## Additional notes

### The approximation bug (or feature?)

While rewriting awecron in Go I realized one fundamental problem with the implementation of scheduling cronjobs. This applies to both awecron.sh and awecron.go. The cronjob is scheduled for next run only after it completes its task. This makes sense, however it creates a problem later. Dynamic sleep will choose to awake on the closest cronjob, and those closest cronjobs will run if their timer is less than current time.

The problem emerges when considering for example 3 cronjobs completing their task at 12:00:00 while last cronjob completed its task at 12:00:02 because of resource usage overload. They were scheduled to run next time at 12:10:00 and 12:10:02 respectively. When the time is reached only the first 3 cronjobs run, while the last cronjob is left to run after the minimum sleep interval at 12:10:07. This minimum sleep time (5 seconds) delay will continue until the system shuts down and scheduled time for all cronjobs will be missed, and the cycle repeats.

This behavior is especially noticeable when parallelism / concurrency is disabled because all the cronjobs are slowly completed in order and have higher chance to add up to more than 1 second. This might be considered an undesired behavior and that is what I thought initially. However, after thinking about this for a while I realized that it might be a good throttling mechanism that dynamically separates cronjobs into clusters by groups of cronjobs that successfully complete their task within a second. This may allow avoiding significant resource usage on low-end systems or when there are many demanding cronjobs.

It appears that this bug may actually be considered as a (previously) _unknown feature_. I am not planning to change anything, and I am satisfied with current scheduling design.

## To-Do
