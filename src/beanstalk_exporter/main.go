package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"beanstalk_exporter/exporter"
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
	metricPath = flag.String("web.telemetry-path", "/metrics", "the path of metric info")
	showVersion = flag.Bool("version", false, "Print exporter version")
	showVersionShort = flag.Bool("v", false, "")

	VERSION = "<<< filled by build >>>"
	BUILD_DATE = "<<< filled by build >>>"
	REVISION = "<<< filled by build >>>"
	GIT_BRANCH = "<<< filled by build >>>"
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
	normalized := make([]string, 0)
	for _, addr := range addrs {
		parts := strings.Split(addr, ":")
		if len(parts) == 1 {
			normalized = append(normalized, parts[0] + ":" + strconv.Itoa(BEANSTALKD_DEFAULT_PORT))
		} else if len(parts) == 2 {
			normalized = append(normalized, addr)
		} else {
			err := fmt.Errorf("Invalid parameter for -addr: %s", addr)
			return nil, err
		}
	}
	return normalized, nil
}

func removeEmptyString(array []string) []string {
	newArray := make([]string, 0)
	for _, str := range array {
		if len(str) != 0 {
			newArray = append(newArray, str)
		}
	}
	return newArray
}

func main() {
	flag.Parse()
	fmt.Printf(
		"Beanstalk Metrics Exporter %s:\n  build date: %s\n  revision: %s\n\n",
		VERSION, BUILD_DATE, REVISION,
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
	tubes := removeEmptyString(strings.Split(*tubeNames, *tubeSep))
	log.Printf("addrs: %v\n", addrs)
	log.Printf("tubes: %v\n", tubes)


	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "beanstalk_exporter_build_info",
		Help: "beanstalk_exporter_build_info",
	}, []string{"version", "revision", "goversion", "branch", "build_date"})

	prometheus.MustRegister(buildInfo)
	buildInfo.WithLabelValues(
		VERSION, REVISION, runtime.Version(), GIT_BRANCH, BUILD_DATE)
	e, err := exporter.NewExporter(addrs, *namespace, tubes)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	prometheus.MustRegister(e)

	log.Printf("Providing metrics at %s%s", *listenAddr, *metricPath)

	http.HandleFunc("/", indexPageHandler)
	http.Handle(*metricPath, promhttp.Handler())
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
