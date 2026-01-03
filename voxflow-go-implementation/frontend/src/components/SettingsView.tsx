import { useState, useEffect } from "react";
import {
  GetConfig,
  SetAPIKey,
  SetHotkey,
  SetWhisperModel,
  SetMode,
} from "../../wailsjs/go/main/App";

interface Config {
  hotkey: string;
  whisper_model: string;
  mode: string;
  api_key_set: boolean;
}

const HOTKEY_OPTIONS = [
  { value: "cmd+shift+v", label: "⌘ + ⇧ + V" },
  { value: "cmd+shift+d", label: "⌘ + ⇧ + D" },
  { value: "ctrl+shift+v", label: "⌃ + ⇧ + V" },
  { value: "cmd+shift+space", label: "⌘ + ⇧ + Space" },
  { value: "ctrl+shift+space", label: "⌃ + ⇧ + Space" },
];

const MODEL_OPTIONS = [
  {
    value: "tiny",
    label: "Tiny (~75 MB)",
    description: "Fastest, least accurate",
  },
  {
    value: "base",
    label: "Base (~142 MB)",
    description: "Good balance of speed and accuracy",
  },
  {
    value: "small",
    label: "Small (~466 MB)",
    description: "Better accuracy, slower",
  },
  {
    value: "medium",
    label: "Medium (~1.5 GB)",
    description: "Best accuracy, slowest",
  },
];

