//go:build !darwin

package window

func ResetBehavior()   {}
func ConstrainWindow() {}

func InstallStatusItem(StatusItemCallbacks) {}
func SetStatusItemState(string)             {}
func SetUpdateItem(string)                  {}

func observeWindowFrame(*Manager)         {}
func setChrome(bool)                      {}
func InstallOverlay()                     {}
func setOverlayState(int, string, string) {}
func setOverlayLevel(float64)             {}
