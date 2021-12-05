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
	"fmt"
	"os"
	"strings"

	"github.com/sberk42/sensor_exporter/sensors"
	log "github.com/sirupsen/logrus"
)

var (
	flagListDevs   = flag.Bool("list-devices", false, "List supported devices and exit.")
	flagDevices    = flag.String("devices", "ALL", "Comma seperated list of device IDs to initialize (see list-devices for known IDs)")
	flagConfigFile = flag.String("config-file", "sensor_exporter.json", "The JSON file with the metric definitions.")
	flagAddr       = flag.String("listen-address", "127.0.0.1:9043", "The address to listen on for HTTP requests.")
	flagLogLevel   = flag.String("log-level", "info", "The log level {trace|debug|info|warn|error}")
)

var config *ExporterConfig

func init() {
	// add sensor specific flags
	for _, sensorDev := range sensors.SupportedSensorDevices {
		sensorDev.InitFlagsFunction()
	}

	flag.Parse()

	// init log level
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})

	logLevel, err := log.ParseLevel(*flagLogLevel)
	if err != nil {
		log.Fatalf("error parsing log level:", err)
	} else {
		log.SetLevel(logLevel)
	}
}

func main() {
	if *flagListDevs {
		fmt.Println("Supported Devices:")
		for id, dev := range sensors.SupportedSensorDevices {
			fmt.Printf("  %s: %s\n", id, dev.Description)
		}

		return
	}

	// read config
	var err error
	config, err = ParseConfigJSON(*flagConfigFile)
	if err != nil {
		log.Fatalf("error reading config file:", err)
	}

	CreateMetricsDescs()

	// init sensors
	sIDs := strings.Split(*flagDevices, ",")
	if sIDs[0] == "ALL" {
		sIDs = make([]string, 0)
		for id := range sensors.SupportedSensorDevices {
			sIDs = append(sIDs, id)
		}
	}
	log.Debugf("initializing devices: %v", sIDs)

	var sds []sensors.SensorDevice
	for _, id := range sIDs {
		sensorDev, ok := sensors.SupportedSensorDevices[id]
		if !ok {
			log.Fatalf("Unknown sensor device '%s' - use list-devices to check supported devices", id)
		}

		dev, err := sensorDev.InitFunction(config.DeviceConfigs[id])
		if err != nil {
			log.Errorf("cannot init sensor: %s", err)
		} else {
			log.Infof("init done: %s, %s", dev.DeviceType(), dev.DeviceId())

			sds = append(sds, dev)
		}
	}

	if len(sds) == 0 {
		log.Fatal("Failed to init any sensor device")
	}

	// register collected sensors
	RegisterCollectorAndServeMetrics(sds, *flagAddr)
}
