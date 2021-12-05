package sensors

/* collector for USC Zytemp device, details how to get data see:
 * https://hackaday.io/project/5301-reverse-engineering-a-low-cost-usb-co-monitor
 */

import (
	"fmt"
	"time"

	"github.com/google/gousb"
	log "github.com/sirupsen/logrus"
)

const default_vid = 0x04d9
const default_pid = 0xa052

// libusb_control_transfer codes see https://libusb.sourceforge.io/api-1.0/libusb_8h_source.html
const usb_ctrl_request_type = 0x21 // LIBUSB_REQUEST_TYPE_CLASS | LIBUSB_RECIPIENT_INTERFACE
const usb_ctrl_request = 0x09      // LIBUSB_REQUEST_SET_CONFIGURATION
const usb_ctrl_value = 0x300
const usb_ctrl_index = 0x000

type sensorDevice struct {
	vId      gousb.ID
	pId      gousb.ID
	context  *gousb.Context
	device   *gousb.Device
	inf      *gousb.Interface
	done     func()
	endPoint *gousb.InEndpoint

	manufacturer string
	product      string
	pkg_counter  float64
	err_connect  float64
	err_io       float64
	err_parse    float64

	temp_k   float64
	humidity float64
	co2      float64
}

func (s *sensorDevice) DeviceType() string {
	return "USB"
}

func (s *sensorDevice) DeviceId() string {
	return fmt.Sprintf("0x%s:0x%s", s.vId.String(), s.pId.String())
}

func (s *sensorDevice) DeviceVendor() string {
	return s.manufacturer
}

func (s *sensorDevice) DeviceName() string {
	return s.product
}

func (s *sensorDevice) GetMeasurements() []Measurement {

	mes := [7]Measurement{}
	count := 0

	if s.temp_k >= 0 {
		mes[count] = Measurement{TEMPERATURE_C, s.temp_k - 273.15, "", ""}
		s.temp_k = -1
		count++
	}

	if s.humidity >= 0 {
		mes[count] = Measurement{HUMIDITY_PERCENT, s.humidity, "", ""}
		s.humidity = -1
		count++
	}

	if s.co2 >= 0 {
		mes[count] = Measurement{CO2_PPM, s.co2, "", ""}
		s.co2 = -1
		count++
	}

	mes[count] = Measurement{VALUES_COUNTER, s.pkg_counter, "", ""}
	count++

	mes[count] = Measurement{ERRORS_CONNECT, s.err_connect, "", ""}
	count++

	mes[count] = Measurement{ERRORS_IO, s.err_io, "", ""}
	count++

	mes[count] = Measurement{ERRORS_PARSE, s.err_parse, "", ""}
	count++

	return mes[:count]
}

func (s *sensorDevice) closeDevice() {
	s.endPoint = nil

	if s.inf != nil {
		s.inf.Close()
	}

	if s.done != nil {
		s.done()
	}
	s.done = nil

	if s.inf != nil {
		s.inf.Close()
	}
	s.inf = nil

	if s.device != nil {
		s.device.Close()
	}
	s.device = nil

	if s.context != nil {
		s.context.Close()
	}
	s.context = nil
}

func (s *sensorDevice) openDevice() error {

	// if device was opened before do a complete cleanup of everything
	s.closeDevice()

	s.context = gousb.NewContext()
	// s.context.Debug(4)

	var err error
	s.device, err = s.context.OpenDeviceWithVIDPID(s.vId, s.pId)
	if err != nil {
		return err
	}
	if s.device == nil {
		return fmt.Errorf("device %s not found", s.DeviceId())
	}

	log.Debugf("zyTemp: Device: %v", s.device)

	err = s.device.SetAutoDetach(true)
	if err != nil {
		return err
	}

	s.manufacturer, _ = s.device.Manufacturer()
	s.product, _ = s.device.Product()

	return nil
}

