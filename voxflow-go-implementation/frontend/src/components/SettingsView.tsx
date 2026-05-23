import { useState, useEffect } from "react";
import {
  GetConfig,
  SetAPIKey,
  SetOpenRouterAPIKey,
  SetHotkey,
  SetHandsFreeHotkey,
  SetPushToTalkHotkey,
  SetWhisperModel,
  GetAllModels,
  DownloadModelByName,
  DeleteModelByName,
  IsWhisperCLIReady,
  CancelDownload,
  GetGeminiModels,
  SetGeminiModel,
  CheckGeminiModel,
  GetOpenRouterModels,
  SetLLMProvider,
  SetOpenRouterModel,
  CheckOpenRouterModel,
  GetGroqModels,
  SetGroqAPIKey,
  SetGroqModel,
  CheckGroqModel,
  GetCerebrasModels,
  SetCerebrasAPIKey,
  SetCerebrasModel,
  CheckCerebrasModel,
  GetLocalURL,
  SetLocalURL,
  GetLocalModel,
  SetLocalModel,
  CheckLocalModel,
  SetRefinementMode,
  SetMuteSystemAudio,
} from "../../wailsjs/go/main/App";

import { EventsOn } from "../../wailsjs/runtime/runtime";
import HotkeyRecorderModal from "./HotkeyRecorderModal";
import HotkeyInput from "./HotkeyInput";
import { Events } from "../constants/events";
import LLMProviderSettings from "./settings/LLMProviderSettings";
import ModelSelectionSettings from "./settings/ModelSelectionSettings";
import PipelineSettings from "./settings/PipelineSettings";
import HotkeySettings from "./settings/HotkeySettings";
import AppRulesSettings from "./settings/AppRulesSettings";
import AppearanceSettings from "./ui/AppearanceSettings";
import SettingsSection from "./ui/SettingsSection";

interface Config {
  hands_free_hotkey: string;
  push_to_talk_hotkey: string;
  hotkey: string;
  whisper_model: string;
  gemini_model: string;
  api_key_set: boolean;
  llm_provider: string;
  openrouter_model: string;
  openrouter_api_key_set: boolean;
  groq_model: string;
  groq_api_key_set: boolean;
  cerebras_model: string;
  cerebras_api_key_set: boolean;

  local_url: string;
  local_model: string;
  refinement_mode: string;
  mute_system_audio: boolean;
}

interface ModelInfo {
  name: string;
  description: string;
  size: number;
  downloaded: boolean;
  file_path: string;
}

interface ModelStatus {
  latency?: number;
  tps?: number;
  working?: boolean;
  checking?: boolean;
}

