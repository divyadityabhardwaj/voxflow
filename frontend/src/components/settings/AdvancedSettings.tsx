import { useEffect, useState } from "react";
import {
  CancelDownload,
  DeleteModelByName,
  DownloadModelByName,
  GetActiveDownload,
  GetAllModels,
  GetConfig,
  IsWhisperCLIReady,
  OpenLogFile,
  SetGroqKey,
  SetTranscription,
  SetWhisperLanguage,
  SetWhisperModel,
} from "../../../wailsjs/go/main/App";
import { main, whisper } from "../../../wailsjs/go/models";
import { BrowserOpenURL, EventsOn } from "../../../wailsjs/runtime/runtime";
import { Events } from "../../constants/events";
import { useToast } from "../../contexts/ToastContext";
import { LANGUAGES } from "../../lib/languages";
import ConfirmModal from "../ConfirmModal";
import SettingsSection, { Disclosure, SettingRow } from "../ui/SettingsSection";

// English-first picks, fastest to most accurate; the rest sit under a disclosure.
const NAMED_MODELS: Record<string, string> = {
  "tiny.en": "Fast",
  "base.en": "Balanced",
  "small.en": "Recommended",
  "large-v3-turbo-q5_0": "Most accurate",
};

const formatSize = (bytes: number) =>
  bytes >= 1024 ** 3
    ? `${(bytes / 1024 ** 3).toFixed(1)} GB`
    : `${Math.round(bytes / 1024 ** 2)} MB`;

export default function AdvancedSettings() {
  const { showToast } = useToast();
  const fail = (what: string) => (err: unknown) =>
    showToast(`${what}: ${String(err)}`, "error");

  const [config, setConfig] = useState<main.ConfigResponse | null>(null);
  const [models, setModels] = useState<whisper.ModelInfo[] | null>(null);
  const [engineReady, setEngineReady] = useState(true);
  const [downloading, setDownloading] = useState<string | null>(null);
  const [progress, setProgress] = useState(0);
  const [toDelete, setToDelete] = useState<string | null>(null);

  const loadModels = () =>
    GetAllModels()
      .then((m) => setModels(m ?? []))
      .catch(fail("Couldn't load speech models"));

  useEffect(() => {
    GetConfig().then(setConfig).catch(fail("Couldn't load settings"));
    loadModels();
    IsWhisperCLIReady().then(setEngineReady).catch(() => {});
    // The download runs in the backend and outlives this tab; pick it back up.
    GetActiveDownload()
      .then((d) => {
        if (d.model) {
          setDownloading(d.model);
          setProgress(Math.round(d.progress));
        }
      })
      .catch(() => {});

    const stop = () => {
      setDownloading(null);
      setProgress(0);
    };
    const offs = [
      EventsOn(Events.ModelDownloadProgress, (d: { model: string; progress: number }) => {
        setDownloading(d.model);
        setProgress(Math.round(d.progress));
      }),
      EventsOn(Events.ModelDownloadComplete, () => {
        stop();
        loadModels();
      }),
      EventsOn(Events.ModelDownloadError, stop),
      EventsOn(Events.ModelDownloadCancelled, stop),
    ];
    return () => offs.forEach((off) => off());
  }, []);

  if (!config || !models) {
    return <p className="text-sm text-tertiary">Loading…</p>;
  }

  const selectModel = (name: string) =>
    SetWhisperModel(name)
      .then(() => setConfig({ ...config, whisper_model: name } as main.ConfigResponse))
      .catch(fail("Couldn't switch speech models"));

  const download = (name: string) => {
    setDownloading(name);
    setProgress(0);
    DownloadModelByName(name).catch((err) => {
      setDownloading(null);
      fail("Couldn't download the model")(err);
    });
  };

  const confirmDelete = () => {
    const name = toDelete;
    setToDelete(null);
    if (name) DeleteModelByName(name).then(loadModels).catch(fail("Couldn't delete the model"));
  };

  const changeLanguage = (lang: string) =>
    SetWhisperLanguage(lang)
      .then(() => setConfig({ ...config, whisper_language: lang } as main.ConfigResponse))
      .catch(fail("Couldn't save the language"));

  const row = (m: whisper.ModelInfo) => {
    const active = config.whisper_model === m.name;
    const named = NAMED_MODELS[m.name];
    return (
      <div key={m.name} className="settings-row">
        <label className="flex items-start gap-3 min-w-0">
          <input
            type="radio"
            name="speech-model"
            checked={active}
            disabled={!m.downloaded}
            onChange={() => selectModel(m.name)}
            className="mt-0.5 w-4 h-4 shrink-0 accent-[var(--primary)]"
          />
          <span className="min-w-0">
            <span className="block text-[13px] text-text">
              {named ? `${named} (${m.name})` : m.name}
            </span>
            <span className="block text-xs text-secondary truncate">
              {m.description} · {formatSize(m.size)}
            </span>
          </span>
        </label>
        <div className="flex items-center gap-2 shrink-0">
          {m.downloaded ? (
            <>
              <span className="text-xs text-secondary">
                {active ? "In use" : "Downloaded"}
              </span>
              {!active && (
                <button
                  type="button"
                  className="btn btn-ghost !px-2 !py-1 !text-danger"
                  aria-label={`Delete ${m.name}`}
                  onClick={() => setToDelete(m.name)}
                >
                  Delete
                </button>
              )}
            </>
          ) : downloading === m.name ? (
            <>
              <div
                className="progress-bar w-20"
                role="progressbar"
                aria-label={`Downloading ${m.name}`}
                aria-valuenow={progress}
                aria-valuemin={0}
                aria-valuemax={100}
              >
                <div style={{ width: `${progress}%` }} />
              </div>
              <span className="text-xs text-secondary w-9 tabular-nums">{progress}%</span>
              <button
                type="button"
                className="btn btn-ghost !px-2 !py-1"
                onClick={() => CancelDownload().catch(fail("Couldn't cancel the download"))}
              >
                Cancel
              </button>
            </>
          ) : (
            <button
              type="button"
              className="btn btn-secondary"
              disabled={!!downloading}
              onClick={() => download(m.name)}
            >
              Download
            </button>
          )}
        </div>
      </div>
    );
  };

  const named = Object.keys(NAMED_MODELS)
    .map((n) => models.find((m) => m.name === n))
    .filter((m): m is whisper.ModelInfo => !!m);
  const others = models.filter((m) => !NAMED_MODELS[m.name]);

  return (
    <div className="space-y-6">
      {!engineReady && (
        <div className="alert alert-warning" role="alert">
          The speech engine is missing, so VoxFlow can't turn your voice into
          text yet. Open Terminal, run <code>brew install whisper-cpp</code>,
          then restart VoxFlow.
        </div>
      )}

      <TranscriptionSection config={config} setConfig={setConfig} />

      <SettingsSection
        title="Speech accuracy"
        description="For transcribing on this Mac. Bigger options are more accurate but slower and take more space."
      >
        {named.map(row)}
      </SettingsSection>

      {others.length > 0 && (
        <Disclosure summary="Other speech models (multilingual)">
          <SettingsSection title="Other models">{others.map(row)}</SettingsSection>
        </Disclosure>
      )}

      <SettingsSection title="Language">
        <SettingRow
          label="Language you speak"
          htmlFor="whisper-language"
          description="English models only understand English. Auto-detect needs a multilingual model and is a little slower."
        >
          <select
            id="whisper-language"
            className="select w-44"
            value={config.whisper_language || "en"}
            onChange={(e) => changeLanguage(e.target.value)}
          >
            {LANGUAGES.map(([code, name]) => (
              <option key={code} value={code}>
                {name}
              </option>
            ))}
          </select>
        </SettingRow>
      </SettingsSection>

      <SettingsSection title="Troubleshooting">
        <SettingRow label="Log file" description="Useful when reporting a problem.">
          <button
            type="button"
            className="btn btn-secondary"
            onClick={() => OpenLogFile().catch(fail("Couldn't open the log"))}
          >
            Open Log
          </button>
        </SettingRow>
      </SettingsSection>

      <ConfirmModal
        isOpen={!!toDelete}
        title="Delete this speech model?"
        message={`${toDelete} will be removed from this Mac. You can download it again later.`}
        confirmText="Delete"
        isDestructive
        onConfirm={confirmDelete}
        onCancel={() => setToDelete(null)}
      />
    </div>
  );
}

