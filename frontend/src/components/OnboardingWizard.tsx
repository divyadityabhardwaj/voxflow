import { useState, useEffect } from "react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { main } from "../../wailsjs/go/models";
import {
  CheckCerebrasModel,
  CheckGeminiModel,
  CheckGroqModel,
  CheckOpenRouterModel,
  CompleteOnboarding,
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
  SetRefinementMode,
} from "../../wailsjs/go/main/App";
import { KEY_URLS, PROVIDERS } from "./settings/LLMProviderSettings";
import { useModelDownload } from "../hooks/useModelDownload";

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

const KEY_CHECKERS: Record<string, (model: string) => Promise<unknown>> = {
  gemini: CheckGeminiModel,
  openrouter: CheckOpenRouterModel,
  groq: CheckGroqModel,
  cerebras: CheckCerebrasModel,
};

const providerConfig = (cfg: main.ConfigResponse, provider: string) =>
  ({
    gemini: { model: cfg.gemini_model, keySet: cfg.api_key_set },
    openrouter: { model: cfg.openrouter_model, keySet: cfg.openrouter_api_key_set },
    groq: { model: cfg.groq_model, keySet: cfg.groq_api_key_set },
    cerebras: { model: cfg.cerebras_model, keySet: cfg.cerebras_api_key_set },
  })[provider];

const GET_KEY_URLS: Record<string, string> = {
  ...KEY_URLS,
  gemini: "https://aistudio.google.com/apikey",
};

const describeKeyError = (err: unknown) => {
  const msg = String(err);
  const status = msg.match(/status:? (\d{3})/)?.[1];
  if (status === "429")
    return "This key is out of quota or rate-limited right now.";
  if (status) return `That key didn't work (error ${status}). Check it and try again.`;
  if (msg.includes("failed to send request"))
    return "Couldn't reach the service. Check your internet connection.";
  return `Couldn't check the key: ${msg.slice(0, 160)}`;
};

// Local stand-in until the shared shortcut formatter lands.
const MODIFIER_GLYPHS: [string[], string][] = [
  [["ctrl", "control"], "⌃"],
  [["alt", "option", "opt"], "⌥"],
  [["shift"], "⇧"],
  [["cmd", "command", "super"], "⌘"],
];

const KEY_NAMES: Record<string, string> = {
  space: "Space",
  return: "Return",
  enter: "Return",
  escape: "Esc",
  esc: "Esc",
  tab: "Tab",
};

const formatHotkey = (hotkey: string) => {
  const parts = hotkey.toLowerCase().split("+");
  const key = parts.pop() ?? "";
  const mods = MODIFIER_GLYPHS.filter(([names]) =>
    names.some((n) => parts.includes(n)),
  )
    .map(([, glyph]) => glyph)
    .join("");
  return mods + (KEY_NAMES[key] ?? key.toUpperCase());
};

