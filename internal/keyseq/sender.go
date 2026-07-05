package keyseq

import (
	"runtime"
	"sync"
	"time"

	"github.com/micmonay/keybd_event"
)

// Sender synthesizes keystrokes for parsed sequences. It is safe for use
// from a single goroutine at a time; the mapper engine serializes sends.
type Sender struct {
	mu       sync.Mutex
	kb       keybd_event.KeyBonding
	readyAt  time.Time
	KeyDelay time.Duration // pause between keystrokes in a sequence
}

// NewSender creates the virtual keyboard. On Linux this registers a uinput
// device, which the display server takes a moment to pick up — Send blocks
// until the device is ready rather than sleeping here.
func NewSender() (*Sender, error) {
	kb, err := keybd_event.NewKeyBonding()
	if err != nil {
		return nil, err
	}
	s := &Sender{kb: kb, KeyDelay: 15 * time.Millisecond}
	if runtime.GOOS == "linux" {
		s.readyAt = time.Now().Add(2 * time.Second)
	}
	return s, nil
}

// Send executes a parsed sequence, one step at a time.
func (s *Sender) Send(steps []Step) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if wait := time.Until(s.readyAt); wait > 0 {
		time.Sleep(wait)
	}
	for _, st := range steps {
		if st.VK == 0 {
			if st.Wait > 0 {
				time.Sleep(st.Wait)
			}
			continue
		}
		s.kb.Clear()
		s.kb.SetKeys(st.VK)
		s.kb.HasSHIFT(st.Shift)
		s.kb.HasCTRL(st.Ctrl)
		s.kb.HasALT(st.Alt)
		s.kb.HasSuper(st.Super)
		s.kb.HasALTGR(st.AltGr)
		if err := s.kb.Launching(); err != nil {
			return err
		}
		time.Sleep(s.KeyDelay)
	}
	return nil
}