export default function SettingsView() {
  const [config, setConfig] = useState<Config | null>(null);
  const [apiKey, setApiKey] = useState("");
  const [saving, setSaving] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    try {
      const cfg = await GetConfig();
      setConfig(cfg as Config);
    } catch (err) {
      console.error("Failed to load config:", err);
    }
  };

  const showSuccess = (field: string) => {
    setSuccess(field);
    setTimeout(() => setSuccess(null), 2000);
  };

  const handleSaveApiKey = async () => {
    if (!apiKey.trim()) return;
    setSaving("apiKey");
    try {
      await SetAPIKey(apiKey);
      setConfig((prev) => (prev ? { ...prev, api_key_set: true } : null));
      setApiKey("");
      showSuccess("apiKey");
    } catch (err) {
      console.error("Failed to save API key:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleHotkeyChange = async (value: string) => {
    setSaving("hotkey");
    try {
      await SetHotkey(value);
      setConfig((prev) => (prev ? { ...prev, hotkey: value } : null));
      showSuccess("hotkey");
    } catch (err) {
      console.error("Failed to save hotkey:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleModelChange = async (value: string) => {
    setSaving("model");
    try {
      await SetWhisperModel(value);
      setConfig((prev) => (prev ? { ...prev, whisper_model: value } : null));
      showSuccess("model");
    } catch (err) {
      console.error("Failed to save model:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleModeChange = async (value: string) => {
    setSaving("mode");
    try {
      await SetMode(value);
      setConfig((prev) => (prev ? { ...prev, mode: value } : null));
      showSuccess("mode");
    } catch (err) {
      console.error("Failed to save mode:", err);
    } finally {
      setSaving(null);
    }
  };

  if (!config) {
    return (
      <div className="flex items-center justify-center h-[calc(100vh-3rem)]">
        <p className="text-dark-500">Loading settings...</p>
      </div>
    );
  }

  return (
    <div className="max-w-2xl mx-auto p-8">
      <h2 className="text-2xl font-semibold text-dark-200 mb-8">Settings</h2>

      <div className="space-y-8">
        {/* API Key */}
        <section className="p-6 bg-dark-900 rounded-xl border border-dark-800">
          <h3 className="text-lg font-medium text-dark-200 mb-4">
            Gemini API Key
          </h3>
          <p className="text-sm text-dark-500 mb-4">
            Get your API key from{" "}
            <a
              href="https://makersuite.google.com/app/apikey"
              target="_blank"
              rel="noopener noreferrer"
              className="text-accent-400 hover:underline"
            >
              Google AI Studio
            </a>
          </p>
          <div className="flex gap-3">
            <div className="relative flex-1">
              <input
                type="password"
                placeholder={
                  config.api_key_set ? "••••••••••••••••" : "Enter your API key"
                }
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg
                         text-dark-200 placeholder-dark-500
                         focus:outline-none focus:ring-2 focus:ring-accent-600"
              />
              {config.api_key_set && (
                <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-idle">
                  ✓ Set
                </span>
              )}
            </div>
            <button
              onClick={handleSaveApiKey}
              disabled={!apiKey.trim() || saving === "apiKey"}
              className="px-5 py-2.5 bg-accent-600 hover:bg-accent-500 disabled:opacity-50
                       text-white rounded-lg transition-colors"
            >
              {saving === "apiKey"
                ? "Saving..."
                : success === "apiKey"
                ? "Saved!"
                : "Save"}
            </button>
          </div>
        </section>

        {/* Hotkey */}
        <section className="p-6 bg-dark-900 rounded-xl border border-dark-800">
          <h3 className="text-lg font-medium text-dark-200 mb-4">
            Global Hotkey
          </h3>
          <p className="text-sm text-dark-500 mb-4">
            Press this keyboard shortcut to start/stop recording from any
            application.
          </p>
          <div className="flex gap-3">
            <select
              value={config.hotkey}
              onChange={(e) => handleHotkeyChange(e.target.value)}
              disabled={saving === "hotkey"}
              className="flex-1 px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg
                       text-dark-200 focus:outline-none focus:ring-2 focus:ring-accent-600"
            >
              {HOTKEY_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
            {success === "hotkey" && (
              <span className="flex items-center text-sm text-idle">
                ✓ Saved
              </span>
            )}
          </div>
        </section>

        {/* Whisper Model */}
        <section className="p-6 bg-dark-900 rounded-xl border border-dark-800">
          <h3 className="text-lg font-medium text-dark-200 mb-4">
            Whisper Model
          </h3>
          <p className="text-sm text-dark-500 mb-4">
            Larger models are more accurate but slower. The model will be
            downloaded if not present.
          </p>
          <div className="space-y-2">
            {MODEL_OPTIONS.map((opt) => (
              <label
                key={opt.value}
                className={`flex items-center gap-4 p-4 rounded-lg border cursor-pointer transition-colors ${
                  config.whisper_model === opt.value
                    ? "bg-accent-600/10 border-accent-600"
                    : "border-dark-800 hover:bg-dark-800"
                }`}
              >
                <input
                  type="radio"
                  name="whisper_model"
                  value={opt.value}
                  checked={config.whisper_model === opt.value}
                  onChange={(e) => handleModelChange(e.target.value)}
                  disabled={saving === "model"}
                  className="w-4 h-4 text-accent-600 focus:ring-accent-600"
                />
                <div>
                  <p className="text-dark-200">{opt.label}</p>
                  <p className="text-sm text-dark-500">{opt.description}</p>
                </div>
              </label>
            ))}
          </div>
        </section>

        {/* Mode */}
        <section className="p-6 bg-dark-900 rounded-xl border border-dark-800">
          <h3 className="text-lg font-medium text-dark-200 mb-4">
            Transcription Mode
          </h3>
          <p className="text-sm text-dark-500 mb-4">
            Choose how Gemini should refine your transcriptions.
          </p>
          <div className="flex gap-4">
            <button
              onClick={() => handleModeChange("casual")}
              disabled={saving === "mode"}
              className={`flex-1 p-4 rounded-lg border transition-colors ${
                config.mode === "casual"
                  ? "bg-accent-600/10 border-accent-600"
                  : "border-dark-800 hover:bg-dark-800"
              }`}
            >
              <p className="font-medium text-dark-200">Casual</p>
              <p className="text-sm text-dark-500 mt-1">
                Conversational, natural
              </p>
            </button>
            <button
              onClick={() => handleModeChange("formal")}
              disabled={saving === "mode"}
              className={`flex-1 p-4 rounded-lg border transition-colors ${
                config.mode === "formal"
                  ? "bg-accent-600/10 border-accent-600"
                  : "border-dark-800 hover:bg-dark-800"
              }`}
            >
              <p className="font-medium text-dark-200">Formal</p>
              <p className="text-sm text-dark-500 mt-1">
                Professional, polished
              </p>
            </button>
          </div>
        </section>
      </div>
    </div>
  );
}
