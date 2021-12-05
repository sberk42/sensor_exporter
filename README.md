# sensor_exporter
prometheus exporter for multiple sensors and devices 

# goal
instead of using multiple exporters with different metric names and labels, I wanted to implement a common exporter for all indoor and outdoor sensors and devices I have

# supported devices
- USB zytemp (as used in TFA Airco2ntrol Coach, based on: https://hackaday.io/project/5301-reverse-engineering-a-low-cost-usb-co-monitor)
- RTL 433MHz weather station sensors (details: https://triq.org/rtl_433)

# project status
- retrieval and exporter works for the devices I own (sample: [metrics_sample.txt](metrics_sample.txt))
- initial configuration of sensor labels in json possible (sample: [sensor_exporter.json](sensor_exporter.json))
- next steps: improve device/sensor specific config (e.g. calibration, ignoring RTL sensors), documentation, ...

# installation
- USB zytemp - just connect device to USB port, exporter should find it (make sure that user has necessary permissions)
- RTL_433 - install rtl_433 and connect a supported SDR

# usage
```
Usage of ./sensor_exporter:
  -config-file string
        The JSON file with the metric definitions. (default "sensor_exporter.json")
  -listen-address string
        The address to listen on for HTTP requests. (default "127.0.0.1:9043")
  -log-level string
        The log level {trace|debug|info|warn|error} (default "info")
  -rtl433-path string
        Path to rtl_433 binary. (default "rtl_433")
```
