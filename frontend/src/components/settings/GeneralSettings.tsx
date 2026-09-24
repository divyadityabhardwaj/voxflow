import { useEffect, useRef, useState } from "react";
import {
  GetConfig,
  GetInputDevices,
  GetPermissions,
  GetPushToTalkKey,
  OpenPrivacySettings,
  RequestMicrophoneAccess,
  SetHandsFreeHotkey,
  SetInputDevice,
  SetMuteSystemAudio,
  SetPushToTalkHotkey,
  SetPushToTalkKey,
  SetRefinementMode,
} from "../../../wailsjs/go/main/App";
import { audio, main } from "../../../wailsjs/go/models";
import { useToast } from "../../contexts/ToastContext";
import { formatShortcut } from "../../lib/shortcut";
import { MODES } from "../../lib/modes";
import HotkeyRecorderModal from "../HotkeyRecorderModal";
import SettingsSection, { SettingRow, StatusText, Toggle } from "../ui/SettingsSection";

const HOLD_KEYS = [
  { id: "right_option", label: "Right ⌥ Option" },
  { id: "right_command", label: "Right ⌘ Command" },
  { id: "fn", label: "🌐 Fn" },
  { id: "chord", label: "Key combination" },
];

type ShortcutField = "ptt" | "handsFree";

