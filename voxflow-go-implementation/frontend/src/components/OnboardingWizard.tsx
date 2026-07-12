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
} from "../../wailsjs/go/main/App";

interface Props {
  onComplete: () => void;
}

const STEPS = ["welcome", "accessibility", "model", "api", "done"] as const;
type Step = (typeof STEPS)[number];

export default function OnboardingWizard({ onComplete }: Props) {
  const [step, setStep] = useState<Step>("welcome");
  const [modelReady, setModelReady] = useState(false);
  const [apiKey, setApiKey] = useState("");
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

  const handleSaveApiKey = async () => {
    if (apiKey.trim()) {
      await SetAPIKey(apiKey.trim());
      await GetConfig();
    }
    setStep("done");
  };

  return (
    <div className="h-full bg-background flex items-center justify-center p-8">
      <div className="max-w-lg w-full settings-card p-8">
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
              A quick setup covers permissions, the local Whisper model, and an
              optional Gemini API key for text refinement.
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
              <p className="text-xs text-green-600 font-bold mb-4">
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
              Whisper is installed. You can add an API key next or finish setup.
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
              Gemini API key (optional)
            </h2>
            <p className="text-sm text-secondary mb-4">
              Refinement polishes dictation with an LLM. You can also set this
              later in Settings.
            </p>
            <input
              type="password"
              className="input w-full mb-4"
              placeholder="AIza..."
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
            />
            <div className="flex flex-col gap-3">
              <button
                type="button"
                className="btn-primary w-full"
                onClick={handleSaveApiKey}
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