func (s *sensorDevice) connectDevice() error {

	if s.endPoint != nil {
		// if endPoint is set we are already connected
		return nil
	}

	var err error
	s.inf, s.done, err = s.device.DefaultInterface()
	if err != nil {
		return err
	}

	log.Debugf("zyTemp: Interface: %v", s.inf)

	_, err = s.device.Control(usb_ctrl_request_type, usb_ctrl_request, usb_ctrl_value, usb_ctrl_index, randomKey)
	if err != nil {
		return err
	}

	s.endPoint, err = s.inf.InEndpoint(1)
	if err != nil {
		return err
	}

	log.Debugf("zyTemp: Endpoint: %v", s.endPoint)

	log.Infof("zyTemp: %s %s (%s) connect and ready to receive data", s.DeviceVendor(), s.DeviceName(), s.DeviceId())

	return nil
}

var (
	randomKey = []byte{0xc4, 0xc6, 0xc0, 0x92, 0x40, 0x23, 0xdc, 0x96}
	cState    = []byte{0x48, 0x74, 0x65, 0x6D, 0x70, 0x39, 0x39, 0x65}
	shuffle   = []uint8{2, 4, 0, 7, 1, 6, 5, 3}
)

func decrypt(data []byte) {

	if data[4] == 0x0d && ((data[0]+data[1]+data[2])&0xff) == data[3] {
		return
	}

	// decrypt taken from https://github.com/huhamhire/air-co2-exporter/blob/master/monitor/decrypt.go

	var dataXor [8]byte
	for i := 0; i < len(cState); i++ {
		idx := shuffle[i]
		dataXor[idx] = data[i] ^ randomKey[idx]
	}

	var dataTemp [8]byte
	for i := 0; i < len(cState); i++ {
		dataTemp[i] = ((dataXor[i] >> 3) | (dataXor[(i-1+8)%8] << 5)) & 0xff
	}

	for i, state := range cState {
		cTemp := ((state >> 4) | (state << 4)) & 0xff
		data[i] = uint8((0x100 + uint16(dataTemp[i]) - uint16(cTemp)) & uint16(0xff))
	}
}

func (s *sensorDevice) monitor() {

	log.Debugf("zyTemp: starting monitoring sensor")

	data := make([]byte, 8)
	for {
		err := s.connectDevice()
		if err != nil {
			log.Errorf("zyTemp: error connecting %s - closing device and retrying after 20 secs", err)

			s.closeDevice()
			s.err_connect++
		}

		if s.endPoint != nil {
			count, err := s.endPoint.Read(data)

			if err != nil {
				log.Warningf("zyTemp: error reading: %s", err)
				s.err_io++
			} else if count < 8 {
				log.Warningf("zyTemp: only read %d bytes instead of 8", count)
				s.err_parse++
			} else {
				s.pkg_counter++
				decrypt(data)

				if data[4] != 0x0d || ((data[0]+data[1]+data[2])&0xff) != data[3] {
					log.Warning("zyTemp: Checksum error")

					s.err_parse++
				} else {
					op := data[0]
					val := float64(uint(data[1])<<8 | uint(data[2]))

					// From http://co2meters.com/Documentation/AppNotes/AN146-RAD-0401-serial-communication.pdf
					if op == 0x50 {
						s.co2 = val
					} else if op == 0x42 {
						s.temp_k = val / 16
					} else if op == 0x41 {
						s.humidity = val / 100
					}
				}
			}
		} else {
			// no endpoint open so sleep a bit
			time.Sleep(20 * time.Second)
		}
	}
}

func InitSensor_zytemp() (SensorDevice, error) {

	// check that device exists
	s := &sensorDevice{vId: default_vid, pId: default_pid,
		temp_k: -1, humidity: -1, co2: -1,
		manufacturer: "", product: "",
		pkg_counter: 0,
		err_connect: 0, err_io: 0, err_parse: 0}

	err := s.openDevice()

	if err != nil {
		log.Errorf("zyTemp: Error decting zytemp sensor: %s", err)
		return nil, err
	}

	go s.monitor()

	return s, nil
}