export default function GeneralSettings() {
  const { showToast } = useToast();
  const fail = (what: string) => (err: unknown) =>
    showToast(`${what}: ${String(err)}`, "error");

  const [config, setConfig] = useState<main.ConfigResponse | null>(null);
  const [ptt, setPtt] = useState<main.PushToTalkKeyInfo | null>(null);
  const [devices, setDevices] = useState<audio.InputDevice[]>([]);
  const [perms, setPerms] = useState<main.Permissions | null>(null);
  const [recording, setRecording] = useState<ShortcutField | null>(null);
  const pttRef = useRef(ptt);
  pttRef.current = ptt;

  useEffect(() => {
    GetConfig().then(setConfig).catch(fail("Couldn't load settings"));
    GetPushToTalkKey().then(setPtt).catch(fail("Couldn't read the hold-to-talk key"));
    GetInputDevices()
      .then((d) => setDevices(d ?? []))
      .catch(fail("Couldn't list microphones"));
  }, []);

  // Permissions change in System Settings, so poll while this tab is open.
  useEffect(() => {
    let lastAX: boolean | undefined;
    const check = () =>
      GetPermissions()
        .then((p) => {
          setPerms(p);
          const key = pttRef.current?.key;
          // The hold key's event tap only starts once Accessibility is on.
          if (lastAX === false && p.accessibility && key) {
            SetPushToTalkKey(key).then(setPtt).catch(() => {});
          }
          lastAX = p.accessibility;
        })
        .catch(() => {});
    check();
    const id = setInterval(check, 2000);
    window.addEventListener("focus", check);
    return () => {
      clearInterval(id);
      window.removeEventListener("focus", check);
    };
  }, []);

  if (!config) {
    return <p className="text-sm text-tertiary">Loading…</p>;
  }

  const update = (patch: Partial<main.ConfigResponse>) =>
    setConfig((c) => (c ? ({ ...c, ...patch } as main.ConfigResponse) : c));

  const changeHoldKey = (key: string) =>
    SetPushToTalkKey(key)
      .then(setPtt)
      .catch(fail("Couldn't change the hold-to-talk key"));

  const changeDevice = (name: string) =>
    SetInputDevice(name)
      .then(() => update({ input_device: name }))
      .catch(fail("Couldn't switch microphones"));

  const changeMode = (mode: string) =>
    SetRefinementMode(mode)
      .then(() => update({ refinement_mode: mode }))
      .catch(fail("Couldn't save what happens after you speak"));

  const changeMute = (on: boolean) =>
    SetMuteSystemAudio(on)
      .then(() => update({ mute_system_audio: on }))
      .catch(fail("Couldn't save the mute setting"));

  // Throws a user-facing error so the recorder stays open and shows it inline.
  const saveShortcut = async (value: string) => {
    const isPtt = recording === "ptt";
    try {
      await (isPtt ? SetPushToTalkHotkey : SetHandsFreeHotkey)(value);
    } catch (err) {
      const msg = String(err);
      throw new Error(
        msg.includes("failed to register")
          ? "That shortcut is taken by another app. Pick another."
          : `Couldn't save that shortcut: ${msg}`,
      );
    }
    update(isPtt ? { push_to_talk_hotkey: value } : { hands_free_hotkey: value });
  };

  const holdKey = ptt?.key ?? "chord";
  const chord = formatShortcut(config.push_to_talk_hotkey);
  const holdHint =
    holdKey !== "chord" && ptt && !ptt.active
      ? `Needs Accessibility (see Permissions below). Until then, hold ${chord}.`
      : holdKey === "fn"
        ? "In System Settings › Keyboard, set “Press 🌐 key to” to “Do Nothing”, or it also opens the emoji picker."
        : "Hold while you speak; let go to paste.";

  const defaultDevice = devices.find((d) => d.default)?.name;
  const savedMissing =
    config.input_device && !devices.some((d) => d.name === config.input_device);
  const mode = config.refinement_mode || "refine";

  return (
    <div className="space-y-6">
      <SettingsSection title="Dictation shortcuts">
        <SettingRow label="Hold to talk" htmlFor="hold-key" description={holdHint}>
          <select
            id="hold-key"
            className="select w-auto"
            value={holdKey}
            onChange={(e) => changeHoldKey(e.target.value)}
          >
            {HOLD_KEYS.map((k) => (
              <option key={k.id} value={k.id}>
                {k.label}
              </option>
            ))}
          </select>
        </SettingRow>
        {holdKey === "chord" && (
          <ShortcutRow
            label="Key combination"
            description="Hold these keys while you speak."
            value={config.push_to_talk_hotkey}
            onChange={() => setRecording("ptt")}
          />
        )}
        <ShortcutRow
          label="Start/stop"
          description="Press once to start, again to finish."
          value={config.hands_free_hotkey}
          onChange={() => setRecording("handsFree")}
        />
      </SettingsSection>

      <SettingsSection title="Microphone">
        <SettingRow label="Input" htmlFor="input-device">
          <select
            id="input-device"
            className="select w-56"
            value={config.input_device ?? ""}
            onChange={(e) => changeDevice(e.target.value)}
          >
            <option value="">
              System default{defaultDevice ? ` (${defaultDevice})` : ""}
            </option>
            {devices.map((d) => (
              <option key={d.name} value={d.name}>
                {d.name}
              </option>
            ))}
            {savedMissing && (
              <option value={config.input_device}>
                {config.input_device} (not connected)
              </option>
            )}
          </select>
        </SettingRow>
      </SettingsSection>

      <SettingsSection title="After you speak">
        {MODES.map((m) => (
          <label key={m.id} className="flex items-start gap-3 settings-pad">
            <input
              type="radio"
              name="after-you-speak"
              value={m.id}
              checked={mode === m.id}
              onChange={() => changeMode(m.id)}
              className="mt-0.5 w-4 h-4 shrink-0 accent-[var(--primary)]"
            />
            <span>
              <span className="block text-[13px] text-text">{m.name}</span>
              <span className="block text-xs text-secondary mt-0.5">{m.desc}</span>
            </span>
          </label>
        ))}
      </SettingsSection>

      <SettingsSection title="Sound">
        <SettingRow
          label={<span id="mute-label">Mute sound while I talk</span>}
          description={
            <span id="mute-desc">
              So music and calls don't end up in your text. The volume comes back
              when you stop.
            </span>
          }
        >
          <Toggle
            checked={config.mute_system_audio}
            onChange={changeMute}
            labelledBy="mute-label"
            describedBy="mute-desc"
          />
        </SettingRow>
      </SettingsSection>

      <SettingsSection title="Permissions">
        <SettingRow
          label="Microphone"
          description={
            perms?.microphone === "authorized" ? (
              <StatusText tone="success">Allowed</StatusText>
            ) : perms?.microphone === "notDetermined" ? (
              <StatusText tone="muted">Not asked yet</StatusText>
            ) : perms ? (
              <StatusText tone="danger">Off — VoxFlow can't hear you</StatusText>
            ) : null
          }
        >
          {perms?.microphone === "notDetermined" ? (
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => RequestMicrophoneAccess().catch(() => {})}
            >
              Allow…
            </button>
          ) : (
            perms?.microphone !== "authorized" && (
              <button
                type="button"
                className="btn btn-secondary"
                onClick={() =>
                  OpenPrivacySettings("microphone").catch(fail("Couldn't open System Settings"))
                }
              >
                Open Settings
              </button>
            )
          )}
        </SettingRow>
        <SettingRow
          label="Accessibility"
          description={
            perms?.accessibility ? (
              <StatusText tone="success">Allowed</StatusText>
            ) : perms ? (
              <StatusText tone="danger">
                Off — needed to paste for you and for hold-to-talk
              </StatusText>
            ) : null
          }
        >
          {perms && !perms.accessibility && (
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() =>
                OpenPrivacySettings("accessibility").catch(fail("Couldn't open System Settings"))
              }
            >
              Open Settings
            </button>
          )}
        </SettingRow>
      </SettingsSection>

      <HotkeyRecorderModal
        isOpen={recording !== null}
        onClose={() => setRecording(null)}
        onSave={saveShortcut}
        initialValue={
          recording === "ptt" ? config.push_to_talk_hotkey : config.hands_free_hotkey
        }
        otherHotkey={
          recording === "ptt" ? config.hands_free_hotkey : config.push_to_talk_hotkey
        }
      />
    </div>
  );
}

function ShortcutRow({
  label,
  description,
  value,
  onChange,
}: {
  label: string;
  description: string;
  value: string;
  onChange: () => void;
}) {
  const shown = value ? formatShortcut(value) : "Not set";
  return (
    <SettingRow label={label} description={description}>
      <kbd className="keycap">{shown}</kbd>
      <button
        type="button"
        className="btn btn-secondary"
        aria-label={`Change ${label} shortcut, currently ${shown}`}
        onClick={onChange}
      >
        Change…
      </button>
    </SettingRow>
  );
}
