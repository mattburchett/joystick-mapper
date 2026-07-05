// Package keyseq parses keyboard sequence strings and sends them as
// synthesized keystrokes. Sequences are plain text plus {TOKEN} directives:
//
//	hello world{ENTER}     type text, then press Enter
//	{CTRL+C}               modifier combos (CTRL, ALT, SHIFT, SUPER, ALTGR)
//	{WAIT:250}             pause for 250 milliseconds
//	{F5} {TAB} {ESC} ...   named keys
//	{{ and }}              literal { and }
package keyseq

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/micmonay/keybd_event"
)

// Step is a single action in a sequence: one keystroke (with optional
// modifiers) or a pure delay.
type Step struct {
	VK    int
	Shift bool
	Ctrl  bool
	Alt   bool
	Super bool
	AltGr bool
	Wait  time.Duration // when > 0 and VK == 0, this step is a pure delay
}

type chord struct {
	vk    int
	shift bool
}

var charTable = map[rune]chord{
	'a': {keybd_event.VK_A, false}, 'b': {keybd_event.VK_B, false},
	'c': {keybd_event.VK_C, false}, 'd': {keybd_event.VK_D, false},
	'e': {keybd_event.VK_E, false}, 'f': {keybd_event.VK_F, false},
	'g': {keybd_event.VK_G, false}, 'h': {keybd_event.VK_H, false},
	'i': {keybd_event.VK_I, false}, 'j': {keybd_event.VK_J, false},
	'k': {keybd_event.VK_K, false}, 'l': {keybd_event.VK_L, false},
	'm': {keybd_event.VK_M, false}, 'n': {keybd_event.VK_N, false},
	'o': {keybd_event.VK_O, false}, 'p': {keybd_event.VK_P, false},
	'q': {keybd_event.VK_Q, false}, 'r': {keybd_event.VK_R, false},
	's': {keybd_event.VK_S, false}, 't': {keybd_event.VK_T, false},
	'u': {keybd_event.VK_U, false}, 'v': {keybd_event.VK_V, false},
	'w': {keybd_event.VK_W, false}, 'x': {keybd_event.VK_X, false},
	'y': {keybd_event.VK_Y, false}, 'z': {keybd_event.VK_Z, false},

	'A': {keybd_event.VK_A, true}, 'B': {keybd_event.VK_B, true},
	'C': {keybd_event.VK_C, true}, 'D': {keybd_event.VK_D, true},
	'E': {keybd_event.VK_E, true}, 'F': {keybd_event.VK_F, true},
	'G': {keybd_event.VK_G, true}, 'H': {keybd_event.VK_H, true},
	'I': {keybd_event.VK_I, true}, 'J': {keybd_event.VK_J, true},
	'K': {keybd_event.VK_K, true}, 'L': {keybd_event.VK_L, true},
	'M': {keybd_event.VK_M, true}, 'N': {keybd_event.VK_N, true},
	'O': {keybd_event.VK_O, true}, 'P': {keybd_event.VK_P, true},
	'Q': {keybd_event.VK_Q, true}, 'R': {keybd_event.VK_R, true},
	'S': {keybd_event.VK_S, true}, 'T': {keybd_event.VK_T, true},
	'U': {keybd_event.VK_U, true}, 'V': {keybd_event.VK_V, true},
	'W': {keybd_event.VK_W, true}, 'X': {keybd_event.VK_X, true},
	'Y': {keybd_event.VK_Y, true}, 'Z': {keybd_event.VK_Z, true},

	'0': {keybd_event.VK_0, false}, '1': {keybd_event.VK_1, false},
	'2': {keybd_event.VK_2, false}, '3': {keybd_event.VK_3, false},
	'4': {keybd_event.VK_4, false}, '5': {keybd_event.VK_5, false},
	'6': {keybd_event.VK_6, false}, '7': {keybd_event.VK_7, false},
	'8': {keybd_event.VK_8, false}, '9': {keybd_event.VK_9, false},

	' ':  {keybd_event.VK_SPACE, false},
	'\n': {keybd_event.VK_ENTER, false},
	'\t': {keybd_event.VK_TAB, false},

	'-':  {keybd_event.VK_MINUS, false},
	'=':  {keybd_event.VK_EQUAL, false},
	'[':  {keybd_event.VK_LEFTBRACE, false},
	']':  {keybd_event.VK_RIGHTBRACE, false},
	'\\': {keybd_event.VK_BACKSLASH, false},
	';':  {keybd_event.VK_SEMICOLON, false},
	'\'': {keybd_event.VK_APOSTROPHE, false},
	',':  {keybd_event.VK_COMMA, false},
	'.':  {keybd_event.VK_DOT, false},
	'/':  {keybd_event.VK_SLASH, false},
	'`':  {keybd_event.VK_GRAVE, false},

	'!': {keybd_event.VK_1, true}, '@': {keybd_event.VK_2, true},
	'#': {keybd_event.VK_3, true}, '$': {keybd_event.VK_4, true},
	'%': {keybd_event.VK_5, true}, '^': {keybd_event.VK_6, true},
	'&': {keybd_event.VK_7, true}, '*': {keybd_event.VK_8, true},
	'(': {keybd_event.VK_9, true}, ')': {keybd_event.VK_0, true},
	'_': {keybd_event.VK_MINUS, true}, '+': {keybd_event.VK_EQUAL, true},
	'{': {keybd_event.VK_LEFTBRACE, true}, '}': {keybd_event.VK_RIGHTBRACE, true},
	'|': {keybd_event.VK_BACKSLASH, true}, ':': {keybd_event.VK_SEMICOLON, true},
	'"': {keybd_event.VK_APOSTROPHE, true}, '<': {keybd_event.VK_COMMA, true},
	'>': {keybd_event.VK_DOT, true}, '?': {keybd_event.VK_SLASH, true},
	'~': {keybd_event.VK_GRAVE, true},
}

