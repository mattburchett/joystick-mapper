//go:build linux

package mapper

import "testing"

// TestRealDevices exercises the js backend against whatever hardware is
// attached; it skips when none is present.
func TestRealDevices(t *testing.T) {
	devices := ListDevices()
	if len(devices) == 0 {
		t.Skip("no joysticks attached")
	}
	for _, info := range devices {
		dev, err := openDevice(info.ID)
		if err != nil {
			t.Errorf("open %d: %v", info.ID, err)
			continue
		}
		buttons, axes, err := dev.Read()
		if err != nil {
			t.Errorf("read %d: %v", info.ID, err)
		}
		if len(buttons) != info.Buttons || len(axes) != info.Axes {
			t.Errorf("device %d: snapshot %d buttons/%d axes, want %d/%d",
				info.ID, len(buttons), len(axes), info.Buttons, info.Axes)
		}
		if info.Name == "" {
			t.Errorf("device %d: empty name", info.ID)
		}
		t.Logf("id=%d name=%q buttons=%d axes=%d", info.ID, info.Name, info.Buttons, info.Axes)
		dev.Close()
	}
}
