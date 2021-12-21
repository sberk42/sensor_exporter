package main

// config structs and JSON support

import (
	"encoding/json"
	"io/ioutil"

	"github.com/sberk42/sensor_exporter/sensors"
	log "github.com/sirupsen/logrus"
)

// Config to add constant labels to
type SensorConfig struct {
	DeviceType   string             `json:"deviceType"`
	DeviceId     string             `json:"deviceId"`
	DeviceVendor string             `json:"deviceVendor"`
	DeviceName   string             `json:"deviceName"`
	SensorModel  string             `json:"sensorModel"`
	SensorId     string             `json:"sensorId"`
	Labels       map[string]string  `json:"labels"`
	Calibrations map[string]float64 `json:"calibrations"`
	Ignore       bool               `json:"ignore"`
	IgnoreCount  int
	idFields     []string
}

type ExporterConfig struct {
	DeviceConfigs map[string]*sensors.DeviceConfig `json:"device_configs"`
	SensorConfigs []*SensorConfig                  `json:"sensor_configs"`
}

func ParseConfigJSON(cfgFile string) (*ExporterConfig, error) {

	// read metrics
	jsonData, err := ioutil.ReadFile(*flagConfigFile)
	if err != nil {
		return nil, err
	}

	var config *ExporterConfig
	err = json.Unmarshal(jsonData, &config)
	if err != nil {
		return nil, err
	}
	log.Debugf("CONFIG: read config: %v", config)

	return config, nil
}
