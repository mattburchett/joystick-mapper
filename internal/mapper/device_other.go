//go:build !linux

package mapper

import (
	"strings"

	"github.com/0xcafed00d/joystick"
)

// libDevice wraps 0xcafed00d/joystick for Windows (winmm). That API caps
// buttons at 32, which matches the winmm limit itself.
type libDevice struct {
	js joystick.Joystick
}

func openDevice(id int) (Device, error) {
	js, err := joystick.Open(id)
	if err != nil {
		return nil, err
	}
	return &libDevice{js: js}, nil
}

func (d *libDevice) Name() string {
	return strings.TrimSpace(strings.TrimRight(d.js.Name(), "\x00"))
}

func (d *libDevice) ButtonCount() int {
	if n := d.js.ButtonCount(); n < 32 {
		return n
	}
	return 32
}

func (d *libDevice) AxisCount() int { return d.js.AxisCount() }

func (d *libDevice) Read() ([]bool, []int, error) {
	state, err := d.js.Read()
	if err != nil {
		return nil, nil, err
	}
	btns := make([]bool, d.ButtonCount())
	for i := range btns {
		btns[i] = state.Buttons&(1<<uint(i)) != 0
	}
	return btns, state.AxisData, nil
}

func (d *libDevice) Close() { d.js.Close() }
