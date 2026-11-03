import { useEffect, useState, type Dispatch, type SetStateAction } from "react";
import {
  CancelDownload,
  DeleteModelByName,
  DownloadModelByName,
  GetActiveDownload,
  GetAllModels,
  GetConfig,
  IsWhisperCLIReady,
  GetInputDevices,
  SetInputDevice,
  SetGroqKey,
  SetTranscription,
  SetWhisperLanguage,
  SetWhisperModel,
} from "../../../wailsjs/go/main/App";
import { audio, main, whisper } from "../../../wailsjs/go/models";
import { BrowserOpenURL, EventsOn } from "../../../wailsjs/runtime/runtime";
import { Events } from "../../constants/events";
import { useToast } from "../../contexts/ToastContext";
import { LANGUAGES } from "../../lib/languages";
import { useRecordingState } from "../../hooks/useRecordingState";
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

export default function SpeechSettings() {
  const { showToast } = useToast();
  const status = useRecordingState();
  const fail = (what: string) => (err: unknown) =>
    showToast(`${what}: ${String(err)}`, "error");

  const [config, setConfig] = useState<main.ConfigResponse | null>(null);
  const [models, setModels] = useState<whisper.ModelInfo[] | null>(null);
  const [devices, setDevices] = useState<audio.InputDevice[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [engineReady, setEngineReady] = useState(true);
  const [downloading, setDownloading] = useState<string | null>(null);
  const [progress, setProgress] = useState(0);
  const [toDelete, setToDelete] = useState<string | null>(null);

  const loadModels = () =>
    GetAllModels()
      .then((m) => setModels(m ?? []))
      .catch(fail("Couldn't load speech models"));

  useEffect(() => {
    GetConfig().then(setConfig).catch((err) => setLoadError(String(err)));
    const refreshDevices = () => GetInputDevices().then((d) => setDevices(d ?? [])).catch(fail("Couldn't list microphones"));
    refreshDevices();
    window.addEventListener("focus", refreshDevices);
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
    return () => { offs.forEach((off) => off()); window.removeEventListener("focus", refreshDevices); };
  }, []);

  if (loadError) return <p role="alert" className="text-sm text-danger">Couldn't load speech settings. Reopen this tab to try again.</p>;

  if (!config) {
    return <p className="text-sm text-tertiary">Loading…</p>;
  }

  const selectModel = (name: string) =>
    SetWhisperModel(name)
      .then(() => setConfig((current) => current ? ({ ...current, whisper_model: name } as main.ConfigResponse) : current))
      .catch(fail("Couldn't switch speech models"));

  const download = (name: string) => {
    setDownloading(name);
    setProgress(0);
    DownloadModelByName(name).then(async () => {
      const current = await GetConfig();
      if (current.transcription === "local" && current.whisper_model === name) {
        await SetWhisperModel(name);
      }
      loadModels();
    }).catch((err) => {
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
      .then(() => setConfig((current) => current ? ({ ...current, whisper_language: lang } as main.ConfigResponse) : current))
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
    .map((n) => (models ?? []).find((m) => m.name === n))
    .filter((m): m is whisper.ModelInfo => !!m);
  const others = (models ?? []).filter((m) => !NAMED_MODELS[m.name]);

  const local = config.transcription !== "groq";
  const defaultDevice = devices.find((d) => d.default)?.name;
  const savedMissing = config.input_device && !devices.some((d) => d.name === config.input_device);
  const changeDevice = (name: string) => SetInputDevice(name)
    .then(() => setConfig((current) => current ? ({ ...current, input_device: name } as main.ConfigResponse) : current))
    .catch(fail("Couldn't switch microphones"));

  return (
    <fieldset disabled={status !== "Idle"} className="space-y-6 min-w-0">
      {status !== "Idle" && <p role="status" className="text-sm text-secondary">Finish or cancel your dictation to change speech settings.</p>}
      <TranscriptionSection config={config} setConfig={setConfig} />

      <SettingsSection title="Microphone">
        <SettingRow label="Input" htmlFor="input-device">
          <select id="input-device" className="select w-56" value={config.input_device ?? ""} onChange={(e) => changeDevice(e.target.value)}>
            <option value="">System default{defaultDevice ? ` (${defaultDevice})` : ""}</option>
            {devices.map((d) => <option key={d.name} value={d.name}>{d.name}</option>)}
            {savedMissing && <option value={config.input_device}>{config.input_device} (not connected)</option>}
          </select>
        </SettingRow>
      </SettingsSection>

      {local && !engineReady && (
        <div className="alert alert-warning" role="alert">
          The speech engine is missing, so VoxFlow can't turn your voice into
          text on this Mac yet. Reinstall VoxFlow to restore the speech engine,
          or choose Groq above.
        </div>
      )}

      {local && <>
      <SettingsSection
        title="Speech accuracy"
        description="For transcribing on this Mac. Bigger options are more accurate but slower and take more space."
      >
        {!models && <p className="settings-pad text-sm text-secondary">Loading speech models…</p>}
        {named.map(row)}
      </SettingsSection>

      {others.length > 0 && (
        <Disclosure summary="Other speech models (multilingual)">
          <SettingsSection title="Other models">{others.map(row)}</SettingsSection>
        </Disclosure>
      )}

      </>}

      <SettingsSection title="Language">
        <SettingRow
          label="Language you speak"
          htmlFor="whisper-language"
          description={local ? "Choose a multilingual model for languages other than English or auto-detect." : "Choose the language you speak, or let Groq detect it."}
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

      <ConfirmModal
        isOpen={!!toDelete}
        title="Delete this speech model?"
        message={`${toDelete} will be removed from this Mac. You can download it again later.`}
        confirmText="Delete"
        isDestructive
        onConfirm={confirmDelete}
        onCancel={() => setToDelete(null)}
      />
    </fieldset>
  );
}

function TranscriptionSection({
  config,
  setConfig,
}: {
  config: main.ConfigResponse;
  setConfig: Dispatch<SetStateAction<main.ConfigResponse | null>>;
}) {
  const { showToast } = useToast();
  const [key, setKey] = useState("");
  const [switching, setSwitching] = useState(false);
  const [saving, setSaving] = useState(false);
  const [keyError, setKeyError] = useState<string | null>(null);
  const groq = config.transcription === "groq";

  const choose = (t: string) => {
    setSwitching(true);
    SetTranscription(t)
      .then(() => setConfig((current) => current ? ({ ...current, transcription: t } as main.ConfigResponse) : current))
      .catch((err) => showToast(`Couldn't switch transcription: ${String(err)}`, "error"))
      .finally(() => setSwitching(false));
  };

  const saveKey = async () => {
    if (!key.trim()) return;
    setSaving(true);
    setKeyError(null);
    try {
      await SetGroqKey(key.trim());
      setKey("");
      setConfig((current) => current ? ({ ...current, groq_api_key_set: true } as main.ConfigResponse) : current);
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
          disabled={switching}
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
    <SettingsSection title="Transcription" description="This controls where your audio is transcribed. AI clean-up has separate settings and may send text to its provider.">
      {option("groq", "Groq (recommended)", "Your audio is sent to Groq for transcription. Requires internet and a Groq API key.")}
      {option("local", "On this Mac", "Audio stays on your Mac and transcription works offline. Requires a speech model download.")}
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
