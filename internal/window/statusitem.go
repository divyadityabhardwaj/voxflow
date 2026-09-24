package window

type StatusItemCallbacks struct {
	ToggleRecording func()
	CancelRecording func()
	OpenApp         func()
	OpenSettings    func()
	Quit            func()
	OpenUpdate      func()
}
