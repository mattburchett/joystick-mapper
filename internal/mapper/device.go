package mapper

import (
	"fmt"
	"sort"
)

// Device abstracts a joystick backend. The Linux backend talks to the
// kernel js interface directly (the popular Go libraries cap buttons at 32
// via a uint32 bitmask; VKB/Virpil-class devices report 80+). Windows uses
// the 0xcafed00d/joystick winmm backend.
type Device interface {
	Name() string
	ButtonCount() int
	AxisCount() int
	// Read returns a snapshot of button and axis state, or the terminal
	// error if the device has disconnected.
	Read() (buttons []bool, axes []int, err error)
	Close()
}

// DeviceInfo describes an attached joystick.
type DeviceInfo struct {
	ID      int
	Name    string
	Buttons int
	Axes    int
}

func (d DeviceInfo) Label() string {
	return fmt.Sprintf("%d: %s", d.ID, d.Name)
}

// ListDevices probes joystick ids 0..15 and returns those that open.
func ListDevices() []DeviceInfo {
	var out []DeviceInfo
	for id := 0; id < 16; id++ {
		dev, err := openDevice(id)
		if err != nil {
			continue
		}
		out = append(out, DeviceInfo{ID: id, Name: dev.Name(), Buttons: dev.ButtonCount(), Axes: dev.AxisCount()})
		dev.Close()
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
