package main

import (
	"voxflow/internal/config"
	"voxflow/internal/macos"
)

// AppRuleDTO is exposed to the frontend for per-app overrides.
type AppRuleDTO struct {
	BundleID       string `json:"bundle_id"`
	AppName        string `json:"app_name,omitempty"`
	RefinementMode string `json:"refinement_mode,omitempty"`
	InjectMethod   string `json:"inject_method,omitempty"`
}

// FrontmostAppInfo describes the currently focused application.
type FrontmostAppInfo struct {
	BundleID string `json:"bundle_id"`
	Name     string `json:"name"`
}

// GetFrontmostApp returns the active macOS application.
func (a *App) GetFrontmostApp() (*FrontmostAppInfo, error) {
	bundleID, name, err := macos.FrontmostApp()
	if err != nil {
		return nil, err
	}
	return &FrontmostAppInfo{BundleID: bundleID, Name: name}, nil
}

// GetAppRules returns all configured per-app rules.
func (a *App) GetAppRules() []AppRuleDTO {
	rules := a.config.GetAppRules()
	out := make([]AppRuleDTO, 0, len(rules))
	for bundleID, rule := range rules {
		out = append(out, AppRuleDTO{
			BundleID:       bundleID,
			RefinementMode: rule.RefinementMode,
			InjectMethod:   rule.InjectMethod,
		})
	}
	return out
}

// SetAppRule saves a per-app refinement/injection override.
func (a *App) SetAppRule(bundleID, refinementMode, injectMethod string) error {
	a.config.SetAppRule(bundleID, config.AppRule{
		RefinementMode: refinementMode,
		InjectMethod:   injectMethod,
	})
	return a.config.Save()
}

// RemoveAppRule deletes a per-app override.
func (a *App) RemoveAppRule(bundleID string) error {
	a.config.RemoveAppRule(bundleID)
	return a.config.Save()
}
