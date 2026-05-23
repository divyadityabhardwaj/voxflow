import { RefObject } from "react";

interface Config {
  llm_provider: string;
  api_key_set: boolean;
  openrouter_api_key_set: boolean;
  groq_api_key_set: boolean;
  cerebras_api_key_set: boolean;
}

interface LLMProviderSettingsProps {
  config: Config;
  saving: string | null;
  success: string | null;
  apiKey: string;
  setApiKey: (val: string) => void;
  openRouterApiKey: string;
  setOpenRouterApiKey: (val: string) => void;
  groqApiKey: string;
  setGroqApiKey: (val: string) => void;
  cerebrasApiKey: string;
  setCerebrasApiKey: (val: string) => void;
  localURL: string;
  setLocalURL: (val: string) => void;
  localModel: string;
  setLocalModel: (val: string) => void;
  localCheckResult: { latency: number; tps: number } | null;
  localCheckError: string | null;
  checkingModel: string | null;
  isProviderDropdownOpen: boolean;
  setIsProviderDropdownOpen: (val: boolean) => void;
  providerDropdownRef: RefObject<HTMLDivElement>;
  handleLLMProviderChange: (provider: string) => Promise<void>;
  handleSaveLocalURL: () => Promise<void>;
  handleSaveLocalModel: () => Promise<void>;
  handleCheckLocalModel: () => Promise<void>;
  handleSaveApiKey: () => Promise<void>;
  handleSaveOpenRouterApiKey: () => Promise<void>;
  handleSaveGroqApiKey: () => Promise<void>;
  handleSaveCerebrasApiKey: () => Promise<void>;
}