var namedKeys = map[string]int{
	"ENTER": keybd_event.VK_ENTER, "RETURN": keybd_event.VK_ENTER,
	"TAB": keybd_event.VK_TAB,
	"ESC": keybd_event.VK_ESC, "ESCAPE": keybd_event.VK_ESC,
	"SPACE":     keybd_event.VK_SPACE,
	"BACKSPACE": keybd_event.VK_BACKSPACE,
	"DELETE":    keybd_event.VK_DELETE, "DEL": keybd_event.VK_DELETE,
	"INSERT": keybd_event.VK_INSERT, "INS": keybd_event.VK_INSERT,
	"HOME": keybd_event.VK_HOME, "END": keybd_event.VK_END,
	"PAGEUP": keybd_event.VK_PAGEUP, "PGUP": keybd_event.VK_PAGEUP,
	"PAGEDOWN": keybd_event.VK_PAGEDOWN, "PGDN": keybd_event.VK_PAGEDOWN,
	"UP": keybd_event.VK_UP, "DOWN": keybd_event.VK_DOWN,
	"LEFT": keybd_event.VK_LEFT, "RIGHT": keybd_event.VK_RIGHT,
	"CAPSLOCK": keybd_event.VK_CAPSLOCK,
	"NUMLOCK":  keybd_event.VK_NUMLOCK,
	"PAUSE":    keybd_event.VK_PAUSE,
	"F1":       keybd_event.VK_F1, "F2": keybd_event.VK_F2,
	"F3": keybd_event.VK_F3, "F4": keybd_event.VK_F4,
	"F5": keybd_event.VK_F5, "F6": keybd_event.VK_F6,
	"F7": keybd_event.VK_F7, "F8": keybd_event.VK_F8,
	"F9": keybd_event.VK_F9, "F10": keybd_event.VK_F10,
	"F11": keybd_event.VK_F11, "F12": keybd_event.VK_F12,
}

// Parse converts a sequence string into executable steps.
func Parse(s string) ([]Step, error) {
	var steps []Step
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == '{' && i+1 < len(runes) && runes[i+1] == '{':
			steps = append(steps, chordStep(charTable['{']))
			i++
		case r == '}' && i+1 < len(runes) && runes[i+1] == '}':
			steps = append(steps, chordStep(charTable['}']))
			i++
		case r == '{':
			end := indexRune(runes, i+1, '}')
			if end < 0 {
				return nil, fmt.Errorf("unclosed '{' at position %d (use {{ for a literal brace)", i+1)
			}
			step, err := parseToken(string(runes[i+1 : end]))
			if err != nil {
				return nil, err
			}
			steps = append(steps, step)
			i = end
		default:
			c, ok := charTable[r]
			if !ok {
				return nil, fmt.Errorf("unsupported character %q (US layout characters only)", r)
			}
			steps = append(steps, chordStep(c))
		}
	}
	return steps, nil
}

func chordStep(c chord) Step {
	return Step{VK: c.vk, Shift: c.shift}
}

func indexRune(runes []rune, from int, want rune) int {
	for i := from; i < len(runes); i++ {
		if runes[i] == want {
			return i
		}
	}
	return -1
}

func parseToken(token string) (Step, error) {
	upper := strings.ToUpper(strings.TrimSpace(token))
	if rest, ok := strings.CutPrefix(upper, "WAIT:"); ok {
		ms, err := strconv.Atoi(strings.TrimSpace(rest))
		if err != nil || ms < 0 {
			return Step{}, fmt.Errorf("invalid wait duration in {%s}", token)
		}
		return Step{Wait: time.Duration(ms) * time.Millisecond}, nil
	}

	parts := strings.Split(upper, "+")
	// "{+}" is a literal plus keystroke
	if upper == "+" {
		return chordStep(charTable['+']), nil
	}
	var step Step
	for idx, p := range parts {
		p = strings.TrimSpace(p)
		last := idx == len(parts)-1
		if !last {
			switch p {
			case "CTRL", "CONTROL":
				step.Ctrl = true
			case "ALT":
				step.Alt = true
			case "SHIFT":
				step.Shift = true
			case "SUPER", "WIN", "META":
				step.Super = true
			case "ALTGR":
				step.AltGr = true
			default:
				return Step{}, fmt.Errorf("unknown modifier %q in {%s}", p, token)
			}
			continue
		}
		if vk, ok := namedKeys[p]; ok {
			step.VK = vk
			continue
		}
		if len([]rune(p)) == 1 {
			r := []rune(strings.ToLower(p))[0]
			if c, ok := charTable[r]; ok {
				step.VK = c.vk
				step.Shift = step.Shift || c.shift
				continue
			}
		}
		return Step{}, fmt.Errorf("unknown key %q in {%s}", p, token)
	}
	if step.VK == 0 {
		return Step{}, fmt.Errorf("no key in token {%s}", token)
	}
	return step, nil
}
