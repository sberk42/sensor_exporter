package main

// prometheus metrics creation and collection

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sberk42/sensor_exporter/sensors"
	log "github.com/sirupsen/logrus"
)

var metricDescs map[sensors.MeasurementType]*prometheus.Desc
var metricTypes map[sensors.MeasurementType]prometheus.ValueType
var metricLabels []string
var configLabelIndex map[string]int

type SensorCollector struct {
	sensorDevices []sensors.SensorDevice
}

// Implement prometheus Collector
func (sc *SensorCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, md := range metricDescs {
		ch <- md
	}
}

func labelsMatchConfig(sc *SensorConfig, labels []string) bool {

	for i, l := range sc.idFields {
		if l != "" && l != labels[i] {
			return false
		}
	}

	return true
}

func createMeasurementLabels(dev sensors.SensorDevice, m *sensors.Measurement) ([]string, *SensorConfig) {
	// create static labels from device and measurement
	labels := make([]string, len(metricLabels))
	labels[0] = dev.DeviceType()
	labels[1] = dev.DeviceId()
	labels[2] = dev.DeviceVendor()
	labels[3] = dev.DeviceName()
	labels[4] = m.SensorModel
	labels[5] = m.SensorId

	// get labels from config
	for _, sc := range config.SensorConfigs {
		if labelsMatchConfig(sc, labels) {
			// add static labels
			for l, v := range sc.Labels {
				labels[configLabelIndex[l]] = v
			}

			return labels, sc
		}
	}

	return labels, nil
}

func createConfigLabels(sc *SensorConfig) []string {
	// create static labels from device and measurement
	labels := make([]string, len(metricLabels))
	labels[0] = sc.DeviceType
	labels[1] = sc.DeviceId
	labels[2] = sc.DeviceVendor
	labels[3] = sc.DeviceName
	labels[4] = sc.SensorModel
	labels[5] = sc.SensorId

	// add static labels
	for l, v := range sc.Labels {
		labels[configLabelIndex[l]] = v
	}

	return labels
}

func (sc *SensorCollector) Collect(ch chan<- prometheus.Metric) {

	for _, sd := range sc.sensorDevices {
		ms := sd.GetMeasurements()

		for _, m := range ms {
			md := metricDescs[m.Type]
			vt := metricTypes[m.Type]

			labels, sdConfig := createMeasurementLabels(sd, &m)

			value := m.Value
			if sdConfig != nil {
				if sdConfig.Ignore {
					log.Debugf("PROM: ignored measurement from %s: %s_%s", sd.DeviceName(), m.SensorModel, m.SensorId)
					sdConfig.IgnoreCount++
					continue
				}

				md := sensors.GetMeasurementTypeDetails(m.Type)
				cal, ok := sdConfig.Calibrations[md.MetricName]
				if ok {
					log.Debugf("PROM: applying calibration offset %f to %s from %s: %s_%s", cal, md.MetricName, sd.DeviceName(), m.SensorModel, m.SensorId)
					value += cal
				}
			}

			metric, err := prometheus.NewConstMetric(md, vt, value, labels...)
			if err != nil {
				log.Errorf("Error creating metric %s", err)
			} else {
				ch <- metric
			}
		}
	}

	// now report ignored values
	md := metricDescs[sensors.IGNORED_COUNTER]
	vt := metricTypes[sensors.IGNORED_COUNTER]
	for _, sc := range config.SensorConfigs {
		if sc.IgnoreCount > 0 {

			labels := createConfigLabels(sc)

			metric, err := prometheus.NewConstMetric(md, vt, float64(sc.IgnoreCount), labels...)
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

func CreateMetricsDescs() {
	mtypes := sensors.GetAllMeasurementTypes()

	metricDescs = make(map[sensors.MeasurementType]*prometheus.Desc, len(mtypes))
	metricTypes = make(map[sensors.MeasurementType]prometheus.ValueType, len(mtypes))
	configLabelIndex = make(map[string]int)

	metricLabels = []string{"device_type", "device_id", "device_vendor", "device_name", "sensor_model", "sensor_id"}
	for i, l := range metricLabels {
		configLabelIndex[l] = i
	}

	// append constant labels from config and fill idFields
	for _, sc := range config.SensorConfigs {
		sc.idFields = []string{sc.DeviceType, sc.DeviceId, sc.DeviceVendor, sc.DeviceName, sc.SensorModel, sc.SensorId}

		log.Debugf("PROM: init sc to %v", sc)

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

func RegisterCollectorAndServeMetrics(sds []sensors.SensorDevice, addr string) {
	// register collected sensors
	log.Infof("Creating prometheus collector for %d successfull initialized sensor devices", len(sds))

	sensorsCol := &SensorCollector{sensorDevices: sds}

	prometheus.MustRegister(sensorsCol)

	// now start http server and serve metrics
	http.Handle("/metrics", promhttp.Handler())
	log.Infof("Exporter started - metrics available at http://%s/metrics", addr)

	log.Fatal(http.ListenAndServe(addr, nil))
}
