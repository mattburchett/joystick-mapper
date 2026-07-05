// Package mapper polls all attached joysticks and fires actions (keyboard
// sequences or profile switches) on button presses and axis deflections.
package mapper

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/mattburchett/joystick-mapper/internal/keyseq"
)

const (
	pollInterval = 8 * time.Millisecond
	scanInterval = time.Second
	maxDevices   = 16
)

// Event is a trigger edge (press or release) observed on a device.
type Event struct {
	Device  string // joystick name
	Trigger string // "button:N", "axis:N:+", "axis:N:-"
	Pressed bool
}

// Action is a compiled mapping target.
type Action struct {
	Steps   []keyseq.Step // keystroke sequence, when this is a keys action
	Profile string        // target profile, when this is a profile switch
	Hold    bool          // profile switch reverts on release
}

// TriggerLabel renders a trigger id in human-readable form.
func TriggerLabel(trigger string) string {
	var n int
	if _, err := fmt.Sscanf(trigger, "button:%d", &n); err == nil {
		return fmt.Sprintf("Button %d", n)
	}
	var dir string
	if _, err := fmt.Sscanf(trigger, "axis:%d:%s", &n, &dir); err == nil {
		if dir == "+" {
			return fmt.Sprintf("Axis %d +", n)
		}
		return fmt.Sprintf("Axis %d −", n)
	}
	return trigger
}

// Key builds the mapping-table key for a device-qualified trigger.
func Key(device, trigger string) string {
	if device == "" {
		return trigger
	}
	return device + "|" + trigger
}

// Engine polls every attached joystick, dispatching trigger edges against
// the active profile's mapping table.
type Engine struct {
	sender *keyseq.Sender

	mu            sync.Mutex
	profiles      map[string]map[string]Action // profile name → trigger key → action
	activeProfile string
	holdReturn    map[string]string // held trigger key → profile to restore
	axisThreshold int
	active        bool
	capture       func(device, trigger string)
	onEvent       func(Event)
	onDevices     func([]DeviceInfo)
	onProfile     func(name string)
	open          map[int]DeviceInfo // currently opened joystick ids

	sendQ chan []keyseq.Step
	stop  chan struct{}
}

func New(sender *keyseq.Sender, axisThreshold int) *Engine {
	return &Engine{
		sender:        sender,
		profiles:      map[string]map[string]Action{},
		holdReturn:    map[string]string{},
		axisThreshold: axisThreshold,
		open:          map[int]DeviceInfo{},
		sendQ:         make(chan []keyseq.Step, 8),
		stop:          make(chan struct{}),
	}
}

// OnEvent registers the UI callback for live trigger edges. Called from
// polling goroutines.
func (e *Engine) OnEvent(fn func(Event)) { e.mu.Lock(); e.onEvent = fn; e.mu.Unlock() }

// OnDevices registers the UI callback for connect/disconnect changes.
func (e *Engine) OnDevices(fn func([]DeviceInfo)) { e.mu.Lock(); e.onDevices = fn; e.mu.Unlock() }

// OnProfileChange registers the UI callback for engine-driven profile
// switches (mapped to joystick buttons).
func (e *Engine) OnProfileChange(fn func(name string)) {
	e.mu.Lock()
	e.onProfile = fn
	e.mu.Unlock()
}

// SetActive enables or disables firing of mapped actions.
func (e *Engine) SetActive(v bool) { e.mu.Lock(); e.active = v; e.mu.Unlock() }

func (e *Engine) Active() bool { e.mu.Lock(); defer e.mu.Unlock(); return e.active }

// SetAxisThreshold updates the deflection needed to count as a press.
func (e *Engine) SetAxisThreshold(v int) { e.mu.Lock(); e.axisThreshold = v; e.mu.Unlock() }

// SetKeyDelay updates the pause between keystrokes.
func (e *Engine) SetKeyDelay(d time.Duration) { e.sender.KeyDelay = d }

// SetProfiles replaces all compiled mapping tables and the active profile.
func (e *Engine) SetProfiles(profiles map[string]map[string]Action, active string) {
	e.mu.Lock()
	e.profiles = profiles
	e.activeProfile = active
	e.mu.Unlock()
}

// SetActiveProfile selects which profile's table is live.
func (e *Engine) SetActiveProfile(name string) {
	e.mu.Lock()
	e.activeProfile = name
	e.mu.Unlock()
}

// StartCapture arranges for the next press to be reported to fn instead of
// fired. Cleared automatically after one capture.
func (e *Engine) StartCapture(fn func(device, trigger string)) {
	e.mu.Lock()
	e.capture = fn
	e.mu.Unlock()
}

// CancelCapture aborts a pending capture.
func (e *Engine) CancelCapture() { e.mu.Lock(); e.capture = nil; e.mu.Unlock() }

// Test sends a sequence directly, as if its trigger had been pressed.
func (e *Engine) Test(steps []keyseq.Step) {
	select {
	case e.sendQ <- steps:
	default:
	}
}

// Devices returns the currently connected joysticks.
func (e *Engine) Devices() []DeviceInfo {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.deviceListLocked()
}

