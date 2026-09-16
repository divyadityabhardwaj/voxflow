package window

// StatusItemCallbacks are invoked from the menu bar item's menu.
type StatusItemCallbacks struct {
	ToggleRecording func()
	OpenApp         func()
	OpenSettings    func()
	Quit            func()
}
