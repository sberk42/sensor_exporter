package sensors

/* define supported measurement types and generic interface to be implemented by each sensor
 */
type MeasurementType int

const (
	TEMPERATURE_C MeasurementType = iota
	HUMIDITY_PERCENT
	CO2_PPM

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
	VALUES_COUNTER:   {"values_counter", "values received from sensor", COUNTER},
	ERRORS_CONNECT:   {"errors_connect_counter", "errors connecting to sensor", COUNTER},
	ERRORS_IO:        {"errors_io_counter", "errors receiving data from sensor", COUNTER},
	ERRORS_PARSE:     {"errors_parse_counter", "errors parsing data from sensor", COUNTER},
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
