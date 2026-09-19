import { useState, useEffect } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { Events } from "../constants/events";
import {
  CompleteOnboarding,
  DownloadModel,
  GetConfig,
  IsAccessibilityGranted,
  IsModelDownloaded,
  PromptAccessibilityExplanation,
  SetAPIKey,
  SetCerebrasAPIKey,
  SetGroqAPIKey,
  SetLLMProvider,
  SetLocalModel,
  SetLocalURL,
  SetOpenRouterAPIKey,
} from "../../wailsjs/go/main/App";
import { KEY_URLS, PROVIDERS } from "./settings/LLMProviderSettings";

interface Props {
  onComplete: () => void;
}

const STEPS = ["welcome", "accessibility", "model", "api", "done"] as const;
type Step = (typeof STEPS)[number];

const DEFAULT_LOCAL_URL = "http://localhost:11434";

const KEY_SETTERS: Record<string, (key: string) => Promise<void>> = {
  gemini: SetAPIKey,
  openrouter: SetOpenRouterAPIKey,
  groq: SetGroqAPIKey,
  cerebras: SetCerebrasAPIKey,
};

const KEY_PLACEHOLDERS: Record<string, string> = {
  gemini: "AIza...",
  openrouter: "sk-or-...",
  groq: "gsk_...",
  cerebras: "csk-...",
};

