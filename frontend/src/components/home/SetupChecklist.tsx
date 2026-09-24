import { useCallback, useEffect, useState } from "react";
import {
  GetConfig,
  GetPermissions,
  GetProviders,
  GetPushToTalkKey,
  OpenPrivacySettings,
  OpenSettings,
  PromptAccessibilityExplanation,
  RequestMicrophoneAccess,
  SetPushToTalkKey,
} from "../../../wailsjs/go/main/App";

interface Setup {
  mic: string;
  accessibility: boolean;
  copyOnly: boolean;
  cleanupOff: boolean;
}

async function loadSetup(): Promise<Setup> {
  const [perms, cfg, providers] = await Promise.all([GetPermissions(), GetConfig(), GetProviders()]);
  const provider = providers.find((p) => p.id === cfg.llm_provider);
  const mode = cfg.refinement_mode || "refine";
  return {
    mic: perms.microphone,
    accessibility: perms.accessibility,
    copyOnly: mode === "copy-only",
    cleanupOff: mode === "raw" || (mode === "refine" && !!provider?.needs_key && !provider.key_set),
  };
}

const POLL_MS = 1000;

// Shown only while something needs fixing.
export default function SetupChecklist() {
  const [setup, setSetup] = useState<Setup | null>(null);

  const refresh = useCallback(async () => {
    const next = await loadSetup().catch(() => null);
    if (!next) return;
    setSetup((prev) => {
      // The hold key's event tap needs Accessibility; retry it once it's granted.
      if (prev && !prev.accessibility && next.accessibility) {
        GetPushToTalkKey().then((k) => SetPushToTalkKey(k.key)).catch(() => {});
      }
      return next;
    });
  }, []);

  useEffect(() => {
    refresh();
    window.addEventListener("focus", refresh);
    return () => window.removeEventListener("focus", refresh);
  }, [refresh]);

  const micOk = setup?.mic === "authorized";
  const pasteOk = !!setup && (setup.accessibility || setup.copyOnly);

  // Permissions are granted in System Settings, which doesn't focus us on return.
  useEffect(() => {
    if (!setup || (micOk && pasteOk)) return;
    const t = setInterval(refresh, POLL_MS);
    return () => clearInterval(t);
  }, [setup, micOk, pasteOk, refresh]);

  if (!setup || (micOk && pasteOk && !setup.cleanupOff)) return null;

  const fixMic = async () => {
    if (setup.mic === "notDetermined") await RequestMicrophoneAccess();
    else await OpenPrivacySettings("microphone");
    refresh();
  };

  const items = [
    {
      ok: micOk,
      label: micOk ? "Microphone" : "Microphone is blocked",
      action: "Fix",
      onFix: fixMic,
    },
    {
      ok: pasteOk,
      label: pasteOk ? "Paste permission" : "VoxFlow can't paste yet — text is copied instead",
      action: "Fix",
      onFix: () => PromptAccessibilityExplanation(),
    },
    {
      ok: !setup.cleanupOff,
      label: setup.cleanupOff ? "Clean-up is off — 'um's and punctuation stay as spoken" : "Clean-up",
      action: "Set up",
      onFix: () => OpenSettings(),
    },
  ];

  return (
    <section aria-label="Finish setting up" className="card p-3 w-full">
      <h2 className="text-[13px] font-semibold text-text px-1 mb-1.5">Finish setting up</h2>
      <ul>
        {items.map((it) => (
          <li key={it.label} className="flex items-center gap-2.5 px-1 py-1.5">
            <span
              className={`flex-none size-5 rounded-full flex items-center justify-center text-[11px] font-bold ${
                it.ok ? "bg-secondary text-[var(--success)]" : "bg-processing text-[#1c1917]"
              }`}
              aria-hidden="true"
            >
              {it.ok ? "✓" : "!"}
            </span>
            <span className={`flex-1 text-[13px] ${it.ok ? "text-secondary" : "text-text"}`}>
              {it.label}
              <span className="sr-only">{it.ok ? " — done" : " — needs attention"}</span>
            </span>
            {!it.ok && (
              <button type="button" className="btn-secondary !py-1 !px-3 !text-xs" onClick={it.onFix}>
                {it.action}
              </button>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
