import { useCallback, useEffect, useState } from "react";
import {
  CheckProviderModel,
  GetConfig,
  GetProviderModels,
  GetProviders,
  SetLocalURL,
  SetProvider,
  SetProviderAPIKey,
  SetProviderModel,
  SetRefinementMode,
} from "../../../wailsjs/go/main/App";
import { main } from "../../../wailsjs/go/models";
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime";
import { useToast } from "../../contexts/ToastContext";
import { KEY_URLS, PROVIDERS, providerLabel } from "../../lib/providers";
import SettingsSection, {
  Disclosure,
  SettingRow,
  StatusText,
  Toggle,
} from "../ui/SettingsSection";

type Verify =
  | { state: "idle" }
  | { state: "checking" }
  | { state: "ok"; latency: number }
  | { state: "fail"; error: string };

export default function CleanupSettings() {
  const { showToast } = useToast();
  const fail = (what: string) => (err: unknown) =>
    showToast(`${what}: ${String(err)}`, "error");

  const [config, setConfig] = useState<main.ConfigResponse | null>(null);
  const [providers, setProviders] = useState<main.ProviderInfo[]>([]);
  const [keyDraft, setKeyDraft] = useState("");
  const [verify, setVerify] = useState<Verify>({ state: "idle" });
  const [models, setModels] = useState<string[]>([]);
  const [modelsError, setModelsError] = useState<string | null>(null);
  const [localURL, setLocalURLDraft] = useState("");
  const [localModel, setLocalModel] = useState("");

  const reloadProviders = useCallback(
    () => GetProviders().then((p) => setProviders(p ?? [])),
    [],
  );

  useEffect(() => {
    GetConfig()
      .then((c) => {
        setConfig(c);
        setLocalURLDraft(c.local_url || "http://localhost:11434");
      })
      .catch(fail("Couldn't load settings"));
    reloadProviders().catch(fail("Couldn't load AI services"));
  }, []);

  const id = config?.llm_provider || "gemini";
  const provider = providers.find((p) => p.id === id);
  const model = provider ? provider.model || provider.default_model : "";
  const canListModels = !!provider && (provider.key_set || !provider.needs_key);

  useEffect(() => {
    if (provider?.local) setLocalModel(provider.model);
  }, [provider?.id, provider?.model]);

  useEffect(() => {
    setModels([]);
    setModelsError(null);
    if (!canListModels) return;
    GetProviderModels(id)
      .then((m) => setModels(m ?? []))
      .catch((err) => setModelsError(String(err)));
  }, [id, canListModels]);

  if (!config) {
    return <p className="text-sm text-tertiary">Loading…</p>;
  }

  const mode = config.refinement_mode || "refine";
  const enabled = mode === "refine";

  const setEnabled = (on: boolean) => {
    const next = on ? "refine" : "raw";
    SetRefinementMode(next)
      .then(() => setConfig({ ...config, refinement_mode: next } as main.ConfigResponse))
      .catch(fail("Couldn't turn clean-up " + (on ? "on" : "off")));
  };

  const runVerify = async (checkModel = model) => {
    setVerify({ state: "checking" });
    try {
      const r = await CheckProviderModel(id, checkModel);
      setVerify({ state: "ok", latency: r.latency });
    } catch (err) {
      setVerify({ state: "fail", error: String(err) });
    }
  };

  const selectProvider = (next: string) =>
    SetProvider(next)
      .then(() => {
        setConfig({ ...config, llm_provider: next } as main.ConfigResponse);
        setVerify({ state: "idle" });
        setKeyDraft("");
        return reloadProviders();
      })
      .catch(fail("Couldn't change the AI clean-up service"));

  const saveKey = async () => {
    const key = keyDraft.trim();
    if (!key) return;
    try {
      await SetProviderAPIKey(id, key);
      setKeyDraft("");
      await reloadProviders();
      runVerify();
    } catch (err) {
      fail("Couldn't save the key")(err);
    }
  };

  const removeKey = () =>
    SetProviderAPIKey(id, "")
      .then(() => {
        setVerify({ state: "idle" });
        return reloadProviders();
      })
      .catch(fail("Couldn't remove the key"));

  const selectModel = (m: string) =>
    SetProviderModel(id, m)
      .then(() => {
        setVerify({ state: "idle" });
        return reloadProviders();
      })
      .catch(fail("Couldn't save the model"));

  const saveLocal = async () => {
    try {
      await SetLocalURL(localURL.trim());
      await SetProviderModel(id, localModel.trim());
      setConfig({ ...config, local_url: localURL.trim() } as main.ConfigResponse);
      await reloadProviders();
      runVerify(localModel.trim());
    } catch (err) {
      fail("Couldn't save your server settings")(err);
    }
  };

  const label = providerLabel(id);
  const verifyStatus =
    verify.state === "checking" ? (
      <StatusText tone="muted">Checking…</StatusText>
    ) : verify.state === "ok" ? (
      <StatusText tone="success">
        Connected ✓ ({(verify.latency / 1000).toFixed(1)} s)
      </StatusText>
    ) : verify.state === "fail" ? (
      <StatusText tone="danger">
        Couldn't connect — check your key or pick another model. ({verify.error})
      </StatusText>
    ) : null;

  const statusLine = !enabled
    ? mode === "copy-only"
      ? "Off — text is copied as you said it (see General › After you speak)."
      : "Off — text is pasted exactly as you said it."
    : provider?.needs_key && !provider.key_set
      ? `On · ${label} needs a key below before it can clean up.`
      : `On · ${label}`;

  return (
    <div className="space-y-6">
      <SettingsSection title="AI clean-up">
        <SettingRow
          label={<span id="cleanup-label">Clean up my text</span>}
          description={
            <span id="cleanup-desc">
              Removes filler words, fixes punctuation and capitalisation, and
              formats lists. {statusLine}
            </span>
          }
        >
          <Toggle
            checked={enabled}
            onChange={setEnabled}
            labelledBy="cleanup-label"
            describedBy="cleanup-desc"
          />
        </SettingRow>
      </SettingsSection>

      {provider?.local ? (
        <SettingsSection
          title="Your own server"
          description="An OpenAI-compatible server such as Ollama or LM Studio. Your text stays on this Mac."
        >
          <div className="settings-pad space-y-3">
            <div>
              <label className="label" htmlFor="local-url">
                Server URL
              </label>
              <input
                id="local-url"
                className="input"
                value={localURL}
                onChange={(e) => setLocalURLDraft(e.target.value)}
                placeholder="http://localhost:11434"
              />
              <p className="hint">Ollama: localhost:11434 · LM Studio: localhost:1234</p>
            </div>
            <div>
              <label className="label" htmlFor="local-model">
                Model
              </label>
              <input
                id="local-model"
                className="input"
                list="local-models"
                value={localModel}
                onChange={(e) => setLocalModel(e.target.value)}
                placeholder="qwen3:8b"
              />
              <datalist id="local-models">
                {models.map((m) => (
                  <option key={m} value={m} />
                ))}
              </datalist>
            </div>
            <div className="flex items-center gap-3 flex-wrap">
              <button
                type="button"
                className="btn btn-primary"
                disabled={!localModel.trim() || verify.state === "checking"}
                onClick={saveLocal}
              >
                Save and verify
              </button>
              {verifyStatus}
            </div>
          </div>
        </SettingsSection>
      ) : (
        provider && (
          <SettingsSection
            title={`${label} key`}
            description="Only text is sent — never audio."
          >
            <div className="settings-pad space-y-3">
              <p className="text-[13px] text-secondary">
                {id === "gemini"
                  ? "Gemini's free key is enough for everyday dictation."
                  : `Paste your ${label} API key.`}{" "}
                {KEY_URLS[id] && (
                  <button
                    type="button"
                    className="text-primary hover:underline"
                    onClick={() => BrowserOpenURL(KEY_URLS[id])}
                  >
                    {id === "gemini" ? "Get a free key" : "Get a key"}
                  </button>
                )}
              </p>
              <div className="flex gap-2">
                <input
                  type="password"
                  aria-label={`${label} API key`}
                  className="input flex-1 min-w-0"
                  placeholder={provider.key_set ? "Saved — paste a new key to replace it" : "Paste your key"}
                  value={keyDraft}
                  onChange={(e) => setKeyDraft(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && saveKey()}
                />
                {keyDraft.trim() ? (
                  <button type="button" className="btn btn-primary" onClick={saveKey}>
                    Save
                  </button>
                ) : (
                  provider.key_set && (
                    <button
                      type="button"
                      className="btn btn-secondary"
                      disabled={verify.state === "checking"}
                      onClick={() => runVerify()}
                    >
                      Verify
                    </button>
                  )
                )}
                {provider.key_set && !keyDraft && (
                  <button type="button" className="btn btn-ghost" onClick={removeKey}>
                    Remove
                  </button>
                )}
              </div>
              {verifyStatus && <div>{verifyStatus}</div>}
            </div>
          </SettingsSection>
        )
      )}

      <Disclosure summary="Advanced">
        <SettingsSection title="Service">
          <SettingRow
            label="Clean-up service"
            htmlFor="llm-provider"
            description={PROVIDERS.find((p) => p.id === id)?.sub}
          >
            <select
              id="llm-provider"
              className="select w-48"
              value={id}
              onChange={(e) => selectProvider(e.target.value)}
            >
              {PROVIDERS.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.label}
                </option>
              ))}
            </select>
          </SettingRow>
          {!provider?.local && (
            <SettingRow
              label="Model"
              htmlFor="llm-model"
              description={
                !canListModels
                  ? "Add a key to choose a model."
                  : modelsError
                    ? <StatusText tone="danger">Couldn't load models: {modelsError}</StatusText>
                    : undefined
              }
            >
              <select
                id="llm-model"
                className="select w-56"
                value={model}
                disabled={!canListModels || models.length === 0}
                onChange={(e) => selectModel(e.target.value)}
              >
                {!models.includes(model) && <option value={model}>{model}</option>}
                {models.map((m) => (
                  <option key={m} value={m}>
                    {m}
                  </option>
                ))}
              </select>
            </SettingRow>
          )}
        </SettingsSection>
      </Disclosure>
    </div>
  );
}
