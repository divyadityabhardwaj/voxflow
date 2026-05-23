import { RefObject } from "react";

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
  isGeminiDropdownOpen: boolean;
  setIsGeminiDropdownOpen: (val: boolean) => void;
  geminiDropdownRef: RefObject<HTMLDivElement>;
  handleGeminiModelSelect: (modelName: string) => Promise<void>;
  checkGeminiModelStatus: (model: string) => Promise<void>;
  openRouterModels: string[];
  openRouterModelsLoading: boolean;
  isOpenRouterDropdownOpen: boolean;
  setIsOpenRouterDropdownOpen: (val: boolean) => void;
  openRouterDropdownRef: RefObject<HTMLDivElement>;
  handleOpenRouterModelSelect: (modelName: string) => Promise<void>;
  checkOpenRouterModelStatus: (model: string) => Promise<void>;
  groqModels: string[];
  groqModelsLoading: boolean;
  isGroqDropdownOpen: boolean;
  setIsGroqDropdownOpen: (val: boolean) => void;
  groqDropdownRef: RefObject<HTMLDivElement>;
  handleGroqModelSelect: (modelName: string) => Promise<void>;
  checkGroqModelStatus: (model: string) => Promise<void>;
  cerebrasModels: string[];
  cerebrasModelsLoading: boolean;
  isCerebrasDropdownOpen: boolean;
  setIsCerebrasDropdownOpen: (val: boolean) => void;
  cerebrasDropdownRef: RefObject<HTMLDivElement>;
  handleCerebrasModelSelect: (modelName: string) => Promise<void>;
  checkCerebrasModelStatus: (model: string) => Promise<void>;
  checkingModel: string | null;
  modelStatuses: Record<string, ModelStatus>;
  loadGeminiModels: () => void;
}

