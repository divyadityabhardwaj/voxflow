package config

func (c *Config) GetPushToTalkKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.PushToTalkKey
}

func (c *Config) SetPushToTalkKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.PushToTalkKey = key
}

func (c *Config) GetInputDevice() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.InputDevice
}

func (c *Config) SetInputDevice(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.InputDevice = name
}
