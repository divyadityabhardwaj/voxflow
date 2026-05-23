import { type ReactNode } from "react";
import SettingsSection from "../ui/SettingsSection";

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
  handleLLMProviderChange: (provider: string) => Promise<void>;
  handleSaveLocalURL: () => Promise<void>;
  handleSaveLocalModel: () => Promise<void>;
  handleCheckLocalModel: () => Promise<void>;
  handleSaveApiKey: () => Promise<void>;
  handleSaveOpenRouterApiKey: () => Promise<void>;
  handleSaveGroqApiKey: () => Promise<void>;
  handleSaveCerebrasApiKey: () => Promise<void>;
}

const PROVIDERS = [
  { id: "gemini", label: "Gemini", sub: "Google AI" },
  { id: "openrouter", label: "OpenRouter", sub: "Multi-model API" },
  { id: "groq", label: "Groq", sub: "Fast inference" },
  { id: "cerebras", label: "Cerebras", sub: "Wafer-scale inference" },
  { id: "local", label: "Local", sub: "Ollama, LM Studio, etc." },
] as const;

function ApiKeyBlock({
  title,
  hint,
  href,
  linkLabel,
  placeholder,
  value,
  onChange,
  isSet,
  onSave,
  saving,
  success,
  saveKey,
}: {
  title: string;
  hint: ReactNode;
  href: string;
  linkLabel: string;
  placeholder: string;
  value: string;
  onChange: (v: string) => void;
  isSet: boolean;
  onSave: () => void;
  saving: string | null;
  success: string | null;
  saveKey: string;
}) {
  return (
    <div className="pt-5 mt-5 border-t border-border">
      <h4 className="text-sm font-medium text-text mb-1">{title}</h4>
      <p className="text-sm text-secondary mb-3">
        {hint}{" "}
        <a
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          className="text-primary hover:underline"
        >
          {linkLabel}
        </a>
      </p>
      <div className="flex gap-2">
        <div className="relative flex-1 min-w-0">
          <input
            type="password"
            placeholder={isSet ? "••••••••••••••••" : placeholder}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            className="input"
          />
          {isSet && (
            <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-[var(--success)]">
              Saved
            </span>
          )}
        </div>
        <button
          type="button"
          onClick={onSave}
          disabled={!value.trim() || saving === saveKey}
          className="btn btn-primary shrink-0"
        >
          {saving === saveKey
            ? "Saving…"
            : success === saveKey
              ? "Saved"
              : "Save"}
        </button>
      </div>
    </div>
  );
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
  handleLLMProviderChange,
  handleSaveLocalURL,
  handleSaveLocalModel,
  handleCheckLocalModel,
  handleSaveApiKey,
  handleSaveOpenRouterApiKey,
  handleSaveGroqApiKey,
  handleSaveCerebrasApiKey,
}: LLMProviderSettingsProps) {
  const provider = config.llm_provider || "gemini";

  return (
    <SettingsSection
      title="LLM provider"
      description="Choose which service refines your transcriptions."
    >
      <div>
        <label className="label" htmlFor="llm-provider">
          Provider
        </label>
        <select
          id="llm-provider"
          className="select"
          value={provider}
          disabled={saving === "llmProvider"}
          onChange={(e) => handleLLMProviderChange(e.target.value)}
        >
          {PROVIDERS.map((p) => (
            <option key={p.id} value={p.id}>
              {p.label} — {p.sub}
            </option>
          ))}
        </select>
      </div>

      {provider === "local" && (
        <div className="pt-5 mt-5 border-t border-border space-y-4">
          <h4 className="text-sm font-medium text-text">Local server</h4>
          <p className="text-sm text-secondary -mt-2">
            Point VoxFlow at an OpenAI-compatible server (Ollama, LM Studio,
            llama.cpp).
          </p>

          <div>
            <label className="label" htmlFor="local-url">
              Server URL
            </label>
            <div className="flex gap-2">
              <input
                id="local-url"
                type="text"
                value={localURL}
                onChange={(e) => setLocalURL(e.target.value)}
                placeholder="http://localhost:11434"
                className="input flex-1 min-w-0"
              />
              <button
                type="button"
                onClick={handleSaveLocalURL}
                disabled={saving === "localURL"}
                className="btn btn-secondary shrink-0"
              >
                {saving === "localURL"
                  ? "Saving…"
                  : success === "localURL"
                    ? "Saved"
                    : "Save"}
              </button>
            </div>
            <p className="hint">
              Ollama: localhost:11434 · LM Studio: localhost:1234
            </p>
          </div>

          <div>
            <label className="label" htmlFor="local-model">
              Model name
            </label>
            <div className="flex gap-2">
              <input
                id="local-model"
                type="text"
                value={localModel}
                onChange={(e) => setLocalModel(e.target.value)}
                placeholder="qwen3:8b"
                className="input flex-1 min-w-0"
              />
              <button
                type="button"
                onClick={handleSaveLocalModel}
                disabled={saving === "localModel"}
                className="btn btn-secondary shrink-0"
              >
                {saving === "localModel"
                  ? "Saving…"
                  : success === "localModel"
                    ? "Saved"
                    : "Save"}
              </button>
            </div>
          </div>

          <div className="flex items-center gap-3 flex-wrap">
            <button
              type="button"
              onClick={handleCheckLocalModel}
              disabled={!localModel || !!checkingModel}
              className="btn btn-secondary"
            >
              {checkingModel === localModel ? "Testing…" : "Test connection"}
            </button>
            {localCheckResult && (
              <span className="text-xs text-[var(--success)]">
                {localCheckResult.latency}ms ·{" "}
                {localCheckResult.tps.toFixed(1)} t/s
              </span>
            )}
            {localCheckError && (
              <span className="text-xs text-[var(--danger)]">
                {localCheckError}
              </span>
            )}
          </div>
        </div>
      )}

      {(provider === "gemini" || !config.llm_provider) && (
        <ApiKeyBlock
          title="Gemini API key"
          hint="Get a key from"
          href="https://makersuite.google.com/app/apikey"
          linkLabel="Google AI Studio"
          placeholder="Enter API key"
          value={apiKey}
          onChange={setApiKey}
          isSet={config.api_key_set}
          onSave={handleSaveApiKey}
          saving={saving}
          success={success}
          saveKey="apiKey"
        />
      )}

      {provider === "openrouter" && (
        <ApiKeyBlock
          title="OpenRouter API key"
          hint="Get a key from"
          href="https://openrouter.ai/settings"
          linkLabel="OpenRouter Settings"
          placeholder="Enter API key"
          value={openRouterApiKey}
          onChange={setOpenRouterApiKey}
          isSet={config.openrouter_api_key_set}
          onSave={handleSaveOpenRouterApiKey}
          saving={saving}
          success={success}
          saveKey="openRouterApiKey"
        />
      )}

      {provider === "groq" && (
        <ApiKeyBlock
          title="Groq API key"
          hint="Get a key from"
          href="https://console.groq.com/keys"
          linkLabel="Groq Console"
          placeholder="Enter API key"
          value={groqApiKey}
          onChange={setGroqApiKey}
          isSet={config.groq_api_key_set}
          onSave={handleSaveGroqApiKey}
          saving={saving}
          success={success}
          saveKey="groqApiKey"
        />
      )}

      {provider === "cerebras" && (
        <ApiKeyBlock
          title="Cerebras API key"
          hint="Get a key from"
          href="https://cloud.cerebras.ai"
          linkLabel="Cerebras Cloud"
          placeholder="Enter API key"
          value={cerebrasApiKey}
          onChange={setCerebrasApiKey}
          isSet={config.cerebras_api_key_set}
          onSave={handleSaveCerebrasApiKey}
          saving={saving}
          success={success}
          saveKey="cerebrasApiKey"
        />
      )}
    </SettingsSection>
  );
}
