package exporter

import (
	"log"
	"sync"
	"time"
	"strconv"
	"strings"

	"github.com/kr/beanstalk"
	"github.com/prometheus/client_golang/prometheus"
)

// how to export:
// metrics:
// cmds_total{addr="127.0.0.1:11300", cmd="delete"}
// tube_cmds_total{addr="127.0.0.1:11300", tube="default", cmd="delete"}
// current_job{addr="127.0.0.1:11300", status="ready"}
// tube_current_job{addr="127.0.0.1:11300", tube="default", state="ready"}


type metricConfig struct {
	Name string
	Help string
	Type uint8
	Labels []string
}

const (
	gaugeType = iota
	counterType
)

const (
	jobStatus = []string{"buried", "delayed", "ready", "reserved", "urgent"}
	serverCmds = []string{}
	tubeCmds = []string{}
	currentJobNames = []string{
		"current-jobs-urgent", "current-jobs-ready",
		"current-jobs-reserved", "current-jobs-delayed",
		"current-jobs-buried",
	}
	currentJobPrefix = "current-jobs-"
	metricConfigs = map[string]metricConfig {
		"current_job": {
			Name: "current_job", Help: "",
			Type: gaugeType, Labels: {"addr", "state"},
		},
		"tube_current_job": {
			Name: "tube_current_job", Help: "",
			Type: gaugeType, Labels: {"addr", "tube", "state"},
		},
		"cmds_total": {
			Name: "cmds_total", Help: "",
			Type: counterType, Labels: {"addr", "cmd"},
		},
		"tube_cmds_total": {
			Name: "tube_cmds_total", Help: "",
			Type: counterType, Labels: {"addr", "tube", "cmd"},
		},
	}
)

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

//
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
	return e, nil
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
			log.Printf("beanstalkd error: %s", err)
			errCount++
			continue
		}
		defer conn.Close()

		stats, err := conn.Stats()
		if err == nil {
			err = e.extractFromStats(stats, addr, scrapes)
		}
		if err != nil {
			log.Printf("beanstalkd error: %s", err)
			errCount++
			continue
		}

		for _, tubeName := range e.tubes {
			tube := &beanstalk.Tube{Conn: conn, Name: tubeName}
			stats, err = tube.Stats()
			if err == nil {
				err = e.extractFromTubeStats(stats, addr, tubeName, scrapes)
			}
			if err != nil {
				log.Printf("beanstalkd error: %s", err)
				errCount++
				continue
			}
		}
	}

	e.scrapeErrors.Set(float64(errCount))
	e.duration.Set(float64(time.Now().UnixNano() - start) / 10e9)
}

func (e *Exporter) extractFromStats(stats map[string]string, addr string, ch chan<- scrapeResult) error {
	var err error
	err = nil
	return nil
}

func (e *Exporter) extractFromTubeStats(
		stats map[string]string, addr string, tube string, ch chan<- scrapeResult) error {
	return nil
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
