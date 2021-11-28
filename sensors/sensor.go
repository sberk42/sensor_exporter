package sensors

/* define supported measurement types and generic interface to be implemented by each sensor device
 */
type MeasurementType int

const (
	// measurements from zyTemp
	TEMPERATURE_C MeasurementType = iota
	HUMIDITY_PERCENT
	CO2_PPM

	// additional measurements from rtl 433 - is there a complete list
	BATTERY
	BATTERY_OK

	POWER_W

	PREASURE_HPA

	RAIN_RATE_MM_H
	RAIN_MM

	UV_INDEX

	WIND_AVG_M_S
	WIND_MAX_M_S
	WIND_DIR_DEG

	// couter types
	VALUES_COUNTER

	// types for error counters
	ERRORS_CONNECT
	ERRORS_IO
	ERRORS_PARSE
)

type Measurement struct {
	Type        MeasurementType
	Value       float64
	SensorModel string // model of sensor to use as label - in case multiple sensors report the same measurement
	SensorId    string // id of sensor to use as label - in case multiple sensors report the same measurement
}

type SensorDevice interface {
	DeviceType() string
	DeviceId() string
	DeviceVendor() string
	DeviceName() string
	GetMeasurements() []Measurement
}

/* now define for each MeasurementType details to be used for creating a prometheus metric for it
 */
type ValueType int

const (
	COUNTER ValueType = iota
	GAUGE
)

type MeasurementTypeDetails struct {
	MetricName  string // will be used as last part prometheus metric FQName
	MetricHelp  string // metric help
	MetricValue ValueType
}

// use map instead of array to make definition easier to read and avoid ordering issues
var mt_details = map[MeasurementType]MeasurementTypeDetails{
	TEMPERATURE_C:    {"temperature_c", "temperature in C", GAUGE},
	HUMIDITY_PERCENT: {"humidity_percent", "humidity in %", GAUGE},
	CO2_PPM:          {"co2_ppm", "co2 concentration in ppm", GAUGE},

	BATTERY:    {"battery", "battery state", GAUGE},
	BATTERY_OK: {"battery_ok", "battery OK", GAUGE},

	POWER_W: {"power_w", "power in W", GAUGE},

	PREASURE_HPA: {"preasure_hpa", "preasure in hPa", GAUGE},

	RAIN_RATE_MM_H: {"rain_rate_mm_h", "rain rate in mm/H", GAUGE},
	RAIN_MM:        {"rain_mm", "rain in mm", GAUGE},

	UV_INDEX: {"uv_index", "UV index", GAUGE},

	WIND_AVG_M_S: {"wind_avg_m_s", "wind average in m/s", GAUGE},
	WIND_MAX_M_S: {"wind_max_m_s", "wind max in m/s", GAUGE},
	WIND_DIR_DEG: {"wind_dir_deg", "wind direction in degree", GAUGE},

	VALUES_COUNTER: {"values_counter", "values received from sensor", COUNTER},
	ERRORS_CONNECT: {"errors_connect_counter", "errors connecting to sensor", COUNTER},
	ERRORS_IO:      {"errors_io_counter", "errors receiving data from sensor", COUNTER},
	ERRORS_PARSE:   {"errors_parse_counter", "errors parsing data from sensor", COUNTER},
}

func GetAllMeasurementTypes() []MeasurementType {

	keys := make([]MeasurementType, 0, len(mt_details))
	for k := range mt_details {
		keys = append(keys, k)
	}

	return keys
}

func GetMeasurementTypeDetails(m_type MeasurementType) MeasurementTypeDetails {
	return mt_details[m_type]
}
