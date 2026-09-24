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