func (e *Engine) deviceListLocked() []DeviceInfo {
	list := make([]DeviceInfo, 0, len(e.open))
	for _, info := range e.open {
		list = append(list, info)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

// Start launches the device scanner and the send worker.
func (e *Engine) Start() {
	go e.sendLoop()
	go e.scanLoop()
}

// Stop terminates all goroutines.
func (e *Engine) Stop() { close(e.stop) }

func (e *Engine) sendLoop() {
	for {
		select {
		case <-e.stop:
			return
		case steps := <-e.sendQ:
			_ = e.sender.Send(steps)
		}
	}
}

// scanLoop opens every joystick that appears and spawns a poller for it,
// handling hotplug in both directions.
func (e *Engine) scanLoop() {
	for {
		for id := 0; id < maxDevices; id++ {
			e.mu.Lock()
			_, isOpen := e.open[id]
			e.mu.Unlock()
			if isOpen {
				continue
			}
			dev, err := openDevice(id)
			if err != nil {
				continue
			}
			info := DeviceInfo{ID: id, Name: dev.Name(), Buttons: dev.ButtonCount(), Axes: dev.AxisCount()}
			e.mu.Lock()
			e.open[id] = info
			onDevices := e.onDevices
			list := e.deviceListLocked()
			e.mu.Unlock()
			if onDevices != nil {
				onDevices(list)
			}
			go e.pollDevice(dev, info)
		}
		select {
		case <-e.stop:
			return
		case <-time.After(scanInterval):
		}
	}
}

// pollDevice runs until the device errors (unplugged) or the engine stops.
func (e *Engine) pollDevice(dev Device, info DeviceInfo) {
	defer func() {
		dev.Close()
		e.mu.Lock()
		delete(e.open, info.ID)
		onDevices := e.onDevices
		list := e.deviceListLocked()
		e.mu.Unlock()
		if onDevices != nil {
			onDevices(list)
		}
	}()

	var prevButtons []bool
	var prevAxis []int8 // -1, 0, +1 per axis
	first := true

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stop:
			return
		case <-ticker.C:
		}

		e.mu.Lock()
		threshold := e.axisThreshold
		e.mu.Unlock()

		buttons, axes, err := dev.Read()
		if err != nil {
			return
		}

		if first {
			// Take the initial state as baseline so resting positions
			// (e.g. analog triggers idling at -32768) don't fire.
			prevButtons = buttons
			prevAxis = make([]int8, len(axes))
			for i := range axes {
				prevAxis[i] = axisSign(axes[i], threshold)
			}
			first = false
			continue
		}

		for b := range buttons {
			if b >= len(prevButtons) || buttons[b] == prevButtons[b] {
				continue
			}
			e.handleEdge(info.Name, fmt.Sprintf("button:%d", b), buttons[b])
		}
		prevButtons = buttons

		for i := range axes {
			if i >= len(prevAxis) {
				break
			}
			sign := axisSign(axes[i], threshold)
			if sign == prevAxis[i] {
				continue
			}
			if prevAxis[i] != 0 {
				e.handleEdge(info.Name, axisTrigger(i, prevAxis[i]), false)
			}
			if sign != 0 {
				e.handleEdge(info.Name, axisTrigger(i, sign), true)
			}
			prevAxis[i] = sign
		}
	}
}

func axisSign(value, threshold int) int8 {
	switch {
	case value >= threshold:
		return 1
	case value <= -threshold:
		return -1
	}
	return 0
}

func axisTrigger(i int, sign int8) string {
	dir := "+"
	if sign < 0 {
		dir = "-"
	}
	return fmt.Sprintf("axis:%d:%s", i, dir)
}

// lookupLocked finds the action for a trigger in the active profile,
// preferring a device-qualified mapping over a device-agnostic one.
func (e *Engine) lookupLocked(device, trigger string) (Action, bool) {
	table := e.profiles[e.activeProfile]
	if table == nil {
		return Action{}, false
	}
	if act, ok := table[Key(device, trigger)]; ok {
		return act, true
	}
	act, ok := table[trigger]
	return act, ok
}

func (e *Engine) handleEdge(device, trigger string, pressed bool) {
	key := Key(device, trigger)

	e.mu.Lock()
	onEvent := e.onEvent
	onProfile := e.onProfile
	capture := e.capture

	if pressed && capture != nil {
		e.capture = nil
		e.mu.Unlock()
		capture(device, trigger)
		if onEvent != nil {
			onEvent(Event{Device: device, Trigger: trigger, Pressed: pressed})
		}
		return
	}

	var steps []keyseq.Step
	var switched string
	notifySwitch := false

	if pressed && e.active {
		if act, ok := e.lookupLocked(device, trigger); ok {
			if act.Profile != "" {
				if act.Hold {
					e.holdReturn[key] = e.activeProfile
				}
				e.activeProfile = act.Profile
				switched, notifySwitch = act.Profile, true
			} else {
				steps = act.Steps
			}
		}
	}
	if !pressed {
		// Hold releases restore the previous profile even if mapping was
		// disabled or the profile edited mid-hold.
		if ret, ok := e.holdReturn[key]; ok {
			delete(e.holdReturn, key)
			e.activeProfile = ret
			switched, notifySwitch = ret, true
		}
	}
	e.mu.Unlock()

	if notifySwitch && onProfile != nil {
		onProfile(switched)
	}
	if onEvent != nil {
		onEvent(Event{Device: device, Trigger: trigger, Pressed: pressed})
	}
	if len(steps) > 0 {
		select {
		case e.sendQ <- steps:
		default: // drop rather than stall polling if the queue is full
		}
	}
}
