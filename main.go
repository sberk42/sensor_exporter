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
	"flag"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sberk42/sensor_exporter/sensors"
	log "github.com/sirupsen/logrus"
)

var (
	flagTest = flag.Bool("test", false, "print all available metrics to stdout")

	flagAddr = flag.String("listen-address", "127.0.0.1:9043", "The address to listen on for HTTP requests.")
)

var metricDescs map[sensors.MeasurementType]*prometheus.Desc
var metricTypes map[sensors.MeasurementType]prometheus.ValueType

type SensorCollector struct {
	sensorDevice sensors.SensorDevice
}

// Implement prometheus Collector
func (fc *SensorCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, md := range metricDescs {
		ch <- md
	}
}

func (sc *SensorCollector) Collect(ch chan<- prometheus.Metric) {
	ms := sc.sensorDevice.GetMeasurements()

	labels := make([]string, 6)
	labels[0] = sc.sensorDevice.DeviceType()
	labels[1] = sc.sensorDevice.DeviceId()
	labels[2] = sc.sensorDevice.DeviceVendor()
	labels[3] = sc.sensorDevice.DeviceName()

	for _, m := range ms {
		md := metricDescs[m.Type]
		vt := metricTypes[m.Type]

		labels[4] = m.SensorModel
		labels[5] = m.SensorId

		metric, err := prometheus.NewConstMetric(md, vt, m.Value, labels...)
		if err != nil {
			log.Errorf("Error creating metric %s", err)
		} else {
			ch <- metric
		}

	}
}

func createMetricsDescs() {
	mtypes := sensors.GetAllMeasurementTypes()

	metricDescs = make(map[sensors.MeasurementType]*prometheus.Desc, len(mtypes))
	metricTypes = make(map[sensors.MeasurementType]prometheus.ValueType, len(mtypes))
	labels := []string{"device_type", "device_id", "device_vendor", "device_name", "sensor_model", "sensor_id"}

	for _, mt := range mtypes {
		mDetails := sensors.GetMeasurementTypeDetails(mt)

		metricDescs[mt] = prometheus.NewDesc("sensor_measurement_"+mDetails.MetricName, mDetails.MetricHelp, labels, nil)

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

func test() {
}

func main() {
	flag.Parse()

	if *flagTest {
		test()
		return
	}

	createMetricsDescs()

	sensorDevice, err := sensors.InitSensor_zytemp()
	if err != nil {
		log.Errorf("cannot init sensor: %s\n", err)
		return
	}

	log.Infof("init done: %s, %s\n", sensorDevice.DeviceType(), sensorDevice.DeviceId())

	zyCol := &SensorCollector{sensorDevice: sensorDevice}

	prometheus.MustRegister(zyCol)

	http.Handle("/metrics", promhttp.Handler())
	log.Infof("metrics available at http://%s/metrics\n", *flagAddr)

	log.Fatal(http.ListenAndServe(*flagAddr, nil))
}
