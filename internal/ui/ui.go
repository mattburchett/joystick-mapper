// Package ui builds the Fyne desktop interface: a mapping editor window
// with per-game profiles, and a system tray icon for background operation.
package ui

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/mattburchett/joystick-mapper/internal/config"
	"github.com/mattburchett/joystick-mapper/internal/keyseq"
	"github.com/mattburchett/joystick-mapper/internal/mapper"
)

const (
	actionKeys        = "keys"
	actionProfile     = "profile"
	actionProfileHold = "profile-hold"
)

var actionLabels = map[string]string{
	actionKeys:        "Type keys",
	actionProfile:     "Switch profile",
	actionProfileHold: "Hold profile",
}

type UI struct {
	app    fyne.App
	win    fyne.Window
	cfg    *config.Config
	engine *mapper.Engine
	icon   fyne.Resource

	profileSelect *widget.Select
	enabledCheck  *widget.Check
	mappingsBox   *fyne.Container
	devicesLabel  *widget.Label
	statusLabel   *widget.Label
	lastInput     *widget.Label
}

// Run wires everything together and enters the Fyne main loop.
func Run(a fyne.App, cfg *config.Config, engine *mapper.Engine, icon fyne.Resource) {
	u := &UI{app: a, cfg: cfg, engine: engine, icon: icon}
	a.SetIcon(icon)

	u.win = a.NewWindow("Joystick Mapper")
	u.win.Resize(fyne.NewSize(760, 600))
	u.win.SetCloseIntercept(func() { u.win.Hide() }) // close button minimizes to tray

	quit := fyne.NewMenuItem("Quit", a.Quit)
	quit.IsQuit = true
	hide := fyne.NewMenuItem("Hide to Tray", u.win.Hide)
	u.win.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("File", hide, fyne.NewMenuItemSeparator(), quit)))

	u.buildMainWindow()
	u.setupTray()
	u.wireEngine()
	u.refreshMappings()
	u.pushMappings()

	engine.SetActive(cfg.Enabled)
	engine.Start()

	u.win.Show()
	a.Run()
}

func (u *UI) buildMainWindow() {
	// --- profile bar ---
	u.profileSelect = widget.NewSelect(u.cfg.ProfileNames(), func(name string) {
		if name == "" || name == u.cfg.ActiveProfile {
			return
		}
		u.cfg.ActiveProfile = name
		u.save()
		u.refreshMappings()
		u.pushMappings()
		u.refreshTray()
	})
	u.profileSelect.SetSelected(u.cfg.ActiveProfile)

	profileBar := container.NewBorder(nil, nil,
		widget.NewLabelWithStyle("Profile", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(
			widget.NewButtonWithIcon("", theme.ContentAddIcon(), u.newProfileDialog),
			widget.NewButtonWithIcon("", theme.ContentCopyIcon(), u.duplicateProfileDialog),
			widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), u.renameProfileDialog),
			widget.NewButtonWithIcon("", theme.DeleteIcon(), u.deleteProfileDialog),
			widget.NewButtonWithIcon("", theme.SettingsIcon(), u.settingsDialog),
		),
		u.profileSelect,
	)

	// --- devices line ---
	u.devicesLabel = widget.NewLabel("Scanning for joysticks…")
	u.devicesLabel.Wrapping = fyne.TextWrapWord

	// --- enable toggle ---
	u.enabledCheck = widget.NewCheck("Mapping enabled (joystick presses fire actions)", func(v bool) {
		u.setEnabled(v)
	})
	u.enabledCheck.SetChecked(u.cfg.Enabled)

	// --- mappings ---
	u.mappingsBox = container.NewVBox()
	addBtn := widget.NewButtonWithIcon("Add mapping — press a joystick button…", theme.ContentAddIcon(), u.captureDialog)
	addBtn.Importance = widget.HighImportance

	help := widget.NewRichTextFromMarkdown(
		"Sequences: plain text plus tokens — `{ENTER}` `{TAB}` `{ESC}` `{F1}`…`{F12}` `{UP}` `{CTRL+C}` `{SHIFT+ALT+X}` `{WAIT:250}` `{{` for a literal brace. Actions can also switch profiles: **Switch profile** is permanent, **Hold profile** reverts on release (shift layer).")
	help.Wrapping = fyne.TextWrapWord

	// --- status bar ---
	u.statusLabel = widget.NewLabel("Ready.")
	u.lastInput = widget.NewLabelWithStyle("", fyne.TextAlignTrailing, fyne.TextStyle{Italic: true})
	statusBar := container.NewBorder(nil, nil, u.statusLabel, u.lastInput)

	top := container.NewVBox(profileBar, u.devicesLabel, u.enabledCheck, widget.NewSeparator())
	center := container.NewBorder(nil, container.NewVBox(addBtn, help), nil, nil,
		container.NewVScroll(u.mappingsBox))

	u.win.SetContent(container.NewBorder(top, statusBar, nil, nil, center))
}