export default function SettingsView() {
  const [config, setConfig] = useState<Config | null>(null);
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [modelsLoading, setModelsLoading] = useState(true);
  const [modelsError, setModelsError] = useState<string | null>(null);
  const [apiKey, setApiKey] = useState("");
  const [openRouterApiKey, setOpenRouterApiKey] = useState("");
  const [saving, setSaving] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [downloading, setDownloading] = useState<string | null>(null);
  const [downloadProgress, setDownloadProgress] = useState(0);
  const [whisperReady, setWhisperReady] = useState(false);

  // Gemini Model State
  const [geminiModels, setGeminiModels] = useState<string[]>([]);
  const [geminiModelsLoading, setGeminiModelsLoading] = useState(false);
  const [geminiModelsError, setGeminiModelsError] = useState<string | null>(
    null,
  );

  // OpenRouter Model State
  const [openRouterModels, setOpenRouterModels] = useState<string[]>([]);
  const [openRouterModelsLoading, setOpenRouterModelsLoading] = useState(false);

  // Groq Model State
  const [groqModels, setGroqModels] = useState<string[]>([]);
  const [groqModelsLoading, setGroqModelsLoading] = useState(false);
  const [groqApiKey, setGroqApiKey] = useState("");

  // Cerebras Model State
  const [cerebrasModels, setCerebrasModels] = useState<string[]>([]);
  const [cerebrasModelsLoading, setCerebrasModelsLoading] = useState(false);
  const [cerebrasApiKey, setCerebrasApiKey] = useState("");

  // Local Server State
  const [localURL, setLocalURL] = useState("http://localhost:11434");
  const [localModel, setLocalModel] = useState("");
  const [localCheckResult, setLocalCheckResult] = useState<{
    latency: number;
    tps: number;
  } | null>(null);
  const [localCheckError, setLocalCheckError] = useState<string | null>(null);

  // Model status (latency for each model)
  const [modelStatuses, setModelStatuses] = useState<
    Record<string, ModelStatus>
  >({});

  const [checkingModel, setCheckingModel] = useState<string | null>(null);

  useEffect(() => {
    loadConfig();
    loadModels();
    checkWhisperCLI();
    setModelStatuses({});

    // Listen for download progress
    EventsOn(
      Events.ModelDownloadProgress,
      (data: { model: string; progress: number }) => {
        setDownloadProgress(Math.round(data.progress));
      },
    );

    EventsOn(Events.ModelDownloadComplete, () => {
      setDownloading(null);
      setDownloadProgress(0);
      loadModels();
    });
  }, []);

  // Load Gemini models when config (and thus API key) is loaded
  useEffect(() => {
    if (
      config?.api_key_set &&
      (config?.llm_provider === "gemini" || !config?.llm_provider)
    ) {
      loadGeminiModels();
    }
  }, [config?.api_key_set, config?.llm_provider]);

  // Load OpenRouter models when provider is OpenRouter
  useEffect(() => {
    if (config?.llm_provider === "openrouter") {
      loadOpenRouterModels();
    } else if (config?.llm_provider === "groq") {
      loadGroqModels();
    } else if (config?.llm_provider === "cerebras") {
      loadCerebrasModels();
    }
  }, [config?.llm_provider]);

  const loadConfig = async () => {
    try {
      const cfg = await GetConfig();
      setConfig(cfg as Config);
      setLocalURL((cfg as Config).local_url || "http://localhost:11434");
      setLocalModel((cfg as Config).local_model || "");
    } catch (err) {
      console.error("Failed to load config:", err);
    }
  };

  const loadGeminiModels = async () => {
    setGeminiModelsLoading(true);
    setGeminiModelsError(null);
    try {
      const models = await GetGeminiModels();
      console.log("Loaded Gemini models:", models);
      const modelsList = models || [];
      setGeminiModels(modelsList);
      if (modelsList.length > 0) {
        setConfig((prev) => {
          if (!prev) return null;
          const currentModel = prev.gemini_model;
          if (!currentModel || !modelsList.includes(currentModel)) {
            const defaultModel = modelsList.find(m => m.includes("gemini-1.5-flash")) || 
                                 modelsList.find(m => m.includes("flash")) || 
                                 modelsList[0];
            if (defaultModel) {
              SetGeminiModel(defaultModel).catch(err => console.error("Failed to auto-set Gemini model:", err));
              return { ...prev, gemini_model: defaultModel };
            }
          }
          return prev;
        });
      }
    } catch (err) {
      console.error("Failed to load Gemini models:", err);
      setGeminiModelsError("Failed to fetch models. Check your API key.");
    } finally {
      setGeminiModelsLoading(false);
    }
  };

  const loadOpenRouterModels = async () => {
    setOpenRouterModelsLoading(true);
    try {
      const models = await GetOpenRouterModels();
      console.log("Loaded OpenRouter models:", models);
      const modelsList = models || [];
      setOpenRouterModels(modelsList);
      if (modelsList.length > 0) {
        setConfig((prev) => {
          if (!prev) return null;
          const currentModel = prev.openrouter_model;
          if (!currentModel || !modelsList.includes(currentModel)) {
            const defaultModel = modelsList.find(m => m.includes("free")) || modelsList[0];
            if (defaultModel) {
              SetOpenRouterModel(defaultModel).catch(err => console.error("Failed to auto-set OpenRouter model:", err));
              return { ...prev, openrouter_model: defaultModel };
            }
          }
          return prev;
        });
      }
    } catch (err) {
      console.error("Failed to load OpenRouter models:", err);
    } finally {
      setOpenRouterModelsLoading(false);
    }
  };

  const loadGroqModels = async () => {
    setGroqModelsLoading(true);
    try {
      const models = await GetGroqModels();
      console.log("Loaded Groq models:", models);
      const modelsList = models || [];
      setGroqModels(modelsList);
      if (modelsList.length > 0) {
        setConfig((prev) => {
          if (!prev) return null;
          const currentModel = prev.groq_model;
          if (!currentModel || !modelsList.includes(currentModel)) {
            const defaultModel = modelsList.find(m => m.includes("llama-3.1-8b-instant")) || 
                                 modelsList.find(m => m.includes("llama3")) || 
                                 modelsList[0];
            if (defaultModel) {
              SetGroqModel(defaultModel).catch(err => console.error("Failed to auto-set Groq model:", err));
              return { ...prev, groq_model: defaultModel };
            }
          }
          return prev;
        });
      }
    } catch (err) {
      console.error("Failed to load Groq models:", err);
    } finally {
      setGroqModelsLoading(false);
    }
  };

  const loadCerebrasModels = async () => {
    setCerebrasModelsLoading(true);
    try {
      const models = await GetCerebrasModels();
      console.log("Loaded Cerebras models:", models);
      const modelsList = models || [];
      setCerebrasModels(modelsList);
      if (modelsList.length > 0) {
        setConfig((prev) => {
          if (!prev) return null;
          const currentModel = prev.cerebras_model;
          if (!currentModel || !modelsList.includes(currentModel)) {
            const defaultModel = modelsList.find(m => m.includes("llama3.1-8b")) || modelsList[0];
            if (defaultModel) {
              SetCerebrasModel(defaultModel).catch(err => console.error("Failed to auto-set Cerebras model:", err));
              return { ...prev, cerebras_model: defaultModel };
            }
          }
          return prev;
        });
      }
    } catch (err) {
      console.error("Failed to load Cerebras models:", err);
    } finally {
      setCerebrasModelsLoading(false);
    }
  };

  const checkGeminiModelStatus = async (model: string) => {
    setCheckingModel(model);
    setModelStatuses((prev) => ({ ...prev, [model]: { checking: true } }));
    console.log(`Checking status for Gemini model: ${model}...`);

    try {
      const result = (await CheckGeminiModel(model)) as {
        latency: number;
        tps: number;
      };
      console.log(
        `Model ${model} is WORKING (${result.latency}ms, ${result.tps.toFixed(1)} t/s).`,
      );
      setModelStatuses((prev) => ({
        ...prev,
        [model]: { working: true, latency: result.latency, tps: result.tps },
      }));
    } catch (err) {
      console.error(`Model ${model} check FAILED:`, err);
      setModelStatuses((prev) => ({
        ...prev,
        [model]: { working: false },
      }));
    } finally {
      setCheckingModel(null);
    }
  };

  const checkOpenRouterModelStatus = async (model: string) => {
    setCheckingModel(model);
    setModelStatuses((prev) => ({ ...prev, [model]: { checking: true } }));
    console.log(`Checking status for OpenRouter model: ${model}...`);

    try {
      const result = (await CheckOpenRouterModel(model)) as {
        latency: number;
        tps: number;
      };
      console.log(
        `Model ${model} is WORKING (${result.latency}ms, ${result.tps.toFixed(1)} t/s).`,
      );
      setModelStatuses((prev) => ({
        ...prev,
        [model]: { working: true, latency: result.latency, tps: result.tps },
      }));
    } catch (err) {
      console.error(`Model ${model} check FAILED:`, err);
      setModelStatuses((prev) => ({
        ...prev,
        [model]: { working: false },
      }));
    } finally {
      setCheckingModel(null);
    }
  };

  const checkGroqModelStatus = async (model: string) => {
    setCheckingModel(model);
    setModelStatuses((prev) => ({ ...prev, [model]: { checking: true } }));
    try {
      const result = (await CheckGroqModel(model)) as {
        latency: number;
        tps: number;
      };
      setModelStatuses((prev) => ({
        ...prev,
        [model]: { working: true, latency: result.latency, tps: result.tps },
      }));
    } catch (err) {
      setModelStatuses((prev) => ({ ...prev, [model]: { working: false } }));
    } finally {
      setCheckingModel(null);
    }
  };

  const checkCerebrasModelStatus = async (model: string) => {
    setCheckingModel(model);
    setModelStatuses((prev) => ({ ...prev, [model]: { checking: true } }));
    try {
      const result = (await CheckCerebrasModel(model)) as {
        latency: number;
        tps: number;
      };
      setModelStatuses((prev) => ({
        ...prev,
        [model]: { working: true, latency: result.latency, tps: result.tps },
      }));
    } catch (err) {
      setModelStatuses((prev) => ({ ...prev, [model]: { working: false } }));
    } finally {
      setCheckingModel(null);
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
      await loadGeminiModels();
    } catch (err) {
      console.error("Failed to save API key:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleSaveOpenRouterApiKey = async () => {
    if (!openRouterApiKey.trim()) return;
    setSaving("openRouterApiKey");
    try {
      await SetOpenRouterAPIKey(openRouterApiKey);
      setConfig((prev) =>
        prev ? { ...prev, openrouter_api_key_set: true } : null,
      );
      setOpenRouterApiKey("");
      showSuccess("openRouterApiKey");
      await loadOpenRouterModels();
    } catch (err) {
      console.error("Failed to save OpenRouter API key:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleSaveGroqApiKey = async () => {
    if (!groqApiKey.trim()) return;
    setSaving("groqApiKey");
    try {
      await SetGroqAPIKey(groqApiKey);
      setConfig((prev) => (prev ? { ...prev, groq_api_key_set: true } : null));
      setGroqApiKey("");
      showSuccess("groqApiKey");
      await loadGroqModels();
    } catch (err) {
      console.error("Failed to save Groq API key:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleSaveCerebrasApiKey = async () => {
    if (!cerebrasApiKey.trim()) return;
    setSaving("cerebrasApiKey");
    try {
      await SetCerebrasAPIKey(cerebrasApiKey);
      setConfig((prev) =>
        prev ? { ...prev, cerebras_api_key_set: true } : null,
      );
      setCerebrasApiKey("");
      showSuccess("cerebrasApiKey");
      await loadCerebrasModels();
    } catch (err) {
      console.error("Failed to save Cerebras API key:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleSaveLocalURL = async () => {
    setSaving("localURL");
    try {
      await SetLocalURL(localURL);
      setConfig((prev) => (prev ? { ...prev, local_url: localURL } : null));
      showSuccess("localURL");
    } catch (err) {
      console.error("Failed to save local URL:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleSaveLocalModel = async () => {
    setSaving("localModel");
    try {
      await SetLocalModel(localModel);
      setConfig((prev) => (prev ? { ...prev, local_model: localModel } : null));
      showSuccess("localModel");
    } catch (err) {
      console.error("Failed to save local model:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleCheckLocalModel = async () => {
    setCheckingModel(localModel);
    setLocalCheckResult(null);
    setLocalCheckError(null);
    try {
      const result = (await CheckLocalModel(localModel)) as {
        latency: number;
        tps: number;
      };
      setLocalCheckResult(result);
    } catch (err) {
      setLocalCheckError(String(err));
    } finally {
      setCheckingModel(null);
    }
  };

  const handleLLMProviderChange = async (provider: string) => {
    setSaving("llmProvider");
    try {
      await SetLLMProvider(provider);
      setConfig((prev) => (prev ? { ...prev, llm_provider: provider } : null));
      showSuccess("llmProvider");
    } catch (err) {
      console.error("Failed to save LLM provider:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleOpenRouterModelSelect = async (modelName: string) => {
    setSaving("openrouter_model");
    try {
      await SetOpenRouterModel(modelName);
      setConfig((prev) =>
        prev ? { ...prev, openrouter_model: modelName } : null,
      );
      showSuccess("openrouter_model");
    } catch (err) {
      console.error("Failed to save OpenRouter model:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleGroqModelSelect = async (modelName: string) => {
    setSaving("groq_model");
    try {
      await SetGroqModel(modelName);
      setConfig((prev) => (prev ? { ...prev, groq_model: modelName } : null));
      showSuccess("groq_model");
    } catch (err) {
      console.error("Failed to save Groq model:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleCerebrasModelSelect = async (modelName: string) => {
    setSaving("cerebras_model");
    try {
      await SetCerebrasModel(modelName);
      setConfig((prev) =>
        prev ? { ...prev, cerebras_model: modelName } : null,
      );
      showSuccess("cerebras_model");
    } catch (err) {
      console.error("Failed to save Cerebras model:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleRefinementModeChange = async (value: string) => {
    setSaving("refinementMode");
    try {
      await SetRefinementMode(value);
      setConfig((prev) => (prev ? { ...prev, refinement_mode: value } : null));
      showSuccess("refinementMode");
    } catch (err) {
      console.error("Failed to save refinement mode:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleMuteSystemAudioChange = async (value: boolean) => {
    setSaving("muteSystemAudio");
    try {
      await SetMuteSystemAudio(value);
      setConfig((prev) => (prev ? { ...prev, mute_system_audio: value } : null));
      showSuccess("muteSystemAudio");
    } catch (err) {
      console.error("Failed to save mute system audio setting:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleHandsFreeChange = async (value: string) => {
    setSaving("handsFree");
    try {
      await SetHandsFreeHotkey(value);
      setConfig((prev) =>
        prev ? { ...prev, hands_free_hotkey: value } : null,
      );
      showSuccess("handsFree");
    } catch (err) {
      console.error("Failed to save hands-free hotkey:", err);
    } finally {
      setSaving(null);
    }
  };

  const handlePushToTalkChange = async (value: string) => {
    setSaving("ptt");
    try {
      await SetPushToTalkHotkey(value);
      setConfig((prev) =>
        prev ? { ...prev, push_to_talk_hotkey: value } : null,
      );
      showSuccess("ptt");
    } catch (err) {
      console.error("Failed to save push-to-talk hotkey:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleModelSelect = async (modelName: string) => {
    setSaving("model");
    try {
      await SetWhisperModel(modelName);
      setConfig((prev) =>
        prev ? { ...prev, whisper_model: modelName } : null,
      );
      showSuccess("model");
    } catch (err) {
      console.error("Failed to save model:", err);
    } finally {
      setSaving(null);
    }
  };

  const handleGeminiModelSelect = async (modelName: string) => {
    setSaving("gemini_model");
    try {
      await SetGeminiModel(modelName);
      setConfig((prev) => (prev ? { ...prev, gemini_model: modelName } : null));
      showSuccess("gemini_model");
    } catch (err) {
      console.error("Failed to save Gemini model:", err);
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

  const [hotkeyModalOpen, setHotkeyModalOpen] = useState(false);
  const [activeHotkeyField, setActiveHotkeyField] = useState<
    "ptt" | "handsFree" | null
  >(null);
  const [activeHotkeyValue, setActiveHotkeyValue] = useState("");

  const openHotkeyModal = (
    field: "ptt" | "handsFree",
    currentValue: string,
  ) => {
    setActiveHotkeyField(field);
    setActiveHotkeyValue(currentValue);
    setHotkeyModalOpen(true);
  };

  const handleHotkeySave = async (newHotkey: string) => {
    if (activeHotkeyField === "ptt") {
      await handlePushToTalkChange(newHotkey);
    } else if (activeHotkeyField === "handsFree") {
      await handleHandsFreeChange(newHotkey);
    }
    setHotkeyModalOpen(false);
  };

  const formatHotkey = (hotkey: string) => {
    // Optional: Prettify the string for display (e.g., cmd -> ⌘)
    // For now, just capitalization is a good start
    return hotkey
      .split("+")
      .map((p) =>
        p === "cmd"
          ? "⌘"
          : p === "shift"
            ? "⇧"
            : p === "opt" || p === "alt"
              ? "⌥"
              : p.toUpperCase(),
      )
      .join(" + ");
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

  const formatSize = (bytes: number) => {
    if (bytes >= 1024 * 1024 * 1024) {
      return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
    }
    return `${Math.round(bytes / (1024 * 1024))} MB`;
  };

  if (!config) {
    return (
      <div className="flex items-center justify-center h-full">
        <p className="text-sm text-tertiary">Loading settings…</p>
      </div>
    );
  }

  return (
    <div className="settings-page animate-fade-in">
      <header className="shrink-0 px-6 pt-6 pb-4 border-b border-border">
        <h2 className="text-xl font-semibold text-text">Settings</h2>
        <p className="text-sm text-secondary mt-0.5">
          Configure transcription, models, and shortcuts.
        </p>
      </header>

      <div className="settings-scroll px-6 py-5">
        <div className="max-w-xl mx-auto space-y-4 pb-8">
        {!whisperReady && (
          <div className="alert alert-warning">
            Whisper CLI not found. Install with{" "}
            <code>brew install whisper-cpp</code>
          </div>
        )}

        <AppearanceSettings />

        <LLMProviderSettings
          config={config}
          saving={saving}
          success={success}
          apiKey={apiKey}
          setApiKey={setApiKey}
          openRouterApiKey={openRouterApiKey}
          setOpenRouterApiKey={setOpenRouterApiKey}
          groqApiKey={groqApiKey}
          setGroqApiKey={setGroqApiKey}
          cerebrasApiKey={cerebrasApiKey}
          setCerebrasApiKey={setCerebrasApiKey}
          localURL={localURL}
          setLocalURL={setLocalURL}
          localModel={localModel}
          setLocalModel={setLocalModel}
          localCheckResult={localCheckResult}
          localCheckError={localCheckError}
          checkingModel={checkingModel}
          handleLLMProviderChange={handleLLMProviderChange}
          handleSaveLocalURL={handleSaveLocalURL}
          handleSaveLocalModel={handleSaveLocalModel}
          handleCheckLocalModel={handleCheckLocalModel}
          handleSaveApiKey={handleSaveApiKey}
          handleSaveOpenRouterApiKey={handleSaveOpenRouterApiKey}
          handleSaveGroqApiKey={handleSaveGroqApiKey}
          handleSaveCerebrasApiKey={handleSaveCerebrasApiKey}
        />

        {/* Model Dropdown List Selection */}
        <ModelSelectionSettings
          config={config}
          saving={saving}
          geminiModels={geminiModels}
          geminiModelsLoading={geminiModelsLoading}
          geminiModelsError={geminiModelsError}
          handleGeminiModelSelect={handleGeminiModelSelect}
          checkGeminiModelStatus={checkGeminiModelStatus}
          openRouterModels={openRouterModels}
          openRouterModelsLoading={openRouterModelsLoading}
          handleOpenRouterModelSelect={handleOpenRouterModelSelect}
          checkOpenRouterModelStatus={checkOpenRouterModelStatus}
          groqModels={groqModels}
          groqModelsLoading={groqModelsLoading}
          handleGroqModelSelect={handleGroqModelSelect}
          checkGroqModelStatus={checkGroqModelStatus}
          cerebrasModels={cerebrasModels}
          cerebrasModelsLoading={cerebrasModelsLoading}
          handleCerebrasModelSelect={handleCerebrasModelSelect}
          checkCerebrasModelStatus={checkCerebrasModelStatus}
          checkingModel={checkingModel}
          modelStatuses={modelStatuses}
          loadGeminiModels={loadGeminiModels}
        />

        {/* Pipeline & Audio Settings */}
        <PipelineSettings
          config={config}
          saving={saving}
          success={success}
          handleRefinementModeChange={handleRefinementModeChange}
          handleMuteSystemAudioChange={handleMuteSystemAudioChange}
        />

        <AppRulesSettings />

        {/* Global Key Hotkeys */}
        <HotkeySettings
          config={config}
          saving={saving}
          success={success}
          openHotkeyModal={openHotkeyModal}
          formatHotkey={formatHotkey}
        />

        <SettingsSection
          title="Speech recognition models"
          description="Download and manage Whisper models. Larger models are more accurate but slower."
        >
          <div className="space-y-2">
            {modelsLoading ? (
              <p className="text-sm text-tertiary text-center py-4">
                Loading models...
              </p>
            ) : modelsError ? (
              <div className="p-3 rounded-md border border-[var(--danger)]/30 bg-[var(--danger)]/10">
                <p className="text-sm text-[var(--danger)]">
                  Failed to load models: {modelsError}
                </p>
                <button
                  type="button"
                  onClick={loadModels}
                  className="text-xs text-primary mt-2 hover:underline"
                >
                  Retry
                </button>
              </div>
            ) : models.length === 0 ? (
              <p className="text-sm text-tertiary text-center py-4">
                No models available
              </p>
            ) : (
              models.map((model) => (
                <div
                  key={model.name}
                  className={`flex items-center justify-between gap-3 p-3 rounded-md border transition-colors ${
                    config.whisper_model === model.name
                      ? "border-primary bg-accent-soft"
                      : "border-border bg-background hover:bg-surface-hover"
                  }`}
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <input
                      type="radio"
                      name="active_model"
                      checked={config.whisper_model === model.name}
                      onChange={() =>
                        model.downloaded && handleModelSelect(model.name)
                      }
                      disabled={!model.downloaded || saving === "model"}
                      className="w-4 h-4 shrink-0 accent-[var(--primary)]"
                    />
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-text capitalize truncate">
                        {model.name}
                        {config.whisper_model === model.name && (
                          <span className="ml-1.5 text-xs text-primary font-normal">
                            Active
                          </span>
                        )}
                      </p>
                      <p className="text-xs text-tertiary truncate">
                        {model.description} · {formatSize(model.size)}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    {model.downloaded ? (
                      <>
                        <span className="text-xs text-[var(--success)] hidden sm:inline">
                          Ready
                        </span>
                        {config.whisper_model !== model.name && (
                          <button
                            type="button"
                            onClick={() => startDeleteModel(model.name)}
                            className="btn btn-ghost !p-1.5 text-[var(--danger)]"
                            title="Delete model"
                            aria-label={`Delete ${model.name}`}
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
                        <div className="progress-bar w-16">
                          <div style={{ width: `${downloadProgress}%` }} />
                        </div>
                        <span className="text-xs text-tertiary w-8 tabular-nums">
                          {downloadProgress}%
                        </span>
                        <button
                          type="button"
                          onClick={handleCancelDownload}
                          className="btn btn-ghost !p-1.5"
                          title="Cancel download"
                          aria-label="Cancel download"
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
                        type="button"
                        onClick={() => handleDownloadModel(model.name)}
                        title={`Download ${model.name}`}
                        className="btn btn-primary !py-1.5 !px-3 !text-xs"
                      >
                        Download
                      </button>
                    )}
                  </div>
                </div>
              ))
            )}
          </div>
        </SettingsSection>
        </div>
      </div>

      {modelToDelete && (
        <div className="modal-overlay">
          <div className="modal-panel" role="dialog" aria-modal="true">
            <h3 className="text-lg font-semibold text-text mb-2">
              Delete model?
            </h3>
            <p className="text-sm text-secondary mb-6">
              Delete <span className="font-medium text-text">{modelToDelete}</span>?
              You will need to download it again.
            </p>
            <div className="flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setModelToDelete(null)}
                className="btn btn-secondary"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={confirmDeleteModel}
                className="btn btn-danger"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      <HotkeyRecorderModal
        isOpen={hotkeyModalOpen}
        onClose={() => setHotkeyModalOpen(false)}
        onSave={handleHotkeySave}
        initialValue={activeHotkeyValue}
      />
    </div>
  );
}
