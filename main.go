package main

/* sensor_exporter is a generic prometheus exporter for different sensors,
 * ensuring common metics and labels, replacing individual exporters used
 * before.
 *
 * Copyright 2021 Andreas Krebs
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sberk42/sensor_exporter/sensors"
	log "github.com/sirupsen/logrus"
)

var (
	flagConfigFile = flag.String("config-file", "sensor_exporter.json", "The JSON file with the metric definitions.")
	flagRtl433Path = flag.String("rtl433-path", "rtl_433", "Path to rtl_433 binary.")
	flagAddr       = flag.String("listen-address", "127.0.0.1:9043", "The address to listen on for HTTP requests.")
	flagLogLevel   = flag.String("log-level", "info", "The log level {trace|debug|info|warn|error}")
)

var metricDescs map[sensors.MeasurementType]*prometheus.Desc
var metricTypes map[sensors.MeasurementType]prometheus.ValueType
var metricLabels []string
var configLabelIndex map[string]int

type SensorCollector struct {
	sensorDevices []sensors.SensorDevice
}

// Config to add constant labels to
type sensorConfig struct {
	DeviceType   string            `json:"deviceType"`
	DeviceId     string            `json:"deviceId"`
	DeviceVendor string            `json:"deviceVendor"`
	DeviceName   string            `json:"deviceName"`
	SensorModel  string            `json:"sensorModel"`
	SensorId     string            `json:"sensorId"`
	Labels       map[string]string `json:"labels"`
	idFields     []string
}

var sensorConfigs []*sensorConfig

// Implement prometheus Collector
func (sc *SensorCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, md := range metricDescs {
		ch <- md
	}
}

func labelsMatchConfig(sc *sensorConfig, labels []string) bool {

	for i, l := range sc.idFields {
		if l != "" && l != labels[i] {
			return false
		}
	}

	return true
}

func createMeasurementLabels(dev sensors.SensorDevice, m *sensors.Measurement) []string {
	// create static labels from device and measurement
	labels := make([]string, len(metricLabels))
	labels[0] = dev.DeviceType()
	labels[1] = dev.DeviceId()
	labels[2] = dev.DeviceVendor()
	labels[3] = dev.DeviceName()
	labels[4] = m.SensorModel
	labels[5] = m.SensorId

	// get labels from config
	for _, sc := range sensorConfigs {
		if labelsMatchConfig(sc, labels) {
			// add static labels
			for l, v := range sc.Labels {
				labels[configLabelIndex[l]] = v
			}

			break
		}
	}

	return labels
}

func (sc *SensorCollector) Collect(ch chan<- prometheus.Metric) {
	for _, sd := range sc.sensorDevices {
		ms := sd.GetMeasurements()

		for _, m := range ms {
			md := metricDescs[m.Type]
			vt := metricTypes[m.Type]

			labels := createMeasurementLabels(sd, &m)

			metric, err := prometheus.NewConstMetric(md, vt, m.Value, labels...)
			if err != nil {
				log.Errorf("Error creating metric %s", err)
			} else {
				ch <- metric
			}
		}
	}
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}

	return -1
}

func createMetricsDescs() {
	mtypes := sensors.GetAllMeasurementTypes()

	metricDescs = make(map[sensors.MeasurementType]*prometheus.Desc, len(mtypes))
	metricTypes = make(map[sensors.MeasurementType]prometheus.ValueType, len(mtypes))
	configLabelIndex = make(map[string]int)

	metricLabels = []string{"device_type", "device_id", "device_vendor", "device_name", "sensor_model", "sensor_id"}
	for i, l := range metricLabels {
		configLabelIndex[l] = i
	}

	// append constant labels from config and fill idFields
	for _, sc := range sensorConfigs {
		sc.idFields = []string{sc.DeviceType, sc.DeviceId, sc.DeviceVendor, sc.DeviceName, sc.SensorModel, sc.SensorId}

		log.Debugf("init sc to %v", sc)

		for lbl := range sc.Labels {
			if indexOf(metricLabels, lbl) == -1 {
				configLabelIndex[lbl] = len(metricLabels)
				metricLabels = append(metricLabels, lbl)
			}
		}
	}

	log.Debugf("metric labels: %v", metricLabels)
	log.Debugf("configLabelIndex: %v", configLabelIndex)

	for _, mt := range mtypes {
		mDetails := sensors.GetMeasurementTypeDetails(mt)

		metricDescs[mt] = prometheus.NewDesc("sensor_measurement_"+mDetails.MetricName, mDetails.MetricHelp, metricLabels, nil)

		var vt prometheus.ValueType

		if mDetails.MetricValue == sensors.COUNTER {
			vt = prometheus.CounterValue
		} else if mDetails.MetricValue == sensors.GAUGE {
			vt = prometheus.GaugeValue
		} else {
			vt = prometheus.UntypedValue
		}

		metricTypes[mt] = vt
	}
}

func init() {
	flag.Parse()

	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})

	logLevel, err := log.ParseLevel(*flagLogLevel)
	if err != nil {
		log.Errorf("error parsing log level:", err)
	} else {
		log.SetLevel(logLevel)
	}
}

func main() {
	// read metrics
	jsonData, err := ioutil.ReadFile(*flagConfigFile)
	if err != nil {
		log.Fatalf("error reading config file:", err)
	}

	err = json.Unmarshal(jsonData, &sensorConfigs)
	if err != nil {
		log.Fatalf("error parsing JSON:", err)
	}

	createMetricsDescs()

	var sds []sensors.SensorDevice
	rtlDev, err := sensors.InitSensor_rtl433(*flagRtl433Path)
	if err != nil {
		log.Errorf("cannot init sensor: %s", err)
	} else {
		log.Infof("init done: %s, %s", rtlDev.DeviceType(), rtlDev.DeviceId())

		sds = append(sds, rtlDev)
	}

	zyTempDev, err := sensors.InitSensor_zytemp()
	if err != nil {
		log.Errorf("cannot init sensor: %s", err)
	} else {
		log.Infof("init done: %s, %s", zyTempDev.DeviceType(), zyTempDev.DeviceId())

		sds = append(sds, zyTempDev)
	}

	sensorsCol := &SensorCollector{sensorDevices: sds}

	prometheus.MustRegister(sensorsCol)

	http.Handle("/metrics", promhttp.Handler())
	log.Infof("metrics available at http://%s/metrics", *flagAddr)

	log.Fatal(http.ListenAndServe(*flagAddr, nil))
}