// shortDevice trims long HID names for display, keeping the distinctive tail.
func shortDevice(name string) string {
	r := []rune(name)
	if len(r) <= 30 {
		return name
	}
	return "…" + strings.TrimSpace(string(r[len(r)-29:]))
}

// --- engine callbacks (arrive on polling goroutines) ---

func (u *UI) wireEngine() {
	u.engine.OnEvent(func(ev mapper.Event) {
		if !ev.Pressed {
			return
		}
		text := fmt.Sprintf("Last input: %s — %s", shortDevice(ev.Device), mapper.TriggerLabel(ev.Trigger))
		fyne.Do(func() { u.lastInput.SetText(text) })
	})
	u.engine.OnDevices(func(devices []mapper.DeviceInfo) {
		fyne.Do(func() {
			if len(devices) == 0 {
				u.devicesLabel.SetText("No joysticks found — waiting for one to be plugged in…")
				return
			}
			names := make([]string, len(devices))
			for i, d := range devices {
				names[i] = fmt.Sprintf("%s (%d buttons, %d axes)", shortDevice(d.Name), d.Buttons, d.Axes)
			}
			u.devicesLabel.SetText("Devices: " + strings.Join(names, " · "))
		})
	})
	u.engine.OnProfileChange(func(name string) {
		fyne.Do(func() {
			if u.cfg.Profile(name) == nil {
				return
			}
			u.cfg.ActiveProfile = name
			u.profileSelect.SetSelected(name) // no-op guard in its callback
			u.refreshMappings()
			u.refreshTray()
			u.statusLabel.SetText("Profile switched to " + name)
		})
	})
}

// --- mappings ---

func (u *UI) refreshMappings() {
	u.mappingsBox.RemoveAll()
	p := u.cfg.Active()
	if len(p.Mappings) == 0 {
		empty := widget.NewLabelWithStyle("No mappings in this profile yet.", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
		u.mappingsBox.Add(empty)
	}
	for i := range p.Mappings {
		u.mappingsBox.Add(u.mappingRow(p, i))
	}
	u.mappingsBox.Refresh()
}

func (u *UI) mappingRow(p *config.Profile, idx int) fyne.CanvasObject {
	m := &p.Mappings[idx]
	if m.Action == "" {
		m.Action = actionKeys
	}

	trigger := widget.NewLabelWithStyle(mapper.TriggerLabel(m.Trigger), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	device := widget.NewLabelWithStyle(shortDevice(m.Device), fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	left := container.NewVBox(trigger, device)

	actionSel := widget.NewSelect(
		[]string{actionLabels[actionKeys], actionLabels[actionProfile], actionLabels[actionProfileHold]},
		func(label string) {
			var action string
			for k, v := range actionLabels {
				if v == label {
					action = k
				}
			}
			if action == m.Action {
				return
			}
			m.Action = action
			u.save()
			u.refreshMappings() // row content depends on the action type
			u.pushMappings()
		})
	actionSel.SetSelected(actionLabels[m.Action])

	var value fyne.CanvasObject
	var buttons []fyne.CanvasObject

	if m.Action == actionKeys {
		entry := widget.NewEntry()
		entry.SetText(m.Sequence)
		entry.PlaceHolder = "keyboard sequence…"
		entry.Validator = func(s string) error {
			_, err := keyseq.Parse(s)
			return err
		}
		entry.OnChanged = func(s string) {
			m.Sequence = s
			u.save()
			u.pushMappings()
		}
		value = entry

		testBtn := widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() {
			steps, err := keyseq.Parse(m.Sequence)
			if err != nil {
				dialog.ShowError(err, u.win)
				return
			}
			u.statusLabel.SetText("Typing in 2 s — focus the target window…")
			go func() {
				time.Sleep(2 * time.Second)
				u.engine.Test(steps)
				fyne.Do(func() { u.statusLabel.SetText("Test sequence sent.") })
			}()
		})
		buttons = append(buttons, testBtn)
	} else {
		profileSel := widget.NewSelect(u.cfg.ProfileNames(), func(name string) {
			if name == m.Profile {
				return
			}
			m.Profile = name
			u.save()
			u.pushMappings()
		})
		profileSel.PlaceHolder = "target profile…"
		if m.Profile != "" {
			profileSel.SetSelected(m.Profile)
		}
		value = profileSel
	}

	deleteBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		p.Mappings = append(p.Mappings[:idx], p.Mappings[idx+1:]...)
		u.save()
		u.refreshMappings()
		u.pushMappings()
	})
	buttons = append(buttons, deleteBtn)

	right := container.NewHBox(append([]fyne.CanvasObject{actionSel}, buttons...)...)
	return container.NewBorder(nil, nil, left, right, value)
}