const KBD =
  "px-1.5 py-0.5 rounded-md bg-surface border border-border text-xs text-text shadow-sm";

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
  const [config, setConfig] = useState<main.ConfigResponse | null>(null);
  const [checkingKey, setCheckingKey] = useState(false);
  const [keyError, setKeyError] = useState<string | null>(null);
  const download = useModelDownload(() => setModelReady(true));

  const stepIndex = STEPS.indexOf(step);

  useEffect(() => {
    IsModelDownloaded().then((ok) => {
      if (ok) setModelReady(true);
    });

    GetConfig().then((cfg) => {
      setConfig(cfg);
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

    return () => {
      window.removeEventListener("focus", checkAccess);
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

  // Leaving "refine" on without a working provider warns on every dictation.
  const skipRefinement = async () => {
    await SetRefinementMode("raw");
    setStep("done");
  };

  const handleSaveProvider = async () => {
    setKeyError(null);
    await SetLLMProvider(provider);
    if (provider === "local") {
      await SetLocalURL(localURL.trim() || DEFAULT_LOCAL_URL);
      await SetLocalModel(localModel.trim());
      if (localModel.trim()) setStep("done");
      else await skipRefinement();
      return;
    }

    const key = apiKey.trim();
    if (!key) {
      if (config && providerConfig(config, provider)?.keySet) setStep("done");
      else await skipRefinement();
      return;
    }

    setCheckingKey(true);
    try {
      await KEY_SETTERS[provider](key);
      const cfg = await GetConfig();
      await KEY_CHECKERS[provider](providerConfig(cfg, provider)?.model ?? "");
      setStep("done");
    } catch (err) {
      setKeyError(describeKeyError(err));
    } finally {
      setCheckingKey(false);
    }
  };

  const handsFreeHotkey = config?.hands_free_hotkey || config?.hotkey || "";
  const pttHotkey = config?.push_to_talk_hotkey || "";

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
            {download.downloading ? (
              <div className="mb-4">
                <div className="h-2 bg-border rounded-full overflow-hidden">
                  <div
                    className="h-full bg-primary transition-all"
                    style={{ width: `${download.percent}%` }}
                  />
                </div>
                <p className="text-xs text-tertiary font-bold mt-2">
                  {download.percent}% complete{" "}
                  {download.totalMB > 0
                    ? `(${download.downloadedMB} MB / ${download.totalMB} MB)`
                    : ""}
                </p>
              </div>
            ) : (
              <>
                {download.error && (
                  <p
                    role="alert"
                    className="text-xs text-[var(--danger)] font-medium mb-3 break-words"
                  >
                    Couldn't set up the speech model: {download.error}
                  </p>
                )}
                <button
                  type="button"
                  className="btn-primary w-full mb-4"
                  onClick={download.start}
                >
                  {download.error ? "Retry" : "Download model"}
                </button>
              </>
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
                setKeyError(null);
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
                  aria-label="API key"
                  placeholder={KEY_PLACEHOLDERS[provider]}
                  value={apiKey}
                  onChange={(e) => {
                    setApiKey(e.target.value);
                    setKeyError(null);
                  }}
                />
                <p className="hint mb-4">
                  <button
                    type="button"
                    className="text-primary hover:underline"
                    onClick={() => BrowserOpenURL(GET_KEY_URLS[provider])}
                  >
                    Get a key
                  </button>
                </p>
              </>
            )}
            {keyError && (
              <p
                role="alert"
                className="text-xs text-[var(--danger)] font-medium mb-4 break-words"
              >
                {keyError}
              </p>
            )}
            <div className="flex flex-col gap-3">
              <button
                type="button"
                className="btn-primary w-full"
                onClick={handleSaveProvider}
                disabled={checkingKey}
              >
                {checkingKey ? "Checking key…" : "Save & continue"}
              </button>
              <button
                type="button"
                className="btn-secondary w-full"
                onClick={skipRefinement}
                disabled={checkingKey}
              >
                {keyError
                  ? "Continue without clean-up"
                  : "Not now — paste exactly what I say"}
              </button>
            </div>
          </>
        )}

        {step === "done" && (
          <>
            <h2 className="text-lg font-semibold text-text mb-3">
              You're ready
            </h2>
            {handsFreeHotkey || pttHotkey ? (
              <ul className="text-sm text-secondary mb-4 space-y-2">
                {handsFreeHotkey && (
                  <li>
                    Press <kbd className={KBD}>{formatHotkey(handsFreeHotkey)}</kbd>{" "}
                    to start dictating in any app, and again to finish.
                  </li>
                )}
                {pttHotkey && (
                  <li>
                    Or hold <kbd className={KBD}>{formatHotkey(pttHotkey)}</kbd>{" "}
                    while you talk and let go to paste.
                  </li>
                )}
              </ul>
            ) : (
              <p className="text-sm text-secondary mb-4">
                Use your configured hotkeys to dictate from anywhere.
              </p>
            )}
            <p className="text-sm text-secondary mb-4">
              Open the full app from the mini pill to change settings anytime.
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
