import { useState, useEffect, useRef } from "react";
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

  const [isProviderDropdownOpen, setIsProviderDropdownOpen] = useState(false);
  const [isGeminiDropdownOpen, setIsGeminiDropdownOpen] = useState(false);
  const [isOpenRouterDropdownOpen, setIsOpenRouterDropdownOpen] =
    useState(false);
  const [isGroqDropdownOpen, setIsGroqDropdownOpen] = useState(false);
  const [isCerebrasDropdownOpen, setIsCerebrasDropdownOpen] = useState(false);

  const providerDropdownRef = useRef<HTMLDivElement>(null);
  const geminiDropdownRef = useRef<HTMLDivElement>(null);
  const openRouterDropdownRef = useRef<HTMLDivElement>(null);
  const groqDropdownRef = useRef<HTMLDivElement>(null);
  const cerebrasDropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        providerDropdownRef.current &&
        !providerDropdownRef.current.contains(event.target as Node)
      ) {
        setIsProviderDropdownOpen(false);
      }
      if (
        geminiDropdownRef.current &&
        !geminiDropdownRef.current.contains(event.target as Node)
      ) {
        setIsGeminiDropdownOpen(false);
      }
      if (
        openRouterDropdownRef.current &&
        !openRouterDropdownRef.current.contains(event.target as Node)
      ) {
        setIsOpenRouterDropdownOpen(false);
      }
      if (
        groqDropdownRef.current &&
        !groqDropdownRef.current.contains(event.target as Node)
      ) {
        setIsGroqDropdownOpen(false);
      }
      if (
        cerebrasDropdownRef.current &&
        !cerebrasDropdownRef.current.contains(event.target as Node)
      ) {
        setIsCerebrasDropdownOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, []);

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
      <div className="flex items-center justify-center h-screen">
        <p className="text-tertiary font-bold uppercase tracking-tighter">
          Loading settings...
        </p>
      </div>
    );
  }

  return (
    <div className="max-w-2xl mx-auto p-8 overflow-y-auto h-screen animate-fade-in">
      <h2 className="font-black text-3xl uppercase tracking-tighter text-primary mb-8">
        Settings
      </h2>

      <div className="space-y-8">
        {/* Whisper CLI Status */}
        {!whisperReady && (
          <section className="p-4 bg-amber-500/10 border-4 border-amber-500 rounded-[2rem]">
            <p className="text-sm text-amber-500 font-bold">
              ⚠️ Whisper CLI not found. Please install via:{" "}
              <code className="bg-secondary px-2 py-0.5 rounded font-mono text-xs border-2 border-border">
                brew install whisper-cpp
              </code>
            </p>
          </section>
        )}

        {/* LLM Provider Selection & Credentials */}
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
          isProviderDropdownOpen={isProviderDropdownOpen}
          setIsProviderDropdownOpen={setIsProviderDropdownOpen}
          providerDropdownRef={providerDropdownRef}
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
          isGeminiDropdownOpen={isGeminiDropdownOpen}
          setIsGeminiDropdownOpen={setIsGeminiDropdownOpen}
          geminiDropdownRef={geminiDropdownRef}
          handleGeminiModelSelect={handleGeminiModelSelect}
          checkGeminiModelStatus={checkGeminiModelStatus}
          openRouterModels={openRouterModels}
          openRouterModelsLoading={openRouterModelsLoading}
          isOpenRouterDropdownOpen={isOpenRouterDropdownOpen}
          setIsOpenRouterDropdownOpen={setIsOpenRouterDropdownOpen}
          openRouterDropdownRef={openRouterDropdownRef}
          handleOpenRouterModelSelect={handleOpenRouterModelSelect}
          checkOpenRouterModelStatus={checkOpenRouterModelStatus}
          groqModels={groqModels}
          groqModelsLoading={groqModelsLoading}
          isGroqDropdownOpen={isGroqDropdownOpen}
          setIsGroqDropdownOpen={setIsGroqDropdownOpen}
          groqDropdownRef={groqDropdownRef}
          handleGroqModelSelect={handleGroqModelSelect}
          checkGroqModelStatus={checkGroqModelStatus}
          cerebrasModels={cerebrasModels}
          cerebrasModelsLoading={cerebrasModelsLoading}
          isCerebrasDropdownOpen={isCerebrasDropdownOpen}
          setIsCerebrasDropdownOpen={setIsCerebrasDropdownOpen}
          cerebrasDropdownRef={cerebrasDropdownRef}
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

        <section className="card p-6 brutal-card">
          <h3 className="text-lg font-medium text-text mb-4">
            Speech Recognition Models
          </h3>
          <p className="text-sm text-tertiary mb-4">
            Download and manage Whisper models. Larger models are more accurate
            but slower.
          </p>
          <div className="space-y-3">
            {modelsLoading ? (
              <p className="text-sm text-tertiary text-center py-4">
                Loading models...
              </p>
            ) : modelsError ? (
              <div className="p-3 bg-red-500/10 border border-red-500 rounded-xl">
                <p className="text-sm text-red-500">
                  Failed to load models: {modelsError}
                </p>
                <button
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
                  className={`flex items-center justify-between p-4 rounded-xl border transition-colors ${
                    config.whisper_model === model.name
                      ? "bg-primary/10 border-primary"
                      : "border-border hover:bg-secondary/50"
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
                      className="w-4 h-4 text-primary focus:border-primary"
                    />
                    <div>
                      <p className="text-text capitalize font-medium">
                        {model.name}
                        {config.whisper_model === model.name && (
                          <span className="ml-2 text-xs text-primary">
                            (Active)
                          </span>
                        )}
                      </p>
                      <p className="text-sm text-tertiary">
                        {model.description} • {formatSize(model.size)}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    {model.downloaded ? (
                      <>
                        <span className="text-xs text-green-500">
                          ✓ Downloaded
                        </span>
                        {config.whisper_model !== model.name && (
                          <button
                            onClick={() => {
                              startDeleteModel(model.name);
                            }}
                            className="p-1.5 text-tertiary hover:text-red-500 hover:bg-red-500/10 rounded transition-colors"
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
                        <div className="w-16 h-2 bg-tertiary rounded-full overflow-hidden">
                          <div
                            className="h-full bg-primary transition-all duration-300"
                            style={{ width: `${downloadProgress}%` }}
                          />
                        </div>
                        <span className="text-xs text-secondary w-8">
                          {downloadProgress}%
                        </span>
                        <button
                          onClick={handleCancelDownload}
                          className="p-1 text-tertiary hover:text-red-500 hover:bg-red-500/10 rounded transition-colors"
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
                        title={`Download ${model.name} model (${Math.round(model.size / 1024 / 1024)}MB)`}
                        className="px-3 py-1.5 text-xs bg-primary hover:bg-primary text-white rounded-xl transition-colors"
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
      </div>

      {/* Delete Confirmation Modal */}
      {modelToDelete && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className="bg-background border border-border rounded-[2rem] p-6 w-full max-w-sm shadow-xl transform transition-all scale-100 opacity-100">
            <h3 className="text-lg font-medium text-text mb-2">
              Delete Model?
            </h3>
            <p className="text-secondary text-sm mb-6">
              Are you sure you want to delete the{" "}
              <span className="text-text font-semibold">{modelToDelete}</span>{" "}
              model? You will need to download it again to use it.
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setModelToDelete(null)}
                className="px-4 py-2 text-sm text-secondary hover:text-text transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={confirmDeleteModel}
                className="px-4 py-2 text-sm bg-red-600 hover:bg-red-500 text-white rounded-xl transition-colors font-medium"
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