export default function OnboardingWizard({ onComplete }: Props) {
  const [step, setStep] = useState<Step>("welcome");
  const [modelReady, setModelReady] = useState(false);
  const [provider, setProvider] = useState("gemini");
  const [apiKey, setApiKey] = useState("");
  const [localURL, setLocalURL] = useState(DEFAULT_LOCAL_URL);
  const [localModel, setLocalModel] = useState("");
  const [accessibilityGranted, setAccessibilityGranted] = useState(false);
  const [downloading, setDownloading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [downloadedMB, setDownloadedMB] = useState(0);
  const [totalMB, setTotalMB] = useState(0);

  const stepIndex = STEPS.indexOf(step);

  useEffect(() => {
    IsModelDownloaded().then((ok) => {
      if (ok) setModelReady(true);
    });

    GetConfig().then((cfg) => {
      setProvider(cfg.llm_provider || "gemini");
      setLocalURL(cfg.local_url || DEFAULT_LOCAL_URL);
      setLocalModel(cfg.local_model || "");
    });

    const checkAccess = () => {
      IsAccessibilityGranted().then((ok) => {
        setAccessibilityGranted(ok);
      });
    };

    checkAccess();
    window.addEventListener("focus", checkAccess);

    const unsubDownload = EventsOn(Events.ModelDownloadProgress, (data: { progress: number; downloaded?: number; total?: number }) => {
      setProgress(Math.round(data.progress));
      if (data.downloaded !== undefined) {
        setDownloadedMB(Number((data.downloaded / (1024 * 1024)).toFixed(1)));
      }
      if (data.total !== undefined) {
        setTotalMB(Number((data.total / (1024 * 1024)).toFixed(1)));
      }
    });

    const unsubModel = EventsOn(
      Events.ModelStatus,
      (status: { downloaded: boolean; loaded: boolean }) => {
        if (status.downloaded && status.loaded) {
          setModelReady(true);
          setDownloading(false);
        }
      },
    );

    return () => {
      window.removeEventListener("focus", checkAccess);
      unsubDownload();
      unsubModel();
    };
  }, []);

  const finish = async () => {
    await CompleteOnboarding();
    onComplete();
  };

  const handleAccessibility = async () => {
    await PromptAccessibilityExplanation();
    const granted = await IsAccessibilityGranted();
    setAccessibilityGranted(granted);
    if (granted) {
      setStep("model");
    }
  };

  const handleSaveProvider = async () => {
    await SetLLMProvider(provider);
    if (provider === "local") {
      await SetLocalURL(localURL.trim() || DEFAULT_LOCAL_URL);
      await SetLocalModel(localModel.trim());
    } else if (apiKey.trim()) {
      await KEY_SETTERS[provider](apiKey.trim());
    }
    setStep("done");
  };

  return (
    <div className="h-full app-shell flex items-center justify-center p-8">
      <div className="max-w-lg w-full card p-8">
        <div className="flex gap-2 mb-8">
          {STEPS.map((s, i) => (
            <div
              key={s}
              className={`h-1 flex-1 rounded-full ${
                i <= stepIndex ? "bg-primary" : "bg-border"
              }`}
            />
          ))}
        </div>

        {step === "welcome" && (
          <>
            <h1 className="text-2xl font-semibold text-text mb-3">
              Welcome to VoxFlow
            </h1>
            <p className="text-sm text-secondary mb-6 leading-relaxed">
              A quick setup covers permissions, the local Whisper model, and
              optional AI refinement of your dictation.
            </p>
            <button
              type="button"
              className="btn-primary w-full"
              onClick={() => setStep("accessibility")}
            >
              Get started
            </button>
          </>
        )}

        {step === "accessibility" && (
          <>
            <h2 className="text-lg font-semibold text-text mb-3">
              Accessibility
            </h2>
            <p className="text-sm text-secondary mb-6">
              VoxFlow pastes transcribed text into other apps. macOS requires
              Accessibility permission for simulated Cmd+V.
            </p>
            {accessibilityGranted && (
              <p className="text-xs text-[var(--success)] font-medium mb-4">
                ✓ Accessibility granted
              </p>
            )}
            <div className="flex flex-col gap-3">
              {accessibilityGranted ? (
                <button
                  type="button"
                  className="btn-primary w-full"
                  onClick={() => setStep("model")}
                >
                  Continue
                </button>
              ) : (
                <button
                  type="button"
                  className="btn-primary w-full"
                  onClick={handleAccessibility}
                >
                  Grant permission
                </button>
              )}
              {!accessibilityGranted && (
                <button
                  type="button"
                  className="btn-secondary w-full"
                  onClick={() => setStep("model")}
                >
                  Skip for now
                </button>
              )}
            </div>
          </>
        )}

        {step === "model" && !modelReady && (
          <>
            <h2 className="text-lg font-semibold text-text mb-3">
              Download speech model
            </h2>
            <p className="text-sm text-secondary mb-4">
              VoxFlow runs Whisper locally. This one-time download is required
              before your first dictation.
            </p>
            {downloading ? (
              <div className="mb-4">
                <div className="h-2 bg-border rounded-full overflow-hidden">
                  <div
                    className="h-full bg-primary transition-all"
                    style={{ width: `${progress}%` }}
                  />
                </div>
                <p className="text-xs text-tertiary font-bold mt-2">
                  {progress}% complete {totalMB > 0 ? `(${downloadedMB} MB / ${totalMB} MB)` : ""}
                </p>
              </div>
            ) : (
              <button
                type="button"
                className="btn-primary w-full mb-4"
                onClick={async () => {
                  setDownloading(true);
                  try {
                    await DownloadModel();
                  } catch {
                    setDownloading(false);
                  }
                }}
              >
                Download model
              </button>
            )}
          </>
        )}

        {step === "model" && modelReady && (
          <>
            <h2 className="text-lg font-semibold text-text mb-3">
              Model ready
            </h2>
            <p className="text-sm text-secondary mb-6">
              Whisper is installed. You can set up AI refinement next or finish
              setup.
            </p>
            <button
              type="button"
              className="btn-primary w-full"
              onClick={() => setStep("api")}
            >
              Continue
            </button>
          </>
        )}

        {step === "api" && (
          <>
            <h2 className="text-lg font-semibold text-text mb-3">
              AI refinement (optional)
            </h2>
            <p className="text-sm text-secondary mb-4">
              Refinement polishes dictation with an LLM. You can also set this
              later in Settings.
            </p>
            <label className="label" htmlFor="onboarding-provider">
              Provider
            </label>
            <select
              id="onboarding-provider"
              className="select w-full mb-4"
              value={provider}
              onChange={(e) => {
                setProvider(e.target.value);
                setApiKey("");
              }}
            >
              {PROVIDERS.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.label} — {p.sub}
                </option>
              ))}
            </select>
            {provider === "local" ? (
              <>
                <label className="label" htmlFor="onboarding-local-url">
                  Server URL
                </label>
                <input
                  id="onboarding-local-url"
                  type="text"
                  className="input w-full mb-4"
                  placeholder={DEFAULT_LOCAL_URL}
                  value={localURL}
                  onChange={(e) => setLocalURL(e.target.value)}
                />
                <label className="label" htmlFor="onboarding-local-model">
                  Model name
                </label>
                <input
                  id="onboarding-local-model"
                  type="text"
                  className="input w-full"
                  placeholder="qwen3:8b"
                  value={localModel}
                  onChange={(e) => setLocalModel(e.target.value)}
                />
                <p className="hint mb-4">
                  Works with Ollama, LM Studio or any OpenAI-compatible server.
                  Nothing leaves your machine.
                </p>
              </>
            ) : (
              <>
                <input
                  type="password"
                  className="input w-full"
                  placeholder={KEY_PLACEHOLDERS[provider]}
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                />
                <p className="hint mb-4">
                  <a
                    href={KEY_URLS[provider]}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline"
                  >
                    Get a key
                  </a>
                </p>
              </>
            )}
            <div className="flex flex-col gap-3">
              <button
                type="button"
                className="btn-primary w-full"
                onClick={handleSaveProvider}
              >
                Save & continue
              </button>
              <button
                type="button"
                className="btn-secondary w-full"
                onClick={() => setStep("done")}
              >
                Skip
              </button>
            </div>
          </>
        )}

        {step === "done" && (
          <>
            <h2 className="text-lg font-semibold text-text mb-3">
              You are all set
            </h2>
            <p className="text-sm text-secondary mb-4">
              Use your configured hotkeys to dictate from anywhere. Open the full
              app from the mini pill to change settings anytime.
            </p>
            <button type="button" className="btn-primary w-full" onClick={finish}>
              Start using VoxFlow
            </button>
          </>
        )}

        {step !== "welcome" && step !== "done" && step !== "model" && (
          <button
            type="button"
            className="text-xs text-tertiary font-bold mt-6 underline"
            onClick={() =>
              setStep(STEPS[Math.max(0, stepIndex - 1)] as Step)
            }
          >
            Back
          </button>
        )}

        {step === "model" && !modelReady && (
          <button
            type="button"
            className="text-xs text-tertiary font-bold mt-6 underline block"
            onClick={() => setStep("accessibility")}
          >
            Back
          </button>
        )}
      </div>
    </div>
  );
}