// pushMappings compiles every profile and hands the tables to the engine.
// Invalid sequences and dangling profile references are skipped until fixed.
func (u *UI) pushMappings() {
	all := make(map[string]map[string]mapper.Action, len(u.cfg.Profiles))
	for _, p := range u.cfg.Profiles {
		table := map[string]mapper.Action{}
		for _, m := range p.Mappings {
			key := mapper.Key(m.Device, m.Trigger)
			switch m.Action {
			case "", actionKeys:
				steps, err := keyseq.Parse(m.Sequence)
				if err == nil && len(steps) > 0 {
					table[key] = mapper.Action{Steps: steps}
				}
			case actionProfile, actionProfileHold:
				if u.cfg.Profile(m.Profile) != nil {
					table[key] = mapper.Action{Profile: m.Profile, Hold: m.Action == actionProfileHold}
				}
			}
		}
		all[p.Name] = table
	}
	u.engine.SetProfiles(all, u.cfg.ActiveProfile)
}

func (u *UI) captureDialog() {
	content := widget.NewLabelWithStyle("Press a joystick button or move an axis…", fyne.TextAlignCenter, fyne.TextStyle{})
	d := dialog.NewCustom("Add mapping", "Cancel", content, u.win)
	d.SetOnClosed(func() { u.engine.CancelCapture() })
	u.engine.StartCapture(func(device, trigger string) {
		fyne.Do(func() {
			d.Hide()
			p := u.cfg.Active()
			for _, m := range p.Mappings {
				if m.Trigger == trigger && m.Device == device {
					u.statusLabel.SetText(mapper.TriggerLabel(trigger) + " is already mapped in this profile.")
					return
				}
			}
			p.Mappings = append(p.Mappings, config.Mapping{Device: device, Trigger: trigger, Action: actionKeys})
			u.save()
			u.refreshMappings()
			u.statusLabel.SetText("Added " + mapper.TriggerLabel(trigger) + " — choose its action.")
		})
	})
	d.Show()
}

// --- profiles ---

func (u *UI) refreshProfiles() {
	u.profileSelect.Options = u.cfg.ProfileNames()
	u.profileSelect.SetSelected(u.cfg.ActiveProfile)
	u.profileSelect.Refresh()
	u.refreshMappings()
	u.pushMappings()
	u.refreshTray()
}

func (u *UI) promptName(title, initial string, cb func(string)) {
	entry := widget.NewEntry()
	entry.SetText(initial)
	items := []*widget.FormItem{widget.NewFormItem("Name", entry)}
	dialog.ShowForm(title, "OK", "Cancel", items, func(ok bool) {
		if ok {
			cb(entry.Text)
		}
	}, u.win)
}

