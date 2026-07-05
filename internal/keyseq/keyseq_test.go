package keyseq

import (
	"testing"
	"time"

	"github.com/micmonay/keybd_event"
)

func TestParseText(t *testing.T) {
	steps, err := Parse("Hi 5!")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 5 {
		t.Fatalf("want 5 steps, got %d: %+v", len(steps), steps)
	}
	if steps[0].VK != keybd_event.VK_H || !steps[0].Shift {
		t.Errorf("H: got %+v", steps[0])
	}
	if steps[1].VK != keybd_event.VK_I || steps[1].Shift {
		t.Errorf("i: got %+v", steps[1])
	}
	if steps[2].VK != keybd_event.VK_SPACE {
		t.Errorf("space: got %+v", steps[2])
	}
	if steps[3].VK != keybd_event.VK_5 || steps[3].Shift {
		t.Errorf("5: got %+v", steps[3])
	}
	if steps[4].VK != keybd_event.VK_1 || !steps[4].Shift {
		t.Errorf("!: got %+v", steps[4])
	}
}

func TestParseTokens(t *testing.T) {
	steps, err := Parse("{ENTER}{F5}{WAIT:250}{CTRL+SHIFT+T}{+}")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 5 {
		t.Fatalf("want 5 steps, got %d", len(steps))
	}
	if steps[0].VK != keybd_event.VK_ENTER {
		t.Errorf("enter: %+v", steps[0])
	}
	if steps[1].VK != keybd_event.VK_F5 {
		t.Errorf("f5: %+v", steps[1])
	}
	if steps[2].Wait != 250*time.Millisecond || steps[2].VK != 0 {
		t.Errorf("wait: %+v", steps[2])
	}
	if steps[3].VK != keybd_event.VK_T || !steps[3].Ctrl || !steps[3].Shift {
		t.Errorf("ctrl+shift+t: %+v", steps[3])
	}
	if steps[4].VK != keybd_event.VK_EQUAL || !steps[4].Shift {
		t.Errorf("plus: %+v", steps[4])
	}
}

func TestParseLiteralBraces(t *testing.T) {
	steps, err := Parse("{{}}")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("want 2 steps, got %d", len(steps))
	}
	if steps[0].VK != keybd_event.VK_LEFTBRACE || !steps[0].Shift {
		t.Errorf("{: %+v", steps[0])
	}
	if steps[1].VK != keybd_event.VK_RIGHTBRACE || !steps[1].Shift {
		t.Errorf("}: %+v", steps[1])
	}
}

func TestParseErrors(t *testing.T) {
	for _, bad := range []string{"{OOPS}", "{unclosed", "{CTRL+}", "{WAIT:x}", "é", "{FOO+A}"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) should fail", bad)
		}
	}
}

func TestParseCaseInsensitiveTokens(t *testing.T) {
	steps, err := Parse("{ctrl+c}{Escape}")
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].VK != keybd_event.VK_C || !steps[0].Ctrl || steps[0].Shift {
		t.Errorf("ctrl+c: %+v", steps[0])
	}
	if steps[1].VK != keybd_event.VK_ESC {
		t.Errorf("escape: %+v", steps[1])
	}
}
