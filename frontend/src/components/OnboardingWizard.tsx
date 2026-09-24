import { useCallback, useEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { main } from "../../wailsjs/go/models";
import {
  CheckProviderModel,
  CompleteOnboarding,
  GetConfig,
  GetPermissions,
  GetProviders,
  GetPushToTalkKey,
  IsModelDownloaded,
  IsModelReady,
  OpenPrivacySettings,
  PromptAccessibilityExplanation,
  RequestMicrophoneAccess,
  SetProvider,
  SetProviderAPIKey,
  SetPushToTalkKey,
  SetRefinementMode,
  ShowMiniMode,
} from "../../wailsjs/go/main/App";
import { useModelDownload } from "../hooks/useModelDownload";
import { loadShortcuts, type Shortcuts } from "../hooks/useDictation";
import Keycap from "./dictation/Keycap";
import TryIt from "./onboarding/TryIt";

interface Props {
  onComplete: () => void;
}

const STEPS = ["welcome", "microphone", "paste", "shortcut", "try", "cleanup", "ready"] as const;
type Step = (typeof STEPS)[number];

const POLL_MS = 1000;

const KEY_URLS: Record<string, string> = {
  gemini: "https://aistudio.google.com/apikey",
  openrouter: "https://openrouter.ai/settings/keys",
  groq: "https://console.groq.com/keys",
  cerebras: "https://cloud.cerebras.ai",
};

const KEY_PLACEHOLDERS: Record<string, string> = {
  gemini: "AIza…",
  openrouter: "sk-or-…",
  groq: "gsk_…",
  cerebras: "csk-…",
};

const describeKeyError = (err: unknown) => {
  const msg = String(err);
  const status = msg.match(/status:? (\d{3})/)?.[1];
  if (status === "429") return "This key is out of quota or rate-limited right now.";
  if (status) return `That key didn't work (error ${status}). Check it and try again.`;
  if (msg.includes("failed to send request")) return "Couldn't reach the service. Check your internet connection.";
  return `Couldn't check the key: ${msg.slice(0, 160)}`;
};

const DRAG = { "--wails-draggable": "drag" } as unknown as CSSProperties;
const NO_DRAG = { "--wails-draggable": "no-drag" } as unknown as CSSProperties;

// Re-run `check` every second while `active`, e.g. while System Settings is open.
function usePoll(active: boolean, check: () => void) {
  useEffect(() => {
    if (!active) return;
    check();
    const t = setInterval(check, POLL_MS);
    return () => clearInterval(t);
  }, [active, check]);
}

const Check = ({ children }: { children: ReactNode }) => (
  <p className="text-[13px] font-medium text-[var(--success)] mb-4" role="status">
    <span aria-hidden="true">✓ </span>
    {children}
  </p>
);

export default function OnboardingWizard({ onComplete }: Props) {
  const [step, setStep] = useState<Step>("welcome");
  const [modelReady, setModelReady] = useState(false);
  const download = useModelDownload(() => setModelReady(true));

  const [mic, setMic] = useState<string>("notDetermined");
  const [micAsking, setMicAsking] = useState(false);
  const [accessibility, setAccessibility] = useState(false);
  const [pasteSkipped, setPasteSkipped] = useState(false);
  const [shortcuts, setShortcuts] = useState<Shortcuts | null>(null);
  const [holdKey, setHoldKey] = useState("");

  const [providers, setProviders] = useState<main.ProviderInfo[]>([]);
  const [provider, setProviderId] = useState("gemini");
  const [apiKey, setApiKey] = useState("");
  const [keyState, setKeyState] = useState<"idle" | "checking" | "ok">("idle");
  const [keyError, setKeyError] = useState<string | null>(null);
  const [cleanupOn, setCleanupOn] = useState(false);

  const stepIndex = STEPS.indexOf(step);
  const next = () => setStep(STEPS[Math.min(STEPS.length - 1, stepIndex + 1)]);
  const back = () => setStep(STEPS[Math.max(0, stepIndex - 1)]);

  const checkPermissions = useCallback(() => {
    GetPermissions()
      .then((p) => {
        setMic(p.microphone);
        setAccessibility(p.accessibility);
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    checkPermissions();
    IsModelReady().then((ok) => ok && setModelReady(true));
    GetPushToTalkKey().then((k) => setHoldKey(k.key)).catch(() => {});
    Promise.all([GetProviders(), GetConfig()])
      .then(([list, cfg]) => {
        const cloud = list.filter((p) => !p.local);
        setProviders(cloud);
        const current = cloud.find((p) => p.id === cfg.llm_provider);
        if (current) setProviderId(current.id);
        if (current?.key_set && cfg.refinement_mode !== "raw") {
          setKeyState("ok");
          setCleanupOn(true);
        }
      })
      .catch(() => {});
  }, [checkPermissions]);

  // The permission panes live in System Settings, which doesn't refocus us.
  usePoll(step === "microphone" && mic !== "authorized" && mic !== "notDetermined", checkPermissions);
  usePoll(step === "paste" && !accessibility, checkPermissions);

  // Move on by itself only when the permission was granted on this step, so
  // Back still lands here.
  const waitingFor = useRef<Step | null>(null);
  useEffect(() => {
    const granted = step === "microphone" ? mic === "authorized" : step === "paste" ? accessibility : null;
    if (granted === null) return;
    if (!granted) {
      waitingFor.current = step;
      return;
    }
    // The hold key's event tap needs Accessibility; retry it now that it's granted.
    if (step === "paste" && holdKey) SetPushToTalkKey(holdKey).catch(() => {});
    if (waitingFor.current !== step) return;
    waitingFor.current = null;
    const t = setTimeout(next, 900);
    return () => clearTimeout(t);
  }, [step, mic, accessibility, holdKey]);

  useEffect(() => {
    if (step === "shortcut" || step === "try" || step === "ready") {
      loadShortcuts().then(setShortcuts).catch(() => {});
    }
  }, [step]);

  const start = async () => {
    next();
    // Overlap the download with the permission steps.
    if (!modelReady && !download.downloading && !(await IsModelDownloaded())) download.start();
  };

  const allowMic = async () => {
    setMicAsking(true);
    try {
      await RequestMicrophoneAccess();
    } finally {
      setMicAsking(false);
      checkPermissions();
    }
  };

  const verifyKey = async () => {
    const key = apiKey.trim();
    if (!key) return;
    setKeyError(null);
    setKeyState("checking");
    try {
      await SetProvider(provider);
      await SetProviderAPIKey(provider, key);
      const info = (await GetProviders()).find((p) => p.id === provider);
      await CheckProviderModel(provider, info?.model || info?.default_model || "");
      setKeyState("ok");
      setCleanupOn(true);
    } catch (err) {
      setKeyError(describeKeyError(err));
      setKeyState("idle");
    }
  };

  // One setting holds both choices, so copy-only (no paste permission) wins.
  const finishCleanup = async (on: boolean) => {
    setCleanupOn(on);
    await SetRefinementMode(pasteSkipped ? "copy-only" : on ? "refine" : "raw").catch(() => {});
    next();
  };

  const finish = async () => {
    await CompleteOnboarding();
    const skipped = pasteSkipped || !cleanupOn || mic !== "authorized";
    // Anything skipped shows up as a checklist on Home; otherwise get out of the way.
    if (!skipped && modelReady) ShowMiniMode();
    onComplete();
  };

  const providerInfo = providers.find((p) => p.id === provider);

  let content: ReactNode;
  let primary: ReactNode = null;

  switch (step) {
    case "welcome":
      content = (
        <>
          <h1 className="text-2xl font-semibold text-text mb-3">Write with your voice — in any app.</h1>
          <p className="text-sm text-secondary leading-relaxed mb-4">
            Press a shortcut, speak naturally, and VoxFlow types clean text wherever your cursor is. Setup takes about 2
            minutes.
          </p>
          <p className="text-xs text-tertiary">Your voice is transcribed on this Mac. It never leaves your computer.</p>
        </>
      );
      primary = (
        <button type="button" className="btn-primary" onClick={start} autoFocus>
          Get started
        </button>
      );
      break;

    case "microphone":
      content = (
        <>
          <h2 className="text-lg font-semibold text-text mb-2">Let VoxFlow hear you.</h2>
          {mic === "authorized" ? (
            <Check>Microphone allowed</Check>
          ) : mic === "notDetermined" ? (
            <p className="text-sm text-secondary mb-4">macOS will ask for microphone access. Click Allow.</p>
          ) : (
            <div className="text-sm text-secondary mb-4 space-y-3" role="alert">
              <p>
                The microphone is blocked. Open System Settings › Privacy &amp; Security › Microphone and turn on
                VoxFlow. This page moves on by itself once it's on.
              </p>
              <button type="button" className="btn-secondary" onClick={() => OpenPrivacySettings("microphone")}>
                Open System Settings
              </button>
            </div>
          )}
        </>
      );
      primary =
        mic === "authorized" ? (
          <button type="button" className="btn-primary" onClick={next}>
            Continue
          </button>
        ) : mic === "notDetermined" ? (
          <button type="button" className="btn-primary" onClick={allowMic} disabled={micAsking}>
            {micAsking ? "Waiting for macOS…" : "Allow microphone"}
          </button>
        ) : (
          <button type="button" className="btn-ghost" onClick={next}>
            Skip for now
          </button>
        );
      break;

    case "paste":
      content = (
        <>
          <h2 className="text-lg font-semibold text-text mb-2">Let VoxFlow type for you.</h2>
          {accessibility ? (
            <Check>Access granted</Check>
          ) : (
            <>
              <p className="text-sm text-secondary mb-3">
                To put text where your cursor is, macOS needs you to switch VoxFlow on under Accessibility.
              </p>
              <p className="text-xs text-tertiary mb-4">
                In the window that opens, turn on the switch next to VoxFlow. This page moves on by itself.
              </p>
            </>
          )}
        </>
      );
      primary = accessibility ? (
        <button type="button" className="btn-primary" onClick={next}>
          Continue
        </button>
      ) : (
        <>
          <button
            type="button"
            className="btn-ghost"
            onClick={() => {
              setPasteSkipped(true);
              next();
            }}
          >
            Skip — I'll paste myself
          </button>
          <button type="button" className="btn-primary" onClick={() => PromptAccessibilityExplanation()}>
            Open System Settings
          </button>
        </>
      );
      break;

    case "shortcut":
      content = (
        <>
          <h2 className="text-lg font-semibold text-text mb-4">Your dictation keys</h2>
          {shortcuts && (
            <ul className="space-y-3 mb-4 text-sm text-secondary">
              <li className="flex items-center gap-2 flex-wrap">
                Hold <Keycap>{shortcuts.hold}</Keycap> to talk, let go to paste.
              </li>
              <li className="flex items-center gap-2 flex-wrap">
                <Keycap>{shortcuts.toggle}</Keycap> starts and stops a longer dictation.
              </li>
            </ul>
          )}
          <p className="text-sm text-secondary mb-2">
            Tip: press <Keycap>Esc</Keycap> while talking to cancel.
          </p>
          {holdKey === "fn" && shortcuts?.hold === "fn" && (
            <p className="text-xs text-tertiary mb-2">
              If fn opens the emoji picker, set “Press 🌐 key to” to “Do nothing” in System Settings › Keyboard.
            </p>
          )}
          {pasteSkipped && holdKey !== "chord" && (
            <p className="text-xs text-tertiary mb-2">
              Holding a single key needs the paste permission, so the shortcut above is used until it's on.
            </p>
          )}
          <p className="text-xs text-tertiary">You can change these later in Settings.</p>
        </>
      );
      primary = (
        <button type="button" className="btn-primary" onClick={next}>
          Continue
        </button>
      );
      break;

    case "try":
      content = (
        <>
          <h2 className="text-lg font-semibold text-text mb-2">Try it.</h2>
          <p className="text-sm text-secondary mb-4">Your text shows up in the box below — nothing is pasted anywhere else.</p>
          <TryIt shortcuts={shortcuts} modelReady={modelReady} downloadPercent={download.percent} />
        </>
      );
      primary = (
        <button type="button" className="btn-primary" onClick={next}>
          Continue
        </button>
      );
      break;

    case "cleanup":
      content = (
        <>
          <h2 className="text-lg font-semibold text-text mb-2">Want VoxFlow to tidy your text?</h2>
          <div className="card p-3 mb-3 text-[13px] space-y-1">
            <p className="text-tertiary">“um so hi this is uh my first dictation”</p>
            <p className="text-text">
              <span aria-hidden="true">→ </span>“Hi, this is my first dictation.”
            </p>
          </div>
          <p className="text-sm text-secondary mb-4">
            Removes “um”s, adds punctuation, formats lists. Uses an AI service with your own key — only text is sent,
            never audio.
          </p>
          {pasteSkipped && (
            <p className="text-xs text-tertiary mb-3">
              Clean-up is off while VoxFlow only copies your text. Turn on paste permission in Settings to use it.
            </p>
          )}
          {keyState === "ok" ? (
            <Check>Key works — clean-up is on{providerInfo ? ` with ${providerInfo.name}` : ""}</Check>
          ) : (
            <div className="space-y-3">
              {providers.length > 1 && (
                <div>
                  <label className="label" htmlFor="onboarding-provider">
                    Service
                  </label>
                  <select
                    id="onboarding-provider"
                    className="select"
                    value={provider}
                    onChange={(e) => {
                      setProviderId(e.target.value);
                      setKeyError(null);
                    }}
                  >
                    {providers.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.id === "gemini" ? `${p.name} — free, recommended` : p.name}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              <div>
                <div className="flex items-center justify-between">
                  <label className="label" htmlFor="onboarding-key">
                    API key
                  </label>
                  {KEY_URLS[provider] && (
                    <button
                      type="button"
                      className="text-xs text-primary hover:underline mb-1.5"
                      onClick={() => BrowserOpenURL(KEY_URLS[provider])}
                    >
                      {provider === "gemini" ? "Get a free key" : "Get a key"}
                    </button>
                  )}
                </div>
                <div className="flex gap-2">
                  <input
                    id="onboarding-key"
                    type="password"
                    className="input flex-1 min-w-0"
                    placeholder={KEY_PLACEHOLDERS[provider] ?? "Paste your key"}
                    value={apiKey}
                    onChange={(e) => {
                      setApiKey(e.target.value);
                      setKeyError(null);
                    }}
                    onKeyDown={(e) => e.key === "Enter" && verifyKey()}
                  />
                  <button
                    type="button"
                    className="btn-secondary shrink-0"
                    onClick={verifyKey}
                    disabled={!apiKey.trim() || keyState === "checking"}
                  >
                    {keyState === "checking" ? "Checking…" : "Verify"}
                  </button>
                </div>
                {keyError && (
                  <p role="alert" className="text-xs text-[var(--danger)] mt-2 break-words">
                    {keyError}
                  </p>
                )}
                <p className="hint">Other services, including ones on your own computer, are in Settings.</p>
              </div>
            </div>
          )}
        </>
      );
      primary =
        keyState === "ok" ? (
          <button type="button" className="btn-primary" onClick={() => finishCleanup(true)}>
            Continue
          </button>
        ) : (
          <button type="button" className="btn-ghost" onClick={() => finishCleanup(false)} disabled={keyState === "checking"}>
            Not now — paste exactly what I say
          </button>
        );
      break;

    case "ready":
      content = (
        <>
          <h2 className="text-lg font-semibold text-text mb-4">You're ready.</h2>
          <dl className="card p-4 grid grid-cols-[auto_1fr] gap-x-4 gap-y-2.5 items-center text-sm mb-4">
            {shortcuts && (
              <>
                <dt>
                  <Keycap>{shortcuts.hold}</Keycap>
                </dt>
                <dd className="text-secondary">Hold to talk</dd>
                <dt>
                  <Keycap>{shortcuts.toggle}</Keycap>
                </dt>
                <dd className="text-secondary">Start / stop</dd>
              </>
            )}
            <dt>
              <Keycap>Esc</Keycap>
            </dt>
            <dd className="text-secondary">Cancel</dd>
          </dl>
          <ul className="text-[13px] text-secondary space-y-1 list-disc pl-5">
            <li>A small pill shows while you talk.</li>
            <li>VoxFlow lives in your menu bar.</li>
          </ul>
        </>
      );
      primary = (
        <button type="button" className="btn-primary" onClick={finish} autoFocus>
          Start dictating
        </button>
      );
      break;
  }

  const showDownload = step !== "welcome" && (!modelReady || download.error);

  return (
    <div className="h-full app-shell flex items-center justify-center p-8" style={DRAG}>
      <div className="max-w-lg w-full card p-8 shadow-soft-lg" style={NO_DRAG}>
        <div className="flex gap-1.5 mb-8 py-1" style={DRAG} aria-label={`Step ${stepIndex + 1} of ${STEPS.length}`} role="img">
          {STEPS.map((s, i) => (
            <div key={s} className={`h-1 flex-1 rounded-full ${i <= stepIndex ? "bg-primary" : "bg-border"}`} />
          ))}
        </div>

        <div key={step} className="animate-fade-in">
          {content}
        </div>

        <div className="flex items-center gap-2 mt-8">
          {stepIndex > 0 && step !== "ready" && (
            <button type="button" className="btn-secondary" onClick={back}>
              Back
            </button>
          )}
          <div className="flex-1" />
          {primary}
        </div>

        {showDownload && (
          <div className="mt-6 pt-4 border-t border-border text-xs text-secondary" role="status" aria-live="polite">
            {download.error ? (
              <div className="flex items-center gap-2">
                <span className="flex-1">Download paused — check your connection.</span>
                <button type="button" className="btn-secondary !py-1 !px-2.5 !text-xs" onClick={download.start}>
                  Retry
                </button>
              </div>
            ) : (
              <>
                <div className="flex justify-between mb-1.5">
                  <span>Downloading speech engine</span>
                  <span className="tabular-nums">
                    {download.percent}%
                    {download.totalMB > 0 && ` (${Math.round(download.downloadedMB)} of ${Math.round(download.totalMB)} MB)`}
                  </span>
                </div>
                <div className="progress-bar">
                  <div style={{ width: `${download.percent}%` }} />
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