func (u *UI) newProfileDialog() {
	u.promptName("New profile", "", func(name string) {
		if _, err := u.cfg.AddProfile(name); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		u.cfg.ActiveProfile = name
		u.save()
		u.refreshProfiles()
	})
}

func (u *UI) duplicateProfileDialog() {
	src := u.cfg.ActiveProfile
	u.promptName("Duplicate profile "+src, src+" copy", func(name string) {
		if _, err := u.cfg.DuplicateProfile(src, name); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		u.cfg.ActiveProfile = name
		u.save()
		u.refreshProfiles()
	})
}

func (u *UI) renameProfileDialog() {
	old := u.cfg.ActiveProfile
	u.promptName("Rename profile "+old, old, func(name string) {
		if err := u.cfg.RenameProfile(old, name); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		u.save()
		u.refreshProfiles()
	})
}

func (u *UI) deleteProfileDialog() {
	name := u.cfg.ActiveProfile
	dialog.ShowConfirm("Delete profile", fmt.Sprintf("Delete profile %q and its mappings?", name), func(ok bool) {
		if !ok {
			return
		}
		if err := u.cfg.DeleteProfile(name); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		u.save()
		u.refreshProfiles()
	}, u.win)
}

// --- settings ---

func (u *UI) settingsDialog() {
	threshold := widget.NewEntry()
	threshold.SetText(fmt.Sprint(u.cfg.AxisThreshold))
	delay := widget.NewEntry()
	delay.SetText(fmt.Sprint(u.cfg.KeyDelayMs))
	items := []*widget.FormItem{
		widget.NewFormItem("Axis threshold (1–32767)", threshold),
		widget.NewFormItem("Delay between keys (ms)", delay),
	}
	dialog.ShowForm("Settings", "Save", "Cancel", items, func(ok bool) {
		if !ok {
			return
		}
		var t, d int
		if _, err := fmt.Sscan(threshold.Text, &t); err == nil && t > 0 && t <= 32767 {
			u.cfg.AxisThreshold = t
			u.engine.SetAxisThreshold(t)
		}
		if _, err := fmt.Sscan(delay.Text, &d); err == nil && d >= 0 {
			u.cfg.KeyDelayMs = d
			u.engine.SetKeyDelay(time.Duration(d) * time.Millisecond)
		}
		u.save()
	}, u.win)
}

// --- tray ---

func (u *UI) setEnabled(v bool) {
	u.cfg.Enabled = v
	u.engine.SetActive(v)
	u.save()
	if u.enabledCheck.Checked != v {
		u.enabledCheck.SetChecked(v)
	}
	u.refreshTray()
}

func (u *UI) setupTray() {
	if desk, ok := u.app.(desktop.App); ok {
		desk.SetSystemTrayIcon(u.icon)
	}
	u.refreshTray()
}

func (u *UI) refreshTray() {
	desk, ok := u.app.(desktop.App)
	if !ok {
		return
	}

	show := fyne.NewMenuItem("Show Window", func() { u.win.Show(); u.win.RequestFocus() })

	enabled := fyne.NewMenuItem("Mapping Enabled", func() { u.setEnabled(!u.cfg.Enabled) })
	enabled.Checked = u.cfg.Enabled

	var profileItems []*fyne.MenuItem
	for _, name := range u.cfg.ProfileNames() {
		n := name
		item := fyne.NewMenuItem(n, func() {
			u.cfg.ActiveProfile = n
			u.save()
			u.refreshProfiles()
		})
		item.Checked = n == u.cfg.ActiveProfile
		profileItems = append(profileItems, item)
	}
	profiles := fyne.NewMenuItem("Profile", nil)
	profiles.ChildMenu = fyne.NewMenu("", profileItems...)

	quit := fyne.NewMenuItem("Quit", u.app.Quit)
	quit.IsQuit = true

	desk.SetSystemTrayMenu(fyne.NewMenu("Joystick Mapper",
		show, fyne.NewMenuItemSeparator(), enabled, profiles,
		fyne.NewMenuItemSeparator(), quit))
}

func (u *UI) save() {
	if err := u.cfg.Save(); err != nil {
		dialog.ShowError(fmt.Errorf("saving config: %w", err), u.win)
	}
}
