package sensors

/* collector for 433 Mhz sensors using rtl_433 for collecting
 * https://github.com/merbanan/rtl_433
 */

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

var FlagRtl433Path string

type rtl433 struct {
	rtl433_path    string
	additionalArgs []string
	deviceId       string
	manufacturer   string
	deviceName     string

	pkg_counter float64
	err_connect float64
	err_io      float64
	err_parse   float64

	cmd          *exec.Cmd
	stdoutPipe   io.ReadCloser
	stdoutReader *bufio.Reader
	stderrPipe   io.ReadCloser
	stderrReader *bufio.Reader

	maxMeaurementsPerSensor int
	sensorMapLock           *sync.Mutex
	sensordata              map[string]map[string]interface{}
}

var valueToMeasurement = map[string]MeasurementType{
	"temperature_c": TEMPERATURE_C,
	"humidity":      HUMIDITY_PERCENT,

	"battery":    BATTERY,
	"battery_ok": BATTERY_OK,

	"power_w": POWER_W,

	"preasure_hpa": PREASURE_HPA,

	"rain_rate_mm_h": RAIN_RATE_MM_H,
	"rain_mm":        RAIN_MM,

	"uv": UV_INDEX,

	"wind_avg_m_s": WIND_AVG_M_S,
	"wind_max_m_s": WIND_MAX_M_S,
	"wind_dir_deg": WIND_DIR_DEG,
}

func (r *rtl433) DeviceType() string {
	return "RTL_433"
}

func (r *rtl433) DeviceId() string {
	return r.deviceId
}

func (r *rtl433) DeviceVendor() string {
	return r.manufacturer
}

func (r *rtl433) DeviceName() string {
	return r.deviceName
}

func (r *rtl433) GetMeasurements() []Measurement {
	r.sensorMapLock.Lock()
	defer r.sensorMapLock.Unlock()

	mes := make([]Measurement, len(r.sensordata)*r.maxMeaurementsPerSensor+4)
	count := 0

	for k, sd := range r.sensordata {
		for m, v := range sd {
			if m == "channel" || m == "id" || m == "mic" || m == "model" || m == "time" {
				// ignore attributes not use as measurements
				continue
			}

			// first need to get unit converted, but we don't know yet wheater value is float, so we use a dummy value
			newM, conv := GetMeasurementConverter(m)

			mt, ok := valueToMeasurement[newM]
			if ok {
				f, err := asFloat(v)
				if err != nil {
					log.Warnf("RTL433: error converting %s (%v) - %s", m, v, err)
				} else {
					if conv != nil {
						f = conv.Convert(f)
					}

					model := asString(sd["model"])
					id := asString(sd["channel"]) + "_" + asString(sd["id"])

					mes[count] = Measurement{mt, f, model, id}
					count++
				}
			} else {
				log.Debugf("RTL433; no rule for measurement %s:%s (%v)", m, newM, v)
			}

		}

		delete(r.sensordata, k)
	}

	mes[count] = Measurement{VALUES_COUNTER, r.pkg_counter, "", ""}
	count++

	mes[count] = Measurement{ERRORS_CONNECT, r.err_connect, "", ""}
	count++

	mes[count] = Measurement{ERRORS_IO, r.err_io, "", ""}
	count++

	mes[count] = Measurement{ERRORS_PARSE, r.err_parse, "", ""}
	count++

	return mes[:count]
}

