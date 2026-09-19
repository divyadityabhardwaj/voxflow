package window

type StatusItemCallbacks struct {
	ToggleRecording func()
	OpenApp         func()
	OpenSettings    func()
	Quit            func()
}