export default function ModelSelectionSettings({
  config,
  saving,
  geminiModels,
  geminiModelsLoading,
  geminiModelsError,
  isGeminiDropdownOpen,
  setIsGeminiDropdownOpen,
  geminiDropdownRef,
  handleGeminiModelSelect,
  checkGeminiModelStatus,
  openRouterModels,
  openRouterModelsLoading,
  isOpenRouterDropdownOpen,
  setIsOpenRouterDropdownOpen,
  openRouterDropdownRef,
  handleOpenRouterModelSelect,
  checkOpenRouterModelStatus,
  groqModels,
  groqModelsLoading,
  isGroqDropdownOpen,
  setIsGroqDropdownOpen,
  groqDropdownRef,
  handleGroqModelSelect,
  checkGroqModelStatus,
  cerebrasModels,
  cerebrasModelsLoading,
  isCerebrasDropdownOpen,
  setIsCerebrasDropdownOpen,
  cerebrasDropdownRef,
  handleCerebrasModelSelect,
  checkCerebrasModelStatus,
  checkingModel,
  modelStatuses,
  loadGeminiModels,
}: ModelSelectionSettingsProps) {
  // Return null if provider is local since we don't have custom model lists dropdowns
  if (config.llm_provider === "local") {
    return null;
  }

  return (
    <section className="card p-6 brutal-card">
      <h3 className="font-black text-xl uppercase tracking-tighter text-primary mb-4">
        Model Selection
      </h3>
      <p className="text-sm text-tertiary mb-6 font-bold">
        Select which specific model from your active provider to use for text refinement.
      </p>

      {/* Gemini Models Dropdown */}
      {(config.llm_provider === "gemini" || !config.llm_provider) && config.api_key_set && (
        <div>
          <div className="flex items-center justify-between mb-4">
            <h4 className="font-black text-lg uppercase tracking-tighter text-text">
              Gemini Model
            </h4>
          </div>

          {geminiModelsLoading ? (
            <p className="text-sm text-tertiary font-bold">Loading models...</p>
          ) : geminiModelsError ? (
            <div className="text-sm text-red-500 font-bold">
              {geminiModelsError}
              <button
                onClick={loadGeminiModels}
                className="ml-2 underline hover:text-red-300"
              >
                Retry
              </button>
            </div>
          ) : (
            <div className="space-y-2">
              <div className="relative" ref={geminiDropdownRef}>
                <button
                  onClick={() =>
                    !saving && setIsGeminiDropdownOpen(!isGeminiDropdownOpen)
                  }
                  disabled={saving === "gemini_model"}
                  className="w-full px-4 py-2.5 bg-secondary border-4 border-border rounded-[2rem]
                           text-text flex items-center justify-between
                           focus:outline-none focus:border-primary focus:shadow-[4px_4px_0px_var(--primary)]
                           disabled:opacity-50 text-left font-bold"
                >
                  <span>
                    {config.gemini_model}
                    {modelStatuses[config.gemini_model]?.working && (
                      <span className="text-emerald-500 ml-2 text-xs font-bold">
                        ✓ {modelStatuses[config.gemini_model]?.latency}ms
                        {" | "}
                        {modelStatuses[config.gemini_model]?.tps?.toFixed(1)} t/s
                      </span>
                    )}
                    {modelStatuses[config.gemini_model]?.working === false && (
                      <span className="text-red-500 ml-2 text-xs font-bold">
                        ✗ Error
                      </span>
                    )}
                  </span>
                  <svg
                    className={`w-4 h-4 text-tertiary transition-transform ${isGeminiDropdownOpen ? "rotate-180" : ""}`}
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

                {isGeminiDropdownOpen && (
                  <div className="absolute z-20 w-full mt-1 bg-secondary border border-border rounded-xl shadow-xl max-h-60 overflow-y-auto">
                    {geminiModels.map((model) => (
                      <div
                        key={model}
                        onClick={() => {
                          handleGeminiModelSelect(model);
                          setIsGeminiDropdownOpen(false);
                        }}
                        className={`flex items-center justify-between px-4 py-3 cursor-pointer transition-colors ${
                          config.gemini_model === model
                            ? "bg-primary/10 text-primary"
                            : "text-text hover:bg-secondary"
                        }`}
                      >
                        <span className="font-medium">{model}</span>
                        <span className="text-xs flex items-center gap-2">
                          {checkingModel === model && (
                            <span className="text-tertiary animate-pulse font-bold">
                              Checking...
                            </span>
                          )}
                          {!checkingModel && modelStatuses[model]?.working && (
                            <span className="text-green-500 font-medium">
                              ✓ {modelStatuses[model]?.latency}ms
                              {modelStatuses[model]?.tps !== undefined &&
                                ` | ${modelStatuses[model]?.tps?.toFixed(1)} t/s`}
                            </span>
                          )}
                          {!checkingModel && modelStatuses[model]?.working === false && (
                            <span className="text-red-500 font-medium">
                              ✗ Error
                            </span>
                          )}
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              checkGeminiModelStatus(model);
                            }}
                            disabled={!!checkingModel}
                            className="text-primary hover:text-primary disabled:opacity-50 font-bold"
                          >
                            Check
                          </button>
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
              <p className="text-xs text-tertiary mt-2 font-bold">
                Select the Gemini model to use for transcription refinement.
              </p>
            </div>
          )}
        </div>
      )}

      {/* OpenRouter Models Dropdown */}
      {config.llm_provider === "openrouter" && config.openrouter_api_key_set && (
        <div>
          <div className="flex items-center justify-between mb-4">
            <h4 className="font-black text-lg uppercase tracking-tighter text-text">
              OpenRouter Model
            </h4>
          </div>

          {openRouterModelsLoading ? (
            <p className="text-sm text-tertiary font-bold">Loading models...</p>
          ) : (
            <div className="space-y-2">
              <div className="relative" ref={openRouterDropdownRef}>
                <button
                  onClick={() =>
                    !saving && setIsOpenRouterDropdownOpen(!isOpenRouterDropdownOpen)
                  }
                  disabled={saving === "openrouter_model"}
                  className="w-full px-4 py-2.5 bg-secondary border border-border rounded-xl
                             text-text flex items-center justify-between
                             focus:outline-none focus:ring-2 focus:border-primary
                             disabled:opacity-50 text-left font-bold"
                >
                  <span>
                    {config.openrouter_model}
                    {modelStatuses[config.openrouter_model]?.working && (
                      <span className="text-green-500 ml-2 text-xs font-bold">
                        ✓ {modelStatuses[config.openrouter_model]?.latency}ms
                        {" | "}
                        {modelStatuses[config.openrouter_model]?.tps?.toFixed(1)} t/s
                      </span>
                    )}
                    {modelStatuses[config.openrouter_model]?.working === false && (
                      <span className="text-red-500 ml-2 text-xs font-bold">
                        ✗ Error
                      </span>
                    )}
                  </span>
                  <svg
                    className={`w-4 h-4 text-tertiary transition-transform ${isOpenRouterDropdownOpen ? "rotate-180" : ""}`}
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

                {isOpenRouterDropdownOpen && (
                  <div className="absolute z-20 w-full mt-1 bg-secondary border border-border rounded-xl shadow-xl max-h-60 overflow-y-auto">
                    {openRouterModels.map((model) => (
                      <div
                        key={model}
                        onClick={() => {
                          handleOpenRouterModelSelect(model);
                          setIsOpenRouterDropdownOpen(false);
                        }}
                        className={`flex items-center justify-between px-4 py-3 cursor-pointer transition-colors ${
                          config.openrouter_model === model
                            ? "bg-primary/10 text-primary"
                            : "text-text hover:bg-secondary"
                        }`}
                      >
                        <span className="font-medium text-sm">
                          {model.split("/")[1]?.split(":")[0]}
                        </span>
                        <span className="text-xs flex items-center gap-2">
                          {checkingModel === model && (
                            <span className="text-tertiary animate-pulse font-bold">
                              Checking...
                            </span>
                          )}
                          {!checkingModel && modelStatuses[model]?.working && (
                            <span className="text-green-500 font-medium">
                              ✓ {modelStatuses[model]?.latency}ms
                            </span>
                          )}
                          {!checkingModel && modelStatuses[model]?.working === false && (
                            <span className="text-red-500 font-medium">
                              ✗ Error
                            </span>
                          )}
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              checkOpenRouterModelStatus(model);
                            }}
                            disabled={!!checkingModel}
                            className="text-primary hover:text-primary disabled:opacity-50 font-bold"
                          >
                            Check
                          </button>
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
              <p className="text-xs text-tertiary mt-2 font-bold">
                Select a free OpenRouter model.
              </p>
            </div>
          )}
        </div>
      )}

      {/* Groq Models Dropdown */}
      {config.llm_provider === "groq" && config.groq_api_key_set && (
        <div>
          <div className="flex items-center justify-between mb-4">
            <h4 className="font-black text-lg uppercase tracking-tighter text-text">
              Groq Model
            </h4>
          </div>

          {groqModelsLoading ? (
            <p className="text-sm text-tertiary font-bold">Loading models...</p>
          ) : (
            <div className="space-y-2">
              <div className="relative" ref={groqDropdownRef}>
                <button
                  onClick={() =>
                    !saving && setIsGroqDropdownOpen(!isGroqDropdownOpen)
                  }
                  disabled={saving === "groq_model"}
                  className="w-full px-4 py-2.5 bg-secondary border border-border rounded-xl
                             text-text flex items-center justify-between
                             focus:outline-none focus:ring-2 focus:border-primary
                             disabled:opacity-50 text-left font-bold"
                >
                  <span>
                    {config.groq_model}
                    {modelStatuses[config.groq_model]?.working && (
                      <span className="text-green-500 ml-2 text-xs font-bold">
                        ✓ {modelStatuses[config.groq_model]?.latency}ms
                        {" | "}
                        {modelStatuses[config.groq_model]?.tps?.toFixed(1)} t/s
                      </span>
                    )}
                    {modelStatuses[config.groq_model]?.working === false && (
                      <span className="text-red-500 ml-2 text-xs font-bold">
                        ✗ Error
                      </span>
                    )}
                  </span>
                  <svg
                    className={`w-4 h-4 text-tertiary transition-transform ${isGroqDropdownOpen ? "rotate-180" : ""}`}
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

                {isGroqDropdownOpen && (
                  <div className="absolute z-20 w-full mt-1 bg-secondary border border-border rounded-xl shadow-xl max-h-60 overflow-y-auto">
                    {groqModels.map((model) => (
                      <div
                        key={model}
                        onClick={() => {
                          handleGroqModelSelect(model);
                          setIsGroqDropdownOpen(false);
                        }}
                        className={`flex items-center justify-between px-4 py-3 cursor-pointer transition-colors ${
                          config.groq_model === model
                            ? "bg-primary/10 text-primary"
                            : "text-text hover:bg-secondary"
                        }`}
                      >
                        <span className="font-medium text-sm">{model}</span>
                        <span className="text-xs flex items-center gap-2">
                          {checkingModel === model && (
                            <span className="text-tertiary animate-pulse font-bold">
                              Checking...
                            </span>
                          )}
                          {!checkingModel && modelStatuses[model]?.working && (
                            <span className="text-green-500 font-medium">
                              ✓ {modelStatuses[model]?.latency}ms
                              {" | "}
                              {modelStatuses[model]?.tps?.toFixed(1)} t/s
                            </span>
                          )}
                          {!checkingModel && modelStatuses[model]?.working === false && (
                            <span className="text-red-500 font-medium">
                              ✗ Error
                            </span>
                          )}
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              checkGroqModelStatus(model);
                            }}
                            disabled={!!checkingModel}
                            className="text-primary hover:text-primary disabled:opacity-50 font-bold"
                          >
                            Check
                          </button>
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
              <p className="text-xs text-tertiary mt-2 font-bold">
                Select a Groq model for ultra-fast LPU inference constraints.
              </p>
            </div>
          )}
        </div>
      )}

      {/* Cerebras Models Dropdown */}
      {config.llm_provider === "cerebras" && config.cerebras_api_key_set && (
        <div>
          <div className="flex items-center justify-between mb-4">
            <h4 className="font-black text-lg uppercase tracking-tighter text-text">
              Cerebras Model
            </h4>
          </div>

          {cerebrasModelsLoading ? (
            <p className="text-sm text-tertiary font-bold">Loading models...</p>
          ) : (
            <div className="space-y-2">
              <div className="relative" ref={cerebrasDropdownRef}>
                <button
                  onClick={() =>
                    !saving && setIsCerebrasDropdownOpen(!isCerebrasDropdownOpen)
                  }
                  disabled={saving === "cerebras_model"}
                  className="w-full px-4 py-2.5 bg-secondary border border-border rounded-xl
                             text-text flex items-center justify-between
                             focus:outline-none focus:ring-2 focus:border-primary
                             disabled:opacity-50 text-left font-bold"
                >
                  <span>
                    {config.cerebras_model}
                    {modelStatuses[config.cerebras_model]?.working && (
                      <span className="text-green-500 ml-2 text-xs font-bold">
                        ✓ {modelStatuses[config.cerebras_model]?.latency}ms
                      </span>
                    )}
                    {modelStatuses[config.cerebras_model]?.working === false && (
                      <span className="text-red-500 ml-2 text-xs font-bold">
                        ✗ Error
                      </span>
                    )}
                  </span>
                  <svg
                    className={`w-4 h-4 text-tertiary transition-transform ${isCerebrasDropdownOpen ? "rotate-180" : ""}`}
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

                {isCerebrasDropdownOpen && (
                  <div className="absolute z-20 w-full mt-1 bg-secondary border border-border rounded-xl shadow-xl max-h-60 overflow-y-auto">
                    {cerebrasModels.map((model) => (
                      <div
                        key={model}
                        onClick={() => {
                          handleCerebrasModelSelect(model);
                          setIsCerebrasDropdownOpen(false);
                        }}
                        className={`flex items-center justify-between px-4 py-3 cursor-pointer transition-colors ${
                          config.cerebras_model === model
                            ? "bg-primary/10 text-primary"
                            : "text-text hover:bg-secondary"
                        }`}
                      >
                        <span className="font-medium text-sm">{model}</span>
                        <span className="text-xs flex items-center gap-2">
                          {checkingModel === model && (
                            <span className="text-tertiary animate-pulse font-bold">
                              Checking...
                            </span>
                          )}
                          {!checkingModel && modelStatuses[model]?.working && (
                            <span className="text-green-500 font-medium">
                              ✓ {modelStatuses[model]?.latency}ms
                            </span>
                          )}
                          {!checkingModel && modelStatuses[model]?.working === false && (
                            <span className="text-red-500 font-medium">
                              ✗ Error
                            </span>
                          )}
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              checkCerebrasModelStatus(model);
                            }}
                            disabled={!!checkingModel}
                            className="text-primary hover:text-primary disabled:opacity-50 font-bold"
                          >
                            Check
                          </button>
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
              <p className="text-xs text-tertiary mt-2 font-bold">
                Select a Cerebras model to leverage wafer-scale high-speed inference.
              </p>
            </div>
          )}
        </div>
      )}
    </section>
  );
}
