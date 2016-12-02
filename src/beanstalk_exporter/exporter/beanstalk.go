package exporter

import (
	"log"
	"sync"
	"time"
	"strconv"

	"github.com/kr/beanstalk"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
)

// how to export:
// metrics:
// cmds_total{addr="127.0.0.1:11300", cmd="delete"}
// tube_cmds_total{addr="127.0.0.1:11300", tube="default", cmd="delete"}
// current_job{addr="127.0.0.1:11300", status="ready"}
// tube_current_job{addr="127.0.0.1:11300", tube="default", state="ready"}

const (
	gaugeType = iota
	counterType
)

type metricConfig struct {
	Name string
	Help string
	Type uint8
	Labels []string
}

type statsDef struct {
	IsTube bool
	Type uint8
	Names []string
}

func (def *statsDef) InitStaticConfig() {
	for _, name := range def.Names {
		var nameMap map[string]string
		if def.IsTube {
			nameMap = tubeMetricNameMap
		} else {
			nameMap = serverMetricNameMap
		}
		metricName, exist := nameMap[name]
		if !exist {
			metricName = strings.Replace(name, "-", "_", -1)
			if def.IsTube {
				metricName = "tube_" + metricName
			}
			nameMap[name] = metricName
		}
		labels := []string{"addr"}
		if def.IsTube {
			labels = append(labels, "tube")
		}
		_, ok := metricConfigs[metricName]
		if !ok {
			metricConfigs[metricName] = metricConfig{
				Name: metricName, Help: name, Type: def.Type,
				Labels: labels,
			}
		}
	}
}

func (def *statsDef) GetMetricName(statsName string) (string, bool) {
	if def.IsTube {
		r, ok := tubeMetricNameMap[statsName]
		return r, ok
	} else {
		r, ok := serverMetricNameMap[statsName]
		return r, ok
	}
}


const (
	currentJobPrefix = "current-jobs-"
	cmdPrefix = "cmd-"
)

var (
	jobStates = []string{"buried", "delayed", "ready", "reserved", "urgent"}
	serverCmds = []string{
		"put", "peek", "peek-ready", "peek-delayed", "peek-buried", "reserve",
		"reserve-with-timeout", "delete", "release", "use", "watch", "ignore",
		"bury", "kick", "touch", "stats", "stats-job", "stats-tube",
		"list-tubes", "list-tube-used", "list-tubes-watched", "pause-tube",
	}
	tubeCmds = []string{"delete", "pause-tube"}
	serverCounterStats = statsDef{
		IsTube: false, Type: counterType,
		Names: []string{
			"job-timeouts", "total-jobs", "total-connections",
			"uptime", "binlog-records-migrated", "binlog-records-written",
		},
	}
	tubeCounterStats = statsDef{
		IsTube: true, Type: counterType,
		Names: []string{"total-jobs"},
	}
	serverGaugeStats = statsDef{
		IsTube: false, Type: gaugeType,
		Names: []string{
			"max-job-size", "current-tubes", "current-connections",
			"current-producers", "current-workers", "current-waiting",
			"binlog-oldest-index", "binlog-current-index",
			"binlog-max-size",
		},
	}
	tubeGaugeStats = statsDef{
		IsTube: true, Type: gaugeType,
		Names: []string{
			"current-using", "current-watching", "current-waiting",
			"pause", "pause-time-left",
		},
	}
	allStats = []*statsDef{
		&serverCounterStats, &serverGaugeStats, &tubeCounterStats, &tubeGaugeStats,
	}
	// map from beanstalkd stats names to prometheus metric names
	// generated when InitStaticConfigs is called
	serverMetricNameMap = map[string]string{
		// server counter
		"job-timeouts": "job_timeouts_total", "total-jobs": "jobs_total",
		"uptime": "uptime_seconds", "total-connections": "connections_total",
		"binlog-records-migrated": "binlog_records_migrated_total",
		"binlog-records-written": "binlog_records_written_total",
		// server gauge
		"max-job-size": "max_job_size_bytes",
		"binlog-max-size": "binlog_max_size_bytes",
	}
	tubeMetricNameMap = map[string]string {
		// tube counter
		"total-jobs": "tube_jobs_tatal",
		// tube gauge
		"pause-time-left": "tube_pause_time_left_seconds",
	}
	metricConfigs = map[string]metricConfig {
		"current_job": {
			Name: "current_job",
			Help: "current job numbers for the beanstalkd server",
			Type: gaugeType, Labels: []string{"addr", "state"},
		},
		"tube_current_job": {
			Name: "tube_current_job",
			Help: "current job numbers in the given tube",
			Type: gaugeType, Labels: []string{"addr", "tube", "state"},
		},
		"cmds_total": {
			Name: "cmds_total",
			Help: "total commands executed for the beanstalkd server",
			Type: counterType, Labels: []string{"addr", "cmd"},
		},
		"tube_cmds_total": {
			Name: "tube_cmds_total",
			Help: "total commands executed for the given tube",
			Type: counterType, Labels: []string{"addr", "tube", "cmd"},
		},
	}
	staticConfigInitialized = sync.Once{}
)


