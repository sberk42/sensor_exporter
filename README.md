# sensor_exporter
prometheus exporter for multiple sensors and devices 

# goal
instead of using multiple exporters with different metric names and labels, I wanted to implement a common exporter for all indoor and outdoor sensors and devices I have

# supported devices
- USB zytemp (as used in TFA Airco2ntrol Coach, based on: https://hackaday.io/project/5301-reverse-engineering-a-low-cost-usb-co-monitor)
- RTL 433MHz weather station sensors (details: https://triq.org/rtl_433)

# project status
- retrieval works for the devices I own
- initial configuration of sensor labels in json possible
- next steps: improve device/sensor specific config (e.g. calibration, ignoring RTL sensors), documentation, ...
