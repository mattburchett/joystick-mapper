// Package config persists devices, profiles, and mappings as JSON in the
// OS config directory (~/.config/joystick-mapper on Linux, %AppData% on
// Windows).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Mapping binds one joystick trigger to an action.
// Trigger forms: "button:N", "axis:N:+", "axis:N:-".
// Device is the joystick name the trigger belongs to; empty matches any
// device. Action selects what happens on press:
//
//	"" or "keys"   type Sequence
//	"profile"      switch to Profile
//	"profile-hold" switch to Profile while held, back on release
type Mapping struct {
	Device   string `json:"device,omitempty"`
	Trigger  string `json:"trigger"`
	Action   string `json:"action,omitempty"`
	Sequence string `json:"sequence,omitempty"`
	Profile  string `json:"profile,omitempty"`
}

// Profile is a named set of mappings, e.g. one per game.
type Profile struct {
	Name     string    `json:"name"`
	Mappings []Mapping `json:"mappings"`
}

type Config struct {
	AxisThreshold int        `json:"axis_threshold"` // 1..32767, axis deflection that counts as a press
	KeyDelayMs    int        `json:"key_delay_ms"`   // pause between keystrokes in a sequence
	Enabled       bool       `json:"enabled"`        // fire mappings on startup
	ActiveProfile string     `json:"active_profile"`
	Profiles      []*Profile `json:"profiles"`
}

func Default() *Config {
	return &Config{
		AxisThreshold: 20000,
		KeyDelayMs:    15,
		Enabled:       true,
		ActiveProfile: "Default",
		Profiles:      []*Profile{{Name: "Default"}},
	}
}

func path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "joystick-mapper", "config.json"), nil
}

// Load reads the config file, returning defaults if it does not exist yet.
func Load() (*Config, error) {
	p, err := path()
	if err != nil {
		return Default(), err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), err
	}
	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return Default(), fmt.Errorf("parsing %s: %w", p, err)
	}
	cfg.normalize()
	return cfg, nil
}

func (c *Config) Save() error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (c *Config) normalize() {
	if len(c.Profiles) == 0 {
		c.Profiles = []*Profile{{Name: "Default"}}
	}
	if c.Profile(c.ActiveProfile) == nil {
		c.ActiveProfile = c.Profiles[0].Name
	}
	if c.AxisThreshold <= 0 || c.AxisThreshold > 32767 {
		c.AxisThreshold = 20000
	}
	if c.KeyDelayMs < 0 {
		c.KeyDelayMs = 15
	}
}

// Profile returns the profile with the given name, or nil.
func (c *Config) Profile(name string) *Profile {
	for _, p := range c.Profiles {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// Active returns the currently selected profile, guaranteed non-nil.
func (c *Config) Active() *Profile {
	if p := c.Profile(c.ActiveProfile); p != nil {
		return p
	}
	c.normalize()
	return c.Profiles[0]
}

// ProfileNames returns profile names in display order.
func (c *Config) ProfileNames() []string {
	names := make([]string, len(c.Profiles))
	for i, p := range c.Profiles {
		names[i] = p.Name
	}
	return names
}

// AddProfile creates an empty profile; errors on duplicate or empty name.
func (c *Config) AddProfile(name string) (*Profile, error) {
	if name == "" {
		return nil, errors.New("profile name cannot be empty")
	}
	if c.Profile(name) != nil {
		return nil, fmt.Errorf("profile %q already exists", name)
	}
	p := &Profile{Name: name}
	c.Profiles = append(c.Profiles, p)
	return p, nil
}

// DuplicateProfile copies an existing profile under a new name.
func (c *Config) DuplicateProfile(src, name string) (*Profile, error) {
	from := c.Profile(src)
	if from == nil {
		return nil, fmt.Errorf("no such profile %q", src)
	}
	p, err := c.AddProfile(name)
	if err != nil {
		return nil, err
	}
	p.Mappings = append([]Mapping(nil), from.Mappings...)
	return p, nil
}

// RenameProfile renames a profile, keeping the active selection in sync.
func (c *Config) RenameProfile(old, name string) error {
	if name == "" {
		return errors.New("profile name cannot be empty")
	}
	if c.Profile(name) != nil {
		return fmt.Errorf("profile %q already exists", name)
	}
	p := c.Profile(old)
	if p == nil {
		return fmt.Errorf("no such profile %q", old)
	}
	p.Name = name
	if c.ActiveProfile == old {
		c.ActiveProfile = name
	}
	return nil
}

// DeleteProfile removes a profile; the last profile cannot be deleted.
func (c *Config) DeleteProfile(name string) error {
	if len(c.Profiles) == 1 {
		return errors.New("cannot delete the last profile")
	}
	for i, p := range c.Profiles {
		if p.Name == name {
			c.Profiles = append(c.Profiles[:i], c.Profiles[i+1:]...)
			if c.ActiveProfile == name {
				c.ActiveProfile = c.Profiles[0].Name
			}
			return nil
		}
	}
	return fmt.Errorf("no such profile %q", name)
}