func labelExtractor(name string, labels map[string]string) {
	if strings.HasPrefix(name, cmdPrefix) {
		cmd := strings.TrimPrefix(name, cmdPrefix)
		labels["cmd"] = cmd
	} else if strings.HasPrefix(name, currentJobPrefix) {
		state := strings.TrimPrefix(name, currentJobPrefix)
		labels["state"] = state
	}
}

type Exporter struct {
	beanstalkd []string
	namespace string
	// a list of tubes to watch
	tubes []string
	// the time spent on the last scrape
	duration prometheus.Gauge
	// current total scrape
	totalScrapes prometheus.Counter
	// the error status of the last scrape
	scrapeErrors prometheus.Gauge

	// all gauge metrics.
	gauges map[string]*prometheus.GaugeVec
	// all counter metrics.
	counters map[string]*prometheus.CounterVec

	gaugesMtx sync.RWMutex
	countersMtx sync.RWMutex
	sync.RWMutex
}


func InitStaticConfigs() {
	initializer := func() {
		for _, cmd := range serverCmds {
			key := cmdPrefix + cmd
			serverCounterStats.Names = append(serverCounterStats.Names, key)
			serverMetricNameMap[key] = "cmds_total"
		}
		for _, cmd := range tubeCmds {
			key := cmdPrefix + cmd
			tubeCounterStats.Names = append(tubeCounterStats.Names, key)
			tubeMetricNameMap[key] = "tube_cmds_total"
		}
		for _, state := range jobStates {
			key := currentJobPrefix + state
			serverMetricNameMap[key] = "current_job"
			tubeMetricNameMap[key] = "tube_current_job"
			serverGaugeStats.Names = append(serverGaugeStats.Names, key)
			tubeGaugeStats.Names = append(tubeGaugeStats.Names, key)
		}
		for _, def := range allStats {
			def.InitStaticConfig()
		}
	}
	staticConfigInitialized.Do(initializer)
}


// assume addrs are all valid beanstalkd addresses
func NewExporter(addrs []string, namespace string, tubes []string) (*Exporter, error) {
	e := Exporter{
		beanstalkd: addrs,
		tubes: tubes,
		namespace: namespace,

		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name: "exporter_last_scrape_duration_seconds",
			Help: "The last scrape duration",
		}),

		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name: "exporter_scrapes_total",
			Help: "Current total beanstalkd scrapes",
		}),

		scrapeErrors: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name: "exporter_last_scrape_error",
			Help: "The last scrape error status.",
		}),
	}

	e.initGauges()
	e.initCounters()
	return &e, nil
}

func (e *Exporter) initGauges() {
	e.gauges = map[string]*prometheus.GaugeVec{}
	for _, cfg := range metricConfigs {
		if cfg.Type == gaugeType {
			metric := createGaugeVecFromConfig(e.namespace, cfg)
			e.gauges[cfg.Name] = metric
		}
	}
}

func (e *Exporter) initCounters() {
	e.counters = map[string]*prometheus.CounterVec{}

	for _, cfg := range metricConfigs {
		if cfg.Type == counterType {
			metric := createCounterVecFromConfig(e.namespace, cfg)
			e.counters[cfg.Name] = metric
		}
	}
}

type scrapeResult struct {
	Name string
	Value float64
	Type uint8
	Labels prometheus.Labels
}


func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range e.gauges {
		m.Describe(ch)
	}
	for _, m := range e.counters {
		m.Describe(ch)
	}

	e.totalScrapes.Describe(ch)
	e.scrapeErrors.Describe(ch)
	e.duration.Describe(ch)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	scrapes := make(chan scrapeResult)
	e.Lock()
	defer e.Unlock()

	e.initGauges()

	go e.scrape(scrapes)
	e.setMetrics(scrapes)

	e.collectGauges(ch)
	e.collectCounters(ch)
}

func (e *Exporter) collectGauges(ch chan<- prometheus.Metric) {
	for _, metric := range e.gauges {
		metric.Collect(ch)
	}

}

