package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	//"github.com/kr/beanstalk"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"strings"
	//"beanstalk_exporter/exporter"
	"strconv"
)

const (
	BEANSTALKD_DEFAULT_PORT = 11300
)

var (
	addrArgHelp = "Addresses of one or more beanstalkd servers, separated by comma.\n" +
		"    \tIf port is not specified, the default port " +
		strconv.Itoa(BEANSTALKD_DEFAULT_PORT) + " is used.\n" +
		"    \tExamples: localhost,192.168.50.4:11300"
	addrInArgs = flag.String("addr", "localhost:11300", addrArgHelp)
	namespace = flag.String("namespace", "beanstalkd",
		"Namespace for metrics")
	tubeNames = flag.String("tube.names", "",
		"The list of tube names separated by tubes.sep to export metrics")
	tubeSep = flag.String("tube.sep", ",", "The separator of tube names")
	listenAddr = flag.String("web.listen-address", ":12300", "The address exporter listens to")
	metricPath = flag.String("web.telemetry-path", "/metrics", "")
	showVersion = flag.Bool("version", false, "Print exporter version")
	showVersionShort = flag.Bool("v", false, "")

	VERSION = "<<< filled by build >>>"
	BUILD_DATE = "<<< filled by build >>>"
	COMMIT_SHA1 = "<<< filled by build >>>"
)

func indexPageHandler(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte(`<html>
<head><title>Beanstalk Exporter</title></head>
<body>
<h1>Beanstalk Exporter</h1>
<p>version: ` + VERSION + `</p>
<p><a href="` + *metricPath + `">Metrics</a></p>
</body>
</html>`))
}

func normalizeAddrs(addrs []string) ([]string, error) {
	normalized := make([]string, len(addrs))
	for _, addr := range addrs {
		parts := strings.Split(addr, ":")
		if len(parts) == 1 {
			normalized = append(normalized, parts[0] + strconv.Itoa(BEANSTALKD_DEFAULT_PORT))
		} else if len(parts) == 2 {
			normalized = append(normalized, addr)
		} else {
			err := fmt.Errorf("Invalid parameter for -addr: %s", addr)
			return nil, err
		}
	}
	return normalized, nil
}

func main() {
	flag.Parse()
	log.Printf(
		"Beanstalk Metrics Exporter %s:\n  build date: %s\n  sha1: %s\n\n",
		VERSION, BUILD_DATE, COMMIT_SHA1,
	)
	if *showVersion || *showVersionShort {
		os.Exit(0)
	}

	addrs := strings.Split(*addrInArgs, ",")
	if len(addrs) == 0 {
		log.Fatal("At least one beanstalkd server address must be specified.")
		os.Exit(1)
	}
	addrs, err := normalizeAddrs(addrs)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	tubes := strings.Split(*tubeNames, *tubeSep)
	fmt.Printf("tubes: %s\n", tubes)

	//e, err := exporter.NewExporter(addrs, *namespace, tubes)
	//fmt.Println(e)

	cpuTemp := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cpu_temp_celsius",
		Help: "Current CPU Temp",
	})
	hdFailure := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hd_errors_total",
			Help: "Number of hard disk failure"},
		[]string{"device"},
	)
	prometheus.MustRegister(cpuTemp)
	prometheus.MustRegister(hdFailure)
	fmt.Printf("time: %s\n", time.Now().UnixNano())

	cpuTemp.Set(65.3)
	hdFailure.With(prometheus.Labels{"device": "/dev/sda"}).Inc()
	http.HandleFunc("/", indexPageHandler)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
