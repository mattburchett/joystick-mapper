// Joystick Mapper maps joystick buttons and axes to programmable keyboard
// sequences, with per-game profiles and a system tray icon.
package main

import (
	_ "embed"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/mattburchett/joystick-mapper/internal/config"
	"github.com/mattburchett/joystick-mapper/internal/keyseq"
	"github.com/mattburchett/joystick-mapper/internal/mapper"
	"github.com/mattburchett/joystick-mapper/internal/ui"
)

//go:embed assets/icon.png
var iconPNG []byte

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("config: %v (using defaults)", err)
	}

	sender, err := keyseq.NewSender()
	if err != nil {
		log.Fatalf("cannot create virtual keyboard: %v\n"+
			"On Linux this needs write access to /dev/uinput "+
			"(add a udev rule or your user to the appropriate group).", err)
	}
	sender.KeyDelay = time.Duration(cfg.KeyDelayMs) * time.Millisecond

	engine := mapper.New(sender, cfg.AxisThreshold)
	defer engine.Stop()

	a := app.NewWithID("com.mattburchett.joystick-mapper")
	icon := fyne.NewStaticResource("icon.png", iconPNG)
	ui.Run(a, cfg, engine, icon)
}
