package main

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"voxflow/internal/config"
	"voxflow/internal/macos"
)

type AppRuleDTO struct {
	BundleID       string `json:"bundle_id"`
	AppName        string `json:"app_name,omitempty"`
	RefinementMode string `json:"refinement_mode,omitempty"`
	InjectMethod   string `json:"inject_method,omitempty"`
}

type FrontmostAppInfo struct {
	BundleID string `json:"bundle_id"`
	Name     string `json:"name"`
}

// Rules store only the bundle ID, so names are looked up and cached here.
var appNames sync.Map

func (a *App) GetFrontmostApp() (*FrontmostAppInfo, error) {
	bundleID, name, err := macos.FrontmostApp()
	if err != nil {
		return nil, err
	}
	appNames.Store(bundleID, name)
	return &FrontmostAppInfo{BundleID: bundleID, Name: name}, nil
}

func appDisplayName(bundleID string) string {
	if name, ok := appNames.Load(bundleID); ok {
		return name.(string)
	}
	// Spotlight finds the installed app without launching it.
	var name string
	out, _ := exec.Command("mdfind", "kMDItemCFBundleIdentifier == "+strconv.Quote(bundleID)).Output()
	if path, _, _ := strings.Cut(string(out), "\n"); path != "" {
		name = strings.TrimSuffix(filepath.Base(path), ".app")
	}
	appNames.Store(bundleID, name)
	return name
}

func (a *App) GetAppRules() []AppRuleDTO {
	rules := a.config.GetAppRules()
	out := make([]AppRuleDTO, 0, len(rules))
	for bundleID, rule := range rules {
		out = append(out, AppRuleDTO{
			BundleID:       bundleID,
			AppName:        appDisplayName(bundleID),
			RefinementMode: rule.RefinementMode,
			InjectMethod:   rule.InjectMethod,
		})
	}
	return out
}

func (a *App) SetAppRule(bundleID, refinementMode, injectMethod string) error {
	a.config.SetAppRule(bundleID, config.AppRule{
		RefinementMode: refinementMode,
		InjectMethod:   injectMethod,
	})
	return a.config.Save()
}

func (a *App) RemoveAppRule(bundleID string) error {
	a.config.RemoveAppRule(bundleID)
	return a.config.Save()
}
