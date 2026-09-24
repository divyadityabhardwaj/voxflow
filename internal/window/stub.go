//go:build !darwin

package window

func FloatEverywhere() {}
func ResetBehavior()   {}
func ConstrainWindow() {}

func InstallStatusItem(StatusItemCallbacks) {}
func SetStatusItemState(string)             {}
func SetUpdateItem(string)                  {}

func observeWindowFrame(*Manager) {}
func setChrome(bool)              {}