func (e *Exporter) collectCounters(ch chan<- prometheus.Metric) {
	for _, metric := range e.counters {
		metric.Collect(ch)
	}

}

func (e *Exporter) setMetrics(scrapes <-chan scrapeResult) {
	for scr := range scrapes {
		if scr.Type == gaugeType {
			e.setForGaugeMetric(scr)
			// a result for server
		} else {
			// a result for given tube
			e.setForCounterMetric(scr)
		}
	}
}

func (e *Exporter) setForGaugeMetric(result scrapeResult) {
	var metric *prometheus.GaugeVec
	metricName := result.Name
	// here we don't lock the first read
	// because it would not cause many problems if
	// we just lose 1 result.
	metric, ok := e.gauges[metricName]
	if !ok {
		cfg, exist := metricConfigs[metricName]
		if !exist {
			return
		}
		e.gaugesMtx.Lock()
		_, ok := e.gauges[metricName]
		if !ok {
			metric = createGaugeVecFromConfig(e.namespace, cfg)
			e.gauges[metricName] = metric
		}
		e.gaugesMtx.Unlock()
	}
	if metric == nil {
		return
	}
	metric.With(result.Labels).Set(result.Value)
}


func (e *Exporter) setForCounterMetric(result scrapeResult) {
	var metric *prometheus.CounterVec
	metric, ok := e.counters[result.Name]

	if !ok {
		cfg, exist := metricConfigs[result.Name]
		if !exist {
			return
		}
		e.countersMtx.Lock()
		_, ok := e.counters[result.Name]
		if !ok {
			metric = createCounterVecFromConfig(e.namespace, cfg)
			e.counters[result.Name] = metric
		}
		e.countersMtx.Unlock()
	}

	if metric == nil {
		return
	}
	metric.With(result.Labels).Set(result.Value)
}

func (e *Exporter) scrape(scrapes chan<- scrapeResult) {
	defer close(scrapes)

	start := time.Now().UnixNano()

	e.totalScrapes.Inc()
	errCount := 0

	proto := "tcp"
	for _, addr := range e.beanstalkd {
		conn, err := beanstalk.Dial(proto, addr)
		if err != nil {
			log.Panicf("Error while connecting to %s: %s", addr, err)
			errCount++
			continue
		}
		defer conn.Close()

		stats, err := conn.Stats()
		if err != nil {
			log.Printf("Error while fetching stats: %s", err)
			errCount++
			continue
		}
		e.extractStats(stats, addr, "", scrapes)

		for _, tubeName := range e.tubes {
			tube := &beanstalk.Tube{Conn: conn, Name: tubeName}
			stats, err = tube.Stats()
			if err != nil {
				log.Printf("Error while fetching stats for tube %s: %s", tubeName, err)
				errCount++
				continue
			}
			e.extractStats(stats, addr, tubeName, scrapes)
		}
	}

	e.scrapeErrors.Set(float64(errCount))
	e.duration.Set(float64(time.Now().UnixNano() - start) / 10e9)
}

func (e *Exporter) extractStats(
		stats map[string]string, addr string, tube string, ch chan<- scrapeResult) {
	var statsDefs []statsDef
	if tube == "" {
		statsDefs = []statsDef{serverGaugeStats, serverCounterStats}
	} else {
		statsDefs = []statsDef{tubeGaugeStats, tubeCounterStats}
	}
	for _, def := range statsDefs {
		for _, statsName := range def.Names {
			metricName, ok := def.GetMetricName(statsName)
			if !ok {
				log.Fatalf("%s has no correspound metric name.\n", statsName)
				continue
			}
			valueStr, ok := stats[statsName]
			if !ok {
				log.Fatalf("%s does not exist in beanstalkd server stats.\n", statsName)
				continue
			}
			value, err := strconv.ParseFloat(valueStr, 64)
			if err != nil {
				log.Fatalf(
					"Error while convert value of %s to float, error: %s\n",
					statsName, err,
				)
				continue
			}
			labels := map[string]string{"addr": addr}
			if tube != "" {
				labels["tube"] = tube
			}
			labelExtractor(statsName, labels)
			ch <- scrapeResult{
				Name: metricName, Value: value, Type: def.Type,
				Labels: labels,
			}
		}
	}
}

func createGaugeVecFromConfig(namespace string, cfg metricConfig) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name: cfg.Name,
		Help: cfg.Help,
	}, cfg.Labels)
}

func createCounterVecFromConfig(namespace string, cfg metricConfig) *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name: cfg.Name,
		Help: cfg.Help,
	}, cfg.Labels)
}
