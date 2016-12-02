# Beanstalkd metrics

### server metrics

comes from `stats` command

should we export pid, version, id and
hostname??
IDK

~~~
pid: 8
version: 1.10
rusage-utime: 0.008000
rusage-stime: 0.004000
id: 442c3147ec0f4534
hostname: 4e9b225dbad3
~~~

done:

~~~
uptime: 79896
binlog-oldest-index: 0
binlog-current-index: 0
binlog-records-migrated: 0
binlog-records-written: 0
binlog-max-size: 10485760
job-timeouts: 0
total-jobs: 1
max-job-size: 65535
current-tubes: 1
current-connections: 2
current-producers: 1
current-workers: 0
current-waiting: 0
total-connections: 26
current-jobs-urgent: 0
current-jobs-ready: 1
current-jobs-reserved: 0
current-jobs-delayed: 0
current-jobs-buried: 0
cmd-put: 1
cmd-peek: 2
cmd-peek-ready: 0
cmd-peek-delayed: 0
cmd-peek-buried: 0
cmd-reserve: 0
cmd-reserve-with-timeout: 0
cmd-delete: 0
cmd-release: 0
cmd-use: 0
cmd-watch: 0
cmd-ignore: 0
cmd-bury: 0
cmd-kick: 0
cmd-touch: 0
cmd-stats: 15
cmd-stats-job: 3
cmd-stats-tube: 10
cmd-list-tubes: 1
cmd-list-tube-used: 0
cmd-list-tubes-watched: 0
cmd-pause-tube: 0
~~~

### job metrics (no intention to support yet)

comes from `stats-job <id>` command

~~~
id: 1
tube: default
state: ready
pri: 2147483648
age: 76363
delay: 0
ttr: 120
time-left: 0
file: 0
reserves: 0
timeouts: 0
releases: 0
buries: 0
kicks: 0
~~~

### tube metrics

comes from `stats-tube <name>` command

should we export name????
no
should we put `current-*` into label?
i think no...

~~~py
name: default
~~~

done:

~~~
total-jobs: 1
current-using: 2
current-watching: 2
current-waiting: 0
pause: 0
pause-time-left: 0
current-jobs-urgent: 0
current-jobs-ready: 1
current-jobs-reserved: 0
current-jobs-delayed: 0
current-jobs-buried: 0
cmd-delete: 0
cmd-pause-tube: 0
~~~
