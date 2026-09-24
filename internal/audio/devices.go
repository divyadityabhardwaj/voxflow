package audio

import "github.com/gordonklaus/portaudio"

type InputDevice struct {
	Name    string `json:"name"`
	Default bool   `json:"default"` // the current system default input
}

// ListInputDevices returns the microphones. When idle it re-reads the device
// list first, since PortAudio only enumerates at initialisation.
func (r *Recorder) ListInputDevices() ([]InputDevice, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.initializeLocked(); err != nil { // a no-op while a stream is open
		return nil, err
	}
	devices, err := portaudio.Devices()
	if err != nil {
		return nil, err
	}
	def, _ := portaudio.DefaultInputDevice()
	var out []InputDevice
	for _, d := range devices {
		if d.MaxInputChannels > 0 {
			out = append(out, InputDevice{Name: d.Name, Default: def != nil && d.Name == def.Name})
		}
	}
	return out, nil
}

// SetInputDevice picks the microphone by name for the next recording; "" is the system default.
func (r *Recorder) SetInputDevice(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deviceName = name
}

// MissingDevice names the chosen microphone when the last Start couldn't find
// it and used the default instead.
func (r *Recorder) MissingDevice() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.missingDevice
}

// inputDeviceLocked finds the chosen microphone, falling back to the default.
func (r *Recorder) inputDeviceLocked() (*portaudio.DeviceInfo, error) {
	r.missingDevice = ""
	if r.deviceName != "" {
		if devices, err := portaudio.Devices(); err == nil {
			for _, d := range devices {
				if d.Name == r.deviceName && d.MaxInputChannels > 0 {
					return d, nil
				}
			}
		}
		r.missingDevice = r.deviceName
	}
	return portaudio.DefaultInputDevice()
}
