# Joystick Mapper

Map joystick buttons and axes to programmable keyboard sequences, with
per-game profiles and a system tray icon. Linux-first, Windows supported.

Built with [Fyne](https://fyne.io) (native GUI + tray), a direct Linux
kernel joystick backend (no 32-button cap — VKB/Virpil devices with 80+
buttons work), and [keybd_event](https://github.com/micmonay/keybd_event)
for keystroke synthesis (uinput on Linux, SendInput on Windows).

## Usage

All attached joysticks are polled simultaneously — mappings remember which
device they belong to, so a shift button on your throttle can layer your
stick (Joystick Gremlin style).

1. Create a **profile** per game with the `+` button (duplicate, rename,
   delete alongside). Profiles are saved automatically.
2. Click **Add mapping**, press a joystick button (or deflect an axis),
   then choose its action:
   - **Type keys** — a keyboard sequence (syntax below)
   - **Switch profile** — permanently activate another profile
   - **Hold profile** — activate another profile while held, revert on
     release (a shift layer, like Gremlin's temporary mode switch)
3. Toggle **Mapping enabled** to arm or disarm everything. Closing the
   window minimizes to the tray; the tray menu can show the window, switch
   profiles, and toggle mapping.

### Sequence syntax

Plain text is typed as-is (US layout). Tokens in braces:

| Token | Meaning |
|---|---|
| `{ENTER}` `{TAB}` `{ESC}` `{SPACE}` `{BACKSPACE}` `{DELETE}` `{INSERT}` | named keys |
| `{UP}` `{DOWN}` `{LEFT}` `{RIGHT}` `{HOME}` `{END}` `{PGUP}` `{PGDN}` | navigation |
| `{F1}` … `{F12}` | function keys |
| `{CTRL+C}`, `{SHIFT+ALT+X}`, `{SUPER+D}`, `{ALTGR+E}` | modifier combos |
| `{WAIT:250}` | pause 250 ms |
| `{{` and `}}` | literal `{` and `}` |

Example: `gg{ENTER}{WAIT:500}{CTRL+SHIFT+T}`

## Permissions (Linux)

Reading joysticks needs `/dev/input/js*`; sending keys needs `/dev/uinput`.
Most desktop distros grant the former via the `input` group or seat ACLs.
For uinput, either add your user to a group with access or create
`/etc/udev/rules.d/99-uinput.rules`:

```
KERNEL=="uinput", MODE="0660", GROUP="input", OPTIONS+="static_node=uinput"
```

then add yourself to `input` and re-log in.

## Building

Linux (needs gcc plus X11/GL headers — `xorg-dev libgl1-mesa-dev
libxkbcommon-dev` on Debian/Ubuntu; included in base packages on Arch):

```
go build -mod=vendor .
```

Windows (native, needs Go + a MinGW gcc):

```
go build -mod=vendor -ldflags -H=windowsgui .
```

Dependencies are vendored; no network access is needed to build.

## Releases

Pushing a `v*` tag runs the GitHub Actions release workflow, which attaches
`joystick-mapper-linux-amd64` and `joystick-mapper-windows-amd64.exe` to
the release.

## Config

Everything (device, profiles, mappings, settings) lives in one JSON file:
`~/.config/joystick-mapper/config.json` on Linux,
`%AppData%\joystick-mapper\config.json` on Windows.