function TranscriptionSection({
  config,
  setConfig,
}: {
  config: main.ConfigResponse;
  setConfig: (c: main.ConfigResponse) => void;
}) {
  const { showToast } = useToast();
  const [key, setKey] = useState("");
  const [saving, setSaving] = useState(false);
  const [keyError, setKeyError] = useState<string | null>(null);
  const groq = config.transcription === "groq";

  const choose = (t: string) =>
    SetTranscription(t)
      .then(() => setConfig({ ...config, transcription: t } as main.ConfigResponse))
      .catch((err) => showToast(`Couldn't switch transcription: ${String(err)}`, "error"));

  const saveKey = async () => {
    if (!key.trim()) return;
    setSaving(true);
    setKeyError(null);
    try {
      await SetGroqKey(key.trim());
      setKey("");
      setConfig({ ...config, groq_api_key_set: true } as main.ConfigResponse);
      showToast("Groq key saved", "success");
    } catch (err) {
      setKeyError(String(err));
    } finally {
      setSaving(false);
    }
  };

  const option = (value: string, label: string, description: string) => (
    <div className="settings-row">
      <label className="flex items-start gap-3 min-w-0">
        <input
          type="radio"
          name="transcription"
          checked={config.transcription === value}
          onChange={() => choose(value)}
          className="mt-0.5 w-4 h-4 shrink-0 accent-[var(--primary)]"
        />
        <span className="min-w-0">
          <span className="block text-[13px] text-text">{label}</span>
          <span className="block text-xs text-secondary">{description}</span>
        </span>
      </label>
    </div>
  );

  return (
    <SettingsSection title="Transcription">
      {option("groq", "Groq (recommended)", "Fast and free with a Groq key. Your audio is sent to Groq.")}
      {option("local", "On this Mac", "Private and works offline. Needs a speech model download and uses about 0.5–1 GB of memory.")}
      {groq && (
        <SettingRow
          label="Groq API key"
          htmlFor="groq-key"
          description={
            <>
              {config.groq_api_key_set ? "A key is saved. Paste a new one to replace it. " : "Needed for Groq transcription. "}
              <button
                type="button"
                className="text-primary hover:underline"
                onClick={() => BrowserOpenURL("https://console.groq.com/keys")}
              >
                Get a free key
              </button>
              {keyError && (
                <span role="alert" className="block text-[var(--danger)] mt-1 break-words">
                  {keyError}
                </span>
              )}
            </>
          }
        >
          <input
            id="groq-key"
            type="password"
            className="input w-44"
            placeholder={config.groq_api_key_set ? "••••••••" : "gsk_…"}
            value={key}
            onChange={(e) => {
              setKey(e.target.value);
              setKeyError(null);
            }}
            onKeyDown={(e) => e.key === "Enter" && saveKey()}
          />
          <button type="button" className="btn btn-secondary" onClick={saveKey} disabled={!key.trim() || saving}>
            {saving ? "Checking…" : "Save"}
          </button>
        </SettingRow>
      )}
    </SettingsSection>
  );
}
