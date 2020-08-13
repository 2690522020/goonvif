package goonvif

import (
	"testing"
	"time"
)

func Test_ONVIF(t *testing.T) {
	cam, err := NewONVIFClient("test", "Profile_1", "192.168.0.1:80", "admin", "123456", nil)
	if err == nil {
		_ = cam.PTZGoto(1, -1, 0)
		time.Sleep(time.Second)
		_ = cam.PTZStop()
	}
}