func asString(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

func asFloat(v interface{}) (float64, error) {
	var f float64
	switch i := v.(type) {
	case float64:
		f = i
	case float32:
		f = float64(i)
	case int64:
		f = float64(i)
	case int32:
		f = float64(i)
	case int:
		f = float64(i)
	case uint64:
		f = float64(i)
	case uint32:
		f = float64(i)
	case uint:
		f = float64(i)
	case string:
		var err error
		f, err = strconv.ParseFloat(i, 64)
		if err != nil {
			return math.NaN(), err
		}
	default:
		return math.NaN(), fmt.Errorf("can't convert %v (%v) to float64", v, i)
	}

	return f, nil
}

func (r *rtl433) monitor() {
	log.Debugf("RTL433: starting monitoring sensor")

	for {
		err := r.run_RTL433(false)

		if err != nil {
			log.Errorf("RTL433: error starting rtl_433: %s - retrying after 20 secs", err)
			r.close()
			r.err_connect++
		}

		if r.stdoutReader != nil {
			line, err := r.stdoutReader.ReadBytes('\n')
			if err != nil {
				log.Warnf("RTL433: Error reading stdout: %s", err)
				r.close()
				r.err_io++
			} else {
				log.Debugf("RTL433: %s", line)
				r.pkg_counter++

				var data map[string]interface{}
				err := json.Unmarshal(line, &data)
				if err != nil {
					log.Warnf("RTL433: Failed to unmarshal data: %s", err)
					r.err_parse++
				} else {
					key := asString(data["model"]) + "_" + asString(data["channel"]) + "_" + asString(data["id"])
					log.Debugf("RTL433: Unmarshalled %s: %v", key, data)

					r.sensorMapLock.Lock()
					r.sensordata[key] = data

					if len(data) > r.maxMeaurementsPerSensor {
						r.maxMeaurementsPerSensor = len(data)
					}

					r.sensorMapLock.Unlock()
				}
			}
		} else {
			time.Sleep(20 * time.Second)
		}
	}
}

func (r *rtl433) close() {
	if r.stdoutPipe != nil {
		r.stdoutPipe.Close()
	}
	r.stdoutPipe = nil
	r.stdoutReader = nil

	if r.stderrPipe != nil {
		r.stderrPipe.Close()
	}
	r.stderrPipe = nil
	r.stderrReader = nil

	if r.cmd != nil && r.cmd.Process != nil {
		r.cmd.Process.Kill()
	}
	r.cmd = nil
}

func (r *rtl433) readStderr() {
	for {
		if r.stderrReader == nil {
			return
		}

		line, err := r.stderrReader.ReadString('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			log.Warnf("RTL433: Error reading stderr: %s", err)
		} else {
			log.Debugf("RTL433: %s", line)
		}
	}
}

func (r *rtl433) run_RTL433(init bool) error {

	if r.cmd != nil {
		return nil // already started
	}

	log.Debugf("RTL433: runnning: %s", r.rtl433_path)

	args := []string{"-v", "-C", "si", "-F", "json"}
	args = append(args, r.additionalArgs...)

	log.Debugf("RTL433: starting %s with args: %v", r.rtl433_path, args)
	r.cmd = exec.Command(r.rtl433_path, args...)

	// make sure rtl is killed if we get killed
	// https://stackoverflow.com/questions/34095254/panic-in-other-goroutine-not-stopping-child-process/34095869#34095869
	// is there a better/portable approach?
	r.cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}

	var err error
	// prepare pipes
	r.stdoutPipe, err = r.cmd.StdoutPipe()
	if err != nil {
		log.Errorf("RTL433: Failed creating stdoutpipe: %s", err)
		r.close()
		return err
	}
	r.stderrPipe, err = r.cmd.StderrPipe()
	if err != nil {
		log.Errorf("RTL433: Failed creating stderrpipe: %s", err)
		r.close()
		return err
	}

	r.stdoutReader = bufio.NewReader(r.stdoutPipe)
	r.stderrReader = bufio.NewReader(r.stderrPipe)

	// start rtl process
	err = r.cmd.Start()
	if err != nil {
		log.Errorf("RTL433: start failed: %s", err)
		r.close()
		return err
	}

	// start thread to wait for end and cleanup
	go func(r *rtl433) {
		err := r.cmd.Wait()
		if err != nil {
			log.Errorf("RTL433: rtl_433 exited with error: %s", err)
		} else {
			log.Errorf("RTL433: rtl_433 unexpectedly exited without error")
		}

		r.close()
	}(r)

	if init {
		// parse output from stderr and look for:
		// trying device  0:  Realtek, RTL2838UHIDIR, SN: 00000001
		// Found Rafael Micro R820T tuner
		// Using device 0: Generic RTL2832U OEM
		tryingRe := regexp.MustCompile(`(?i)trying device\s*\d+:\s*(\w.*)`)
		foundRe := regexp.MustCompile(`(?i)Found\s*(\w.*)`)
		usingRe := regexp.MustCompile(`(?i)Using device\s*\d+:\s*(\w.*)`)

		for {
			line, err := r.stderrReader.ReadString('\n')
			if err == io.EOF {
				r.close()
				return fmt.Errorf("received EOF during init of rtl_433")
			} else if err != nil {
				r.close()
				return fmt.Errorf("unexpected error during init of rtl_433: %s", err)
			}

			log.Debugf("RTL433: %s", line)

			m := tryingRe.FindStringSubmatch(line)
			if len(m) > 1 {
				log.Debugf("RTL433: Setting Manufacturer to %v", m)
				r.manufacturer = m[1]
			}

			m = foundRe.FindStringSubmatch(line)
			if len(m) > 1 {
				log.Debugf("RTL433: Setting DeviceName to %v", m)
				r.deviceName = m[1]
			}

			m = usingRe.FindStringSubmatch(line)
			if len(m) > 1 {
				log.Debugf("RTL433: Setting DeviceId to %v - init done", m)
				r.deviceId = m[1]
				break
			}

			if strings.HasPrefix(line, "Reading samples") {
				log.Warnf("RTL433: rtl_433 starts reading - but not all info parsed", m)
				break
			}
		}
	}

	// we're ready so create map for storing sensor data
	r.sensorMapLock = &sync.Mutex{}
	r.sensordata = make(map[string]map[string]interface{})
	r.maxMeaurementsPerSensor = 4 // a wild guess

	// we got the device info, the rest of stderr goes to debug
	go r.readStderr()

	return nil
}

func InitFlags_rtl433() {
	flag.StringVar(&FlagRtl433Path, "rtl433-path", "rtl_433", "Path to rtl_433 binary.")
}

func InitSensor_rtl433(cfg *DeviceConfig) (SensorDevice, error) {

	var addArgs []string
	if cfg != nil && (*cfg)["additional_args"] != "" {
		addArgs = strings.Split((*cfg)["additional_args"], " ")
	}

	// check that device exists
	r := &rtl433{rtl433_path: FlagRtl433Path, additionalArgs: addArgs, deviceId: "<unknown>", manufacturer: "<unknown>", deviceName: "<unknown>"}

	err := r.run_RTL433(true)

	if err != nil {
		log.Errorf("RTL433: Error dedecting RLT_433 sensor: %s", err)
		return nil, err
	}

	go r.monitor()

	return r, nil
}
