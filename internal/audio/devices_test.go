package audio

import "testing"

func TestListInputDevices(t *testing.T) {
	r := NewRecorder()
	defer r.Terminate()
	devices, err := r.ListInputDevices()
	if err != nil || len(devices) == 0 {
		t.Skipf("no input devices here: %v", err)
	}
	defaults := 0
	for _, d := range devices {
		if d.Name == "" {
			t.Errorf("unnamed device in %v", devices)
		}
		if d.Default {
			defaults++
		}
	}
	if defaults > 1 {
		t.Errorf("%d defaults in %v", defaults, devices)
	}
	t.Logf("inputs: %v", devices)
}
