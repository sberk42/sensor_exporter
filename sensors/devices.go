package sensors

/* define generic interface to be implemented by each sensor device
 */
type SensorDevice interface {
	DeviceType() string
	DeviceId() string
	DeviceVendor() string
	DeviceName() string
	GetMeasurements() []Measurement
}

type DeviceConfig map[string]string

type SupportedDevice struct {
	Description       string
	InitFlagsFunction func()
	InitFunction      func(*DeviceConfig) (SensorDevice, error)
}

var SupportedSensorDevices = map[string]*SupportedDevice{
	"usb_zytemp": {"USB CO2 sensor: Holtek Semiconductor, Inc. USB-zyTemp", InitFlags_zytemp, InitSensor_zytemp},
	"rtl_433":    {"Generic wrapper using rtl_433 to collect measurements", InitFlags_rtl433, InitSensor_rtl433},
}
