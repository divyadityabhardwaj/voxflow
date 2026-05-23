import SettingsSection from "../ui/SettingsSection";

interface Config {
  llm_provider: string;
  api_key_set: boolean;
  openrouter_api_key_set: boolean;
  groq_api_key_set: boolean;
  cerebras_api_key_set: boolean;
  gemini_model: string;
  openrouter_model: string;
  groq_model: string;
  cerebras_model: string;
}

interface ModelStatus {
  latency?: number;
  tps?: number;
  working?: boolean;
  checking?: boolean;
}

interface ModelSelectionSettingsProps {
  config: Config;
  saving: string | null;
  geminiModels: string[];
  geminiModelsLoading: boolean;
  geminiModelsError: string | null;
  handleGeminiModelSelect: (modelName: string) => Promise<void>;
  checkGeminiModelStatus: (model: string) => Promise<void>;
  openRouterModels: string[];
  openRouterModelsLoading: boolean;
  handleOpenRouterModelSelect: (modelName: string) => Promise<void>;
  checkOpenRouterModelStatus: (model: string) => Promise<void>;
  groqModels: string[];
  groqModelsLoading: boolean;
  handleGroqModelSelect: (modelName: string) => Promise<void>;
  checkGroqModelStatus: (model: string) => Promise<void>;
  cerebrasModels: string[];
  cerebrasModelsLoading: boolean;
  handleCerebrasModelSelect: (modelName: string) => Promise<void>;
  checkCerebrasModelStatus: (model: string) => Promise<void>;
  checkingModel: string | null;
  modelStatuses: Record<string, ModelStatus>;
  loadGeminiModels: () => void;
}

function ModelStatusBadge({
  model,
  statuses,
  checking,
}: {
  model: string;
  statuses: Record<string, ModelStatus>;
  checking: string | null;
}) {
  if (checking === model) {
    return (
      <span className="text-xs text-tertiary animate-pulse-soft">Testing…</span>
    );
  }
  const s = statuses[model];
  if (s?.working) {
    return (
      <span className="text-xs text-[var(--success)]">
        {s.latency}ms
        {s.tps !== undefined ? ` · ${s.tps.toFixed(1)} t/s` : ""}
      </span>
    );
  }
  if (s?.working === false) {
    return <span className="text-xs text-[var(--danger)]">Failed</span>;
  }
  return null;
}

function ModelPicker({
  label,
  id,
  models,
  loading,
  error,
  value,
  savingKey,
  saving,
  onSelect,
  onTest,
  checkingModel,
  modelStatuses,
  onRetry,
}: {
  label: string;
  id: string;
  models: string[];
  loading: boolean;
  error?: string | null;
  value: string;
  savingKey: string;
  saving: string | null;
  onSelect: (m: string) => void;
  onTest: () => void;
  checkingModel: string | null;
  modelStatuses: Record<string, ModelStatus>;
  onRetry?: () => void;
}) {
  if (loading) {
    return <p className="text-sm text-tertiary">Loading models…</p>;
  }

  if (error) {
    return (
      <div className="text-sm text-[var(--danger)]">
        {error}
        {onRetry && (
          <button
            type="button"
            onClick={onRetry}
            className="ml-2 text-primary hover:underline"
          >
            Retry
          </button>
        )}
      </div>
    );
  }

  if (models.length === 0) {
    return <p className="text-sm text-tertiary">No models available.</p>;
  }

  return (
    <div>
      <label className="label" htmlFor={id}>
        {label}
      </label>
      <div className="flex gap-2">
        <select
          id={id}
          className="select flex-1 min-w-0"
          value={value || models[0]}
          disabled={saving === savingKey}
          onChange={(e) => onSelect(e.target.value)}
        >
          {models.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
        <button
          type="button"
          onClick={onTest}
          disabled={!value || !!checkingModel}
          className="btn btn-secondary shrink-0"
        >
          Test
        </button>
      </div>
      <div className="mt-2 min-h-[18px]">
        <ModelStatusBadge
          model={value}
          statuses={modelStatuses}
          checking={checkingModel}
        />
      </div>
    </div>
  );
}

export default function ModelSelectionSettings({
  config,
  saving,
  geminiModels,
  geminiModelsLoading,
  geminiModelsError,
  handleGeminiModelSelect,
  checkGeminiModelStatus,
  openRouterModels,
  openRouterModelsLoading,
  handleOpenRouterModelSelect,
  checkOpenRouterModelStatus,
  groqModels,
  groqModelsLoading,
  handleGroqModelSelect,
  checkGroqModelStatus,
  cerebrasModels,
  cerebrasModelsLoading,
  handleCerebrasModelSelect,
  checkCerebrasModelStatus,
  checkingModel,
  modelStatuses,
  loadGeminiModels,
}: ModelSelectionSettingsProps) {
  const provider = config.llm_provider || "gemini";

  if (provider === "local") {
    return null;
  }

  const showGemini =
    (provider === "gemini" || !config.llm_provider) && config.api_key_set;
  const showOpenRouter =
    provider === "openrouter" && config.openrouter_api_key_set;
  const showGroq = provider === "groq" && config.groq_api_key_set;
  const showCerebras = provider === "cerebras" && config.cerebras_api_key_set;

  if (!showGemini && !showOpenRouter && !showGroq && !showCerebras) {
    return null;
  }

  return (
    <SettingsSection
      title="Refinement model"
      description="Pick the model used to polish transcriptions."
    >
      <div className="space-y-5">
        {showGemini && (
          <ModelPicker
            label="Gemini model"
            id="gemini-model"
            models={geminiModels}
            loading={geminiModelsLoading}
            error={geminiModelsError}
            value={config.gemini_model}
            savingKey="gemini_model"
            saving={saving}
            onSelect={handleGeminiModelSelect}
            onTest={() => checkGeminiModelStatus(config.gemini_model)}
            checkingModel={checkingModel}
            modelStatuses={modelStatuses}
            onRetry={loadGeminiModels}
          />
        )}

        {showOpenRouter && (
          <ModelPicker
            label="OpenRouter model"
            id="openrouter-model"
            models={openRouterModels}
            loading={openRouterModelsLoading}
            value={config.openrouter_model}
            savingKey="openrouter_model"
            saving={saving}
            onSelect={handleOpenRouterModelSelect}
            onTest={() => checkOpenRouterModelStatus(config.openrouter_model)}
            checkingModel={checkingModel}
            modelStatuses={modelStatuses}
          />
        )}

        {showGroq && (
          <ModelPicker
            label="Groq model"
            id="groq-model"
            models={groqModels}
            loading={groqModelsLoading}
            value={config.groq_model}
            savingKey="groq_model"
            saving={saving}
            onSelect={handleGroqModelSelect}
            onTest={() => checkGroqModelStatus(config.groq_model)}
            checkingModel={checkingModel}
            modelStatuses={modelStatuses}
          />
        )}

        {showCerebras && (
          <ModelPicker
            label="Cerebras model"
            id="cerebras-model"
            models={cerebrasModels}
            loading={cerebrasModelsLoading}
            value={config.cerebras_model}
            savingKey="cerebras_model"
            saving={saving}
            onSelect={handleCerebrasModelSelect}
            onTest={() => checkCerebrasModelStatus(config.cerebras_model)}
            checkingModel={checkingModel}
            modelStatuses={modelStatuses}
          />
        )}
      </div>
    </SettingsSection>
  );
}
