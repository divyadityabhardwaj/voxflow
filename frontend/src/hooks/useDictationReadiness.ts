import { useCallback, useEffect, useRef, useState } from "react";
import {
  GetConfig, GetPermissions, GetProviders, GetPushToTalkKey,
  IsModelDownloaded, IsModelReady, IsWhisperCLIReady,
  OpenPrivacySettings, PromptAccessibilityExplanation,
  RequestMicrophoneAccess, SetPushToTalkKey,
} from "../../wailsjs/go/main/App";
import { EventsEmit, EventsOn } from "../../wailsjs/runtime/runtime";
import { Events } from "../constants/events";
import { evaluateReadiness } from "../lib/readiness";

export function useDictationReadiness() {
  const [setup, setSetup] = useState<ReturnType<typeof evaluateReadiness> | null>(null);
  const [error, setError] = useState(false);
  const request = useRef(0);
  const hadAccessibility = useRef<boolean | null>(null);

  const refresh = useCallback(async () => {
    const id = ++request.current;
    try {
      const [cfg, perms, providers] = await Promise.all([GetConfig(), GetPermissions(), GetProviders()]);
      const local = cfg.transcription !== "groq";
      const [engineReady, modelDownloaded, modelReady] = local
        ? await Promise.all([IsWhisperCLIReady(), IsModelDownloaded(), IsModelReady()])
        : [true, true, true];
      if (id !== request.current) return;
      if (hadAccessibility.current === false && perms.accessibility) {
        GetPushToTalkKey().then((key) => SetPushToTalkKey(key.key)).catch(() => {});
      }
      hadAccessibility.current = perms.accessibility;
      const provider = providers.find((p) => p.id === cfg.llm_provider);
      setSetup(evaluateReadiness({
        microphone: perms.microphone,
        accessibility: perms.accessibility,
        transcription: cfg.transcription,
        groqKeySet: cfg.groq_api_key_set,
        engineReady, modelDownloaded, modelReady,
        refinementMode: cfg.refinement_mode || "refine",
        cleanupProvider: provider ? { name: provider.name, needsKey: provider.needs_key, keySet: provider.key_set } : undefined,
      }));
      setError(false);
    } catch {
      if (id === request.current) { setError(true); setSetup(null); }
    }
  }, []);

  useEffect(() => {
    refresh();
    window.addEventListener("focus", refresh);
    const offs = [Events.ModelStatus, Events.ModelDownloadComplete, Events.ModelLoadError]
      .map((event) => EventsOn(event, refresh));
    return () => {
      ++request.current;
      window.removeEventListener("focus", refresh);
      offs.forEach((off) => off());
    };
  }, [refresh]);

  // System Settings doesn't always return focus to this window after a grant.
  useEffect(() => {
    if (!setup?.needsPermission) return;
    const timer = setInterval(refresh, 2000);
    return () => clearInterval(timer);
  }, [setup?.needsPermission, refresh]);

  const fix = async () => {
    if (error) { await refresh(); return; }
    if (setup?.issue?.kind === "microphone") {
      if (setup.microphone === "notDetermined") await RequestMicrophoneAccess();
      else await OpenPrivacySettings("microphone");
      await refresh();
    } else if (setup?.issue?.kind === "accessibility") {
      await PromptAccessibilityExplanation();
      await refresh();
    } else {
      EventsEmit(Events.OpenSettings, "speech");
    }
  };

  return { setup, error, checking: !setup && !error, fix, refresh };
}

export type DictationReadiness = ReturnType<typeof useDictationReadiness>;
