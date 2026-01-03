import { useState, useEffect } from "react";
import {
  GetConfig,
  SetAPIKey,
  SetHotkey,
  SetWhisperModel,
  SetMode,
  GetAllModels,
  DownloadModelByName,
  DeleteModelByName,
  IsWhisperCLIReady,
  CancelDownload,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

interface Config {
  hotkey: string;
  whisper_model: string;
  mode: string;
  api_key_set: boolean;
}

interface ModelInfo {
  name: string;
  description: string;
  size: number;
  downloaded: boolean;
  file_path: string;
}

const HOTKEY_OPTIONS = [
  { value: "cmd+shift+v", label: "⌘ + ⇧ + V" },
  { value: "cmd+shift+d", label: "⌘ + ⇧ + D" },
  { value: "ctrl+shift+v", label: "⌃ + ⇧ + V" },
  { value: "cmd+shift+space", label: "⌘ + ⇧ + Space" },
  { value: "ctrl+shift+space", label: "⌃ + ⇧ + Space" },
];

export default function SettingsView() {
  const [config, setConfig] = useState<Config | null>(null);
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [modelsLoading, setModelsLoading] = useState(true);
  const [modelsError, setModelsError] = useState<string | null>(null);
  const [apiKey, setApiKey] = useState("");
  const [saving, setSaving] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [downloading, setDownloading] = useState<string | null>(null);
  const [downloadProgress, setDownloadProgress] = useState(0);
  const [whisperReady, setWhisperReady] = useState(false);

  useEffect(() => {
    loadConfig();
    loadModels();
    checkWhisperCLI();

    // Listen for download progress
    EventsOn(
      "model-download-progress",
      (data: { model: string; progress: number }) => {
        setDownloadProgress(Math.round(data.progress));
      }
    );

    EventsOn("model-download-complete", () => {
      setDownloading(null);
      setDownloadProgress(0);
      loadModels();
    });
  }, []);

  const loadConfig = async () => {
    try {
      const cfg = await GetConfig();
      setConfig(cfg as Config);
    } catch (err) {
      console.error("Failed to load config:", err);
    }
  };

  const loadModels = async () => {
    setModelsLoading(true);
    setModelsError(null);
    try {
      const modelList = await GetAllModels();
      console.log("Loaded models:", modelList);
      setModels(modelList || []);
    } catch (err) {
      console.error("Failed to load models:", err);
      setModelsError(String(err));
    } finally {
      setModelsLoading(false);
    }
  };

  const checkWhisperCLI = async () => {
    try {
      const ready = await IsWhisperCLIReady();
      setWhisperReady(ready);
    } catch (err) {
      console.error("Failed to check whisper CLI:", err);
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

  const handleModelSelect = async (modelName: string) => {
    setSaving("model");
    try {
      await SetWhisperModel(modelName);
      setConfig((prev) =>
        prev ? { ...prev, whisper_model: modelName } : null
      );
      showSuccess("model");
    } catch (err) {
      console.error("Failed to save model:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleDownloadModel = async (modelName: string) => {
    setDownloading(modelName);
    setDownloadProgress(0);
    try {
      await DownloadModelByName(modelName);
    } catch (err) {
      console.error("Failed to download model:", err);
      setDownloading(null);
    }
  };

  const handleCancelDownload = async () => {
    try {
      await CancelDownload();
      setDownloading(null);
      setDownloadProgress(0);
    } catch (err) {
      console.error("Failed to cancel download:", err);
    }
  };

  const [modelToDelete, setModelToDelete] = useState<string | null>(null);

  const startDeleteModel = (modelName: string) => {
    setModelToDelete(modelName);
  };

  const confirmDeleteModel = async () => {
    if (!modelToDelete) return;

    const modelName = modelToDelete;
    setModelToDelete(null); // Close modal immediately

    try {
      await DeleteModelByName(modelName);
      loadModels();
    } catch (err) {
      console.error("Failed to delete model:", err);
      alert(String(err));
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

  const formatSize = (bytes: number) => {
    if (bytes >= 1024 * 1024 * 1024) {
      return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
    }
    return `${Math.round(bytes / (1024 * 1024))} MB`;
  };

  if (!config) {
    return (
      <div className="flex items-center justify-center h-[calc(100vh-3rem)]">
        <p className="text-dark-500">Loading settings...</p>
      </div>
    );
  }

  return (
    <div className="max-w-2xl mx-auto p-8 overflow-y-auto h-[calc(100vh-3rem)]">
      <h2 className="text-2xl font-semibold text-dark-200 mb-8">Settings</h2>

      <div className="space-y-8">
        {/* Whisper CLI Status */}
        {!whisperReady && (
          <section className="p-4 bg-yellow-900/20 border border-yellow-700 rounded-xl">
            <p className="text-sm text-yellow-400">
              ⚠️ Whisper CLI not found. Please install via:{" "}
              <code className="bg-dark-800 px-2 py-0.5 rounded">
                brew install whisper-cpp
              </code>
            </p>
          </section>
        )}

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

        <section className="p-6 bg-dark-900 rounded-xl border border-dark-800">
          <h3 className="text-lg font-medium text-dark-200 mb-4">
            Speech Recognition Models
          </h3>
          <p className="text-sm text-dark-500 mb-4">
            Download and manage Whisper models. Larger models are more accurate
            but slower.
          </p>
          <div className="space-y-3">
            {modelsLoading ? (
              <p className="text-sm text-dark-500 text-center py-4">
                Loading models...
              </p>
            ) : modelsError ? (
              <div className="p-3 bg-red-900/20 border border-red-800 rounded-lg">
                <p className="text-sm text-red-400">
                  Failed to load models: {modelsError}
                </p>
                <button
                  onClick={loadModels}
                  className="text-xs text-accent-400 mt-2 hover:underline"
                >
                  Retry
                </button>
              </div>
            ) : models.length === 0 ? (
              <p className="text-sm text-dark-500 text-center py-4">
                No models available
              </p>
            ) : (
              models.map((model) => (
                <div
                  key={model.name}
                  className={`flex items-center justify-between p-4 rounded-lg border transition-colors ${
                    config.whisper_model === model.name
                      ? "bg-accent-600/10 border-accent-600"
                      : "border-dark-800 hover:bg-dark-800/50"
                  }`}
                >
                  <div className="flex items-center gap-4">
                    {/* Select radio */}
                    <input
                      type="radio"
                      name="active_model"
                      checked={config.whisper_model === model.name}
                      onChange={() =>
                        model.downloaded && handleModelSelect(model.name)
                      }
                      disabled={!model.downloaded || saving === "model"}
                      className="w-4 h-4 text-accent-600 focus:ring-accent-600"
                    />
                    <div>
                      <p className="text-dark-200 capitalize font-medium">
                        {model.name}
                        {config.whisper_model === model.name && (
                          <span className="ml-2 text-xs text-accent-400">
                            (Active)
                          </span>
                        )}
                      </p>
                      <p className="text-sm text-dark-500">
                        {model.description} • {formatSize(model.size)}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    {model.downloaded ? (
                      <>
                        <span className="text-xs text-idle">✓ Downloaded</span>
                        {config.whisper_model !== model.name && (
                          <button
                            onClick={() => {
                              startDeleteModel(model.name);
                            }}
                            className="p-1.5 text-dark-500 hover:text-red-400 hover:bg-red-900/20 rounded transition-colors"
                            title="Delete model"
                          >
                            <svg
                              className="w-4 h-4"
                              fill="none"
                              stroke="currentColor"
                              viewBox="0 0 24 24"
                            >
                              <path
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                strokeWidth={2}
                                d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                              />
                            </svg>
                          </button>
                        )}
                      </>
                    ) : downloading === model.name ? (
                      <div className="flex items-center gap-2">
                        <div className="w-16 h-2 bg-dark-700 rounded-full overflow-hidden">
                          <div
                            className="h-full bg-accent-500 transition-all duration-300"
                            style={{ width: `${downloadProgress}%` }}
                          />
                        </div>
                        <span className="text-xs text-dark-400 w-8">
                          {downloadProgress}%
                        </span>
                        <button
                          onClick={handleCancelDownload}
                          className="p-1 text-dark-500 hover:text-red-400 hover:bg-red-900/20 rounded transition-colors"
                          title="Cancel download"
                        >
                          <svg
                            className="w-4 h-4"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              strokeWidth={2}
                              d="M6 18L18 6M6 6l12 12"
                            />
                          </svg>
                        </button>
                      </div>
                    ) : (
                      <button
                        onClick={() => handleDownloadModel(model.name)}
                        className="px-3 py-1.5 text-xs bg-accent-600 hover:bg-accent-500 text-white rounded-lg transition-colors"
                      >
                        Download
                      </button>
                    )}
                  </div>
                </div>
              ))
            )}
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

      {/* Delete Confirmation Modal */}
      {modelToDelete && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-xl p-6 w-full max-w-sm shadow-xl transform transition-all scale-100 opacity-100">
            <h3 className="text-lg font-medium text-dark-100 mb-2">
              Delete Model?
            </h3>
            <p className="text-dark-400 text-sm mb-6">
              Are you sure you want to delete the{" "}
              <span className="text-dark-200 font-semibold">
                {modelToDelete}
              </span>{" "}
              model? You will need to download it again to use it.
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setModelToDelete(null)}
                className="px-4 py-2 text-sm text-dark-300 hover:text-dark-100 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={confirmDeleteModel}
                className="px-4 py-2 text-sm bg-red-600 hover:bg-red-500 text-white rounded-lg transition-colors font-medium"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