export default function LLMProviderSettings({
  config,
  saving,
  success,
  apiKey,
  setApiKey,
  openRouterApiKey,
  setOpenRouterApiKey,
  groqApiKey,
  setGroqApiKey,
  cerebrasApiKey,
  setCerebrasApiKey,
  localURL,
  setLocalURL,
  localModel,
  setLocalModel,
  localCheckResult,
  localCheckError,
  checkingModel,
  isProviderDropdownOpen,
  setIsProviderDropdownOpen,
  providerDropdownRef,
  handleLLMProviderChange,
  handleSaveLocalURL,
  handleSaveLocalModel,
  handleCheckLocalModel,
  handleSaveApiKey,
  handleSaveOpenRouterApiKey,
  handleSaveGroqApiKey,
  handleSaveCerebrasApiKey,
}: LLMProviderSettingsProps) {
  return (
    <section className="card p-6 brutal-card">
      <h3 className="font-black text-xl uppercase tracking-tighter text-primary mb-4">
        LLM Provider
      </h3>
      <p className="text-sm text-tertiary mb-4 font-bold">
        Choose which LLM provider to use for text refinement.
      </p>

      {/* Provider Selector Dropdown */}
      <div className="relative mb-6" ref={providerDropdownRef}>
        <button
          onClick={() =>
            !saving && setIsProviderDropdownOpen(!isProviderDropdownOpen)
          }
          disabled={saving === "llmProvider"}
          className="w-full px-4 py-2.5 bg-secondary border-4 border-border rounded-[2rem]
                   text-text flex items-center justify-between
                   focus:outline-none focus:border-primary focus:shadow-[4px_4px_0px_var(--primary)]
                   disabled:opacity-50 text-left capitalize font-bold"
        >
          <span>{config.llm_provider || "gemini"}</span>
          <svg
            className={`w-4 h-4 text-tertiary transition-transform ${isProviderDropdownOpen ? "rotate-180" : ""}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M19 9l-7 7-7-7"
            />
          </svg>
        </button>

        {isProviderDropdownOpen && (
          <div className="absolute z-20 w-full mt-1 bg-secondary border-4 border-border rounded-[2rem] shadow-[8px_8px_0px_var(--primary)] overflow-hidden">
            {[
              {
                id: "gemini",
                label: "Gemini",
                sub: "Google's fast & capable model",
              },
              {
                id: "openrouter",
                label: "OpenRouter",
                sub: "Free open-source models",
              },
              {
                id: "groq",
                label: "Groq",
                sub: "Ultra-fast LPU inference",
              },
              {
                id: "cerebras",
                label: "Cerebras",
                sub: "Wafer-scale fast inference",
              },
              {
                id: "local",
                label: "Local",
                sub: "Run models locally on your device",
              },
            ].map((prov) => (
              <div
                key={prov.id}
                onClick={() => {
                  handleLLMProviderChange(prov.id);
                  setIsProviderDropdownOpen(false);
                }}
                className={`flex flex-col px-4 py-3 cursor-pointer transition-colors font-bold ${
                  (config.llm_provider || "gemini") === prov.id
                    ? "bg-primary/10 text-primary"
                    : "text-tertiary hover:bg-secondary"
                }`}
              >
                <span className="font-medium capitalize">{prov.label}</span>
                <span className="text-xs text-tertiary mt-1 font-bold">
                  {prov.sub}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      {config.llm_provider === "local" && (
        <div className="pt-6 border-t-4 border-border">
          <h4 className="font-black text-lg uppercase tracking-tighter text-text mb-4">
            Local Server
          </h4>
          <p className="text-sm text-tertiary mb-4 font-bold">
            Point VoxFlow at any running OpenAI-compatible server (Ollama,
            LM Studio, llama.cpp, etc.). Start your server separately and
            enter its URL and model name below.
          </p>

          {/* Server URL */}
          <div className="mb-4">
            <label className="block text-xs font-black uppercase tracking-tighter text-tertiary mb-2">
              Server URL
            </label>
            <div className="flex gap-2">
              <input
                type="text"
                value={localURL}
                onChange={(e) => setLocalURL(e.target.value)}
                placeholder="http://localhost:11434"
                className="flex-1 px-4 py-3 bg-secondary border-4 border-border rounded-[2rem] text-text font-bold text-sm focus:outline-none focus:border-primary"
              />
              <button
                onClick={handleSaveLocalURL}
                disabled={saving === "localURL"}
                className="px-4 py-2 bg-primary hover:bg-primary/90 disabled:opacity-50 text-white rounded-[2rem] border-4 border-primary/80 font-black uppercase tracking-tighter text-xs transition-colors"
              >
                {saving === "localURL"
                  ? "Saving…"
                  : success === "localURL"
                    ? "Saved ✓"
                    : "Save"}
              </button>
            </div>
            <p className="text-xs text-tertiary mt-1 font-bold">
              Ollama default: http://localhost:11434 · LM Studio:
              http://localhost:1234
            </p>
          </div>

          {/* Model Name */}
          <div className="mb-4">
            <label className="block text-xs font-black uppercase tracking-tighter text-tertiary mb-2">
              Model Name
            </label>
            <div className="flex gap-2">
              <input
                type="text"
                value={localModel}
                onChange={(e) => setLocalModel(e.target.value)}
                placeholder="qwen3:8b"
                className="flex-1 px-4 py-3 bg-secondary border-4 border-border rounded-[2rem] text-text font-bold text-sm focus:outline-none focus:border-primary"
              />
              <button
                onClick={handleSaveLocalModel}
                disabled={saving === "localModel"}
                className="px-4 py-2 bg-primary hover:bg-primary/90 disabled:opacity-50 text-white rounded-[2rem] border-4 border-primary/80 font-black uppercase tracking-tighter text-xs transition-colors"
              >
                {saving === "localModel"
                  ? "Saving…"
                  : success === "localModel"
                    ? "Saved ✓"
                    : "Save"}
              </button>
            </div>
            <p className="text-xs text-tertiary mt-1 font-bold">
              Examples: qwen3:8b · llama3:8b · mistral · phi4 ·
              deepseek-r1:7b
            </p>
          </div>

          {/* Test Connection */}
          <div className="flex items-center gap-3">
            <button
              onClick={handleCheckLocalModel}
              disabled={!localModel || !!checkingModel}
              className="px-4 py-2 bg-secondary hover:bg-border disabled:opacity-40 disabled:cursor-not-allowed text-text rounded-[2rem] border-4 border-border font-black uppercase tracking-tighter text-xs transition-colors"
            >
              {checkingModel === localModel ? "Testing…" : "Test Connection"}
            </button>
            {localCheckResult && (
              <span className="text-xs text-emerald-500 font-black">
                ✓ {localCheckResult.latency}ms ·{" "}
                {localCheckResult.tps.toFixed(1)} t/s
              </span>
            )}
            {localCheckError && (
              <span className="text-xs text-red-500 font-black">
                ✗ {localCheckError}
              </span>
            )}
          </div>
        </div>
      )}

      {/* Gemini API Key Section */}
      {(config.llm_provider === "gemini" || !config.llm_provider) && (
        <div className="pt-6 border-t-4 border-border">
          <h4 className="font-black text-lg uppercase tracking-tighter text-text mb-4">
            Gemini API Key
          </h4>
          <p className="text-sm text-tertiary mb-4 font-bold">
            Get your API key from{" "}
            <a
              href="https://makersuite.google.com/app/apikey"
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary hover:underline font-bold"
            >
              Google AI Studio
            </a>
          </p>
          <div className="flex gap-3 mb-6">
            <div className="relative flex-1">
              <input
                type="password"
                placeholder={
                  config.api_key_set ? "••••••••••••••••" : "Enter your API key"
                }
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                className="w-full px-4 py-2.5 bg-secondary border-4 border-border rounded-[2rem]
                         text-text placeholder-tertiary
                         focus:outline-none focus:border-primary focus:shadow-[4px_4px_0px_var(--primary)] font-bold"
              />
              {config.api_key_set && (
                <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-emerald-500 font-bold">
                  ✓ Set
                </span>
              )}
            </div>
            <button
              onClick={handleSaveApiKey}
              disabled={!apiKey.trim() || saving === "apiKey"}
              className="px-5 py-2.5 bg-primary hover:bg-primary/90 disabled:opacity-50
                       text-white rounded-[2rem] border-4 border-primary/80 transition-colors font-bold uppercase tracking-tighter"
            >
              {saving === "apiKey"
                ? "Saving..."
                : success === "apiKey"
                  ? "Saved!"
                  : "Save"}
            </button>
          </div>
        </div>
      )}

      {/* OpenRouter Section */}
      {config.llm_provider === "openrouter" && (
        <div className="pt-6 border-t border-border">
          <h4 className="font-medium text-text mb-4">OpenRouter API Key</h4>
          <p className="text-sm text-tertiary mb-4">
            Get your API key from{" "}
            <a
              href="https://openrouter.ai/settings"
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary hover:underline"
            >
              OpenRouter Settings
            </a>
          </p>
          <div className="flex gap-3 mb-6">
            <div className="relative flex-1">
              <input
                type="password"
                placeholder={
                  config.openrouter_api_key_set
                    ? "••••••••••••••••"
                    : "Enter your OpenRouter API key"
                }
                value={openRouterApiKey}
                onChange={(e) => setOpenRouterApiKey(e.target.value)}
                className="w-full px-4 py-2.5 bg-secondary border border-border rounded-xl
                         text-text placeholder:text-tertiary
                         focus:outline-none focus:ring-2 focus:border-primary"
              />
              {config.openrouter_api_key_set && (
                <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-green-500">
                  ✓ Set
                </span>
              )}
            </div>
            <button
              onClick={handleSaveOpenRouterApiKey}
              disabled={
                !openRouterApiKey.trim() || saving === "openRouterApiKey"
              }
              className="px-5 py-2.5 bg-primary hover:bg-primary disabled:opacity-50
                       text-white rounded-xl transition-colors"
            >
              {saving === "openRouterApiKey"
                ? "Saving..."
                : success === "openRouterApiKey"
                  ? "Saved!"
                  : "Save"}
            </button>
          </div>
        </div>
      )}

      {/* Groq Section */}
      {config.llm_provider === "groq" && (
        <div className="pt-6 border-t border-border">
          <h4 className="font-medium text-text mb-4">Groq API Key</h4>
          <p className="text-sm text-tertiary mb-4">
            Get your API key from{" "}
            <a
              href="https://console.groq.com/keys"
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary hover:underline"
            >
              Groq Console
            </a>
          </p>
          <div className="flex gap-3 mb-6">
            <div className="relative flex-1">
              <input
                type="password"
                placeholder={
                  config.groq_api_key_set
                    ? "••••••••••••••••"
                    : "Enter your Groq API key"
                }
                value={groqApiKey}
                onChange={(e) => setGroqApiKey(e.target.value)}
                className="w-full px-4 py-2.5 bg-secondary border border-border rounded-xl
                         text-text placeholder:text-tertiary
                         focus:outline-none focus:ring-2 focus:border-primary"
              />
              {config.groq_api_key_set && (
                <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-green-500">
                  ✓ Set
                </span>
              )}
            </div>
            <button
              onClick={handleSaveGroqApiKey}
              disabled={!groqApiKey.trim() || saving === "groqApiKey"}
              className="px-5 py-2.5 bg-primary hover:bg-primary disabled:opacity-50
                       text-white rounded-xl transition-colors"
            >
              {saving === "groqApiKey"
                ? "Saving..."
                : success === "groqApiKey"
                  ? "Saved!"
                  : "Save"}
            </button>
          </div>
        </div>
      )}

      {/* Cerebras Section */}
      {config.llm_provider === "cerebras" && (
        <div className="pt-6 border-t border-border">
          <h4 className="font-medium text-text mb-4">Cerebras API Key</h4>
          <p className="text-sm text-tertiary mb-4">
            Get your API key from{" "}
            <a
              href="https://cloud.cerebras.ai"
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary hover:underline"
            >
              Cerebras Cloud
            </a>
          </p>
          <div className="flex gap-3 mb-6">
            <div className="relative flex-1">
              <input
                type="password"
                placeholder={
                  config.cerebras_api_key_set
                    ? "••••••••••••••••"
                    : "Enter your Cerebras API key"
                }
                value={cerebrasApiKey}
                onChange={(e) => setCerebrasApiKey(e.target.value)}
                className="w-full px-4 py-2.5 bg-secondary border border-border rounded-xl
                         text-text placeholder:text-tertiary
                         focus:outline-none focus:ring-2 focus:border-primary"
              />
              {config.cerebras_api_key_set && (
                <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-green-500">
                  ✓ Set
                </span>
              )}
            </div>
            <button
              onClick={handleSaveCerebrasApiKey}
              disabled={
                !cerebrasApiKey.trim() || saving === "cerebrasApiKey"
              }
              className="px-5 py-2.5 bg-primary hover:bg-primary disabled:opacity-50
                       text-white rounded-xl transition-colors"
            >
              {saving === "cerebrasApiKey"
                ? "Saving..."
                : success === "cerebrasApiKey"
                  ? "Saved!"
                  : "Save"}
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
