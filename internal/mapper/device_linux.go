//go:build linux

package mapper

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Kernel joystick API (linux/joystick.h).
const (
	jsEventButton = 0x01
	jsEventAxis   = 0x02
	jsEventInit   = 0x80

	jsiocgAxes    = 0x80016a11 // _IOR('j', 0x11, __u8)
	jsiocgButtons = 0x80016a12 // _IOR('j', 0x12, __u8)
	jsiocgName    = 0x80806a13 // JSIOCGNAME(128)
)

type linuxDevice struct {
	file    *os.File
	name    string
	buttons int
	axes    int

	mu       sync.Mutex
	btnState []bool
	axState  []int
	readErr  error
}

func openDevice(id int) (Device, error) {
	f, err := os.OpenFile(fmt.Sprintf("/dev/input/js%d", id), os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	var nAxes, nButtons uint8
	var nameBuf [128]byte
	if err := ioctl(f, jsiocgAxes, unsafe.Pointer(&nAxes)); err != nil {
		f.Close()
		return nil, fmt.Errorf("JSIOCGAXES: %w", err)
	}
	if err := ioctl(f, jsiocgButtons, unsafe.Pointer(&nButtons)); err != nil {
		f.Close()
		return nil, fmt.Errorf("JSIOCGBUTTONS: %w", err)
	}
	if err := ioctl(f, jsiocgName, unsafe.Pointer(&nameBuf[0])); err != nil {
		f.Close()
		return nil, fmt.Errorf("JSIOCGNAME: %w", err)
	}

	name := string(nameBuf[:])
	if i := strings.IndexByte(name, 0); i >= 0 {
		name = name[:i]
	}

	d := &linuxDevice{
		file:     f,
		name:     strings.TrimSpace(name),
		buttons:  int(nButtons),
		axes:     int(nAxes),
		btnState: make([]bool, nButtons),
		axState:  make([]int, nAxes),
	}
	go d.readLoop()
	return d, nil
}

func ioctl(f *os.File, req uint, arg unsafe.Pointer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), uintptr(req), uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

// readLoop consumes 8-byte js_event records:
//
//	struct js_event { __u32 time; __s16 value; __u8 type; __u8 number; };
//
// The kernel replays current state as INIT events on open, so the snapshot
// is correct from the start.
func (d *linuxDevice) readLoop() {
	buf := make([]byte, 8)
	for {
		if _, err := d.file.Read(buf); err != nil {
			d.mu.Lock()
			d.readErr = err
			d.mu.Unlock()
			return
		}
		value := int16(binary.LittleEndian.Uint16(buf[4:6]))
		evType := buf[6]
		number := int(buf[7])

		d.mu.Lock()
		if evType&jsEventButton != 0 && number < len(d.btnState) {
			d.btnState[number] = value != 0
		}
		if evType&jsEventAxis != 0 && number < len(d.axState) {
			d.axState[number] = int(value)
		}
		d.mu.Unlock()
	}
}

func (d *linuxDevice) Name() string     { return d.name }
func (d *linuxDevice) ButtonCount() int { return d.buttons }
func (d *linuxDevice) AxisCount() int   { return d.axes }

func (d *linuxDevice) Read() ([]bool, []int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.readErr != nil {
		return nil, nil, d.readErr
	}
	btns := make([]bool, len(d.btnState))
	copy(btns, d.btnState)
	axes := make([]int, len(d.axState))
	copy(axes, d.axState)
	return btns, axes, nil
}

func (d *linuxDevice) Close() { d.file.Close() }
