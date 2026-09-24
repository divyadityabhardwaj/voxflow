import { useCallback, useEffect, useState } from "react";
import {
  GetAppRules,
  GetFrontmostApp,
  RemoveAppRule,
  SetAppRule,
} from "../../../wailsjs/go/main/App";
import { main } from "../../../wailsjs/go/models";
import { useToast } from "../../contexts/ToastContext";
import { MODES } from "../../lib/modes";
import SettingsSection from "../ui/SettingsSection";

const INJECT_METHODS = [
  { id: "paste", name: "Pasting (⌘V)" },
  { id: "type", name: "Typing it out (for apps that block paste)" },
  { id: "clipboard", name: "Copying to the clipboard only" },
];

export default function AppRulesSettings() {
  const { showToast } = useToast();
  const fail = (what: string) => (err: unknown) =>
    showToast(`${what}: ${String(err)}`, "error");

  const [rules, setRules] = useState<main.AppRuleDTO[] | null>(null);
  const [current, setCurrent] = useState<main.FrontmostAppInfo | null>(null);

  const loadRules = useCallback(
    () =>
      GetAppRules()
        .then((r) => {
          const nameOf = (x: main.AppRuleDTO) => x.app_name || x.bundle_id;
          // Rules come from a Go map, so their order changes on every load.
          setRules((r ?? []).sort((a, b) => nameOf(a).localeCompare(nameOf(b))));
        })
        .catch(fail("Couldn't load app rules")),
    [],
  );

  // The backend reports the last app used before VoxFlow.
  useEffect(() => {
    loadRules();
    const refresh = () => GetFrontmostApp().then(setCurrent).catch(() => setCurrent(null));
    refresh();
    window.addEventListener("focus", refresh);
    return () => window.removeEventListener("focus", refresh);
  }, [loadRules]);

  const save = (bundleID: string, mode: string, inject: string) =>
    SetAppRule(bundleID, mode, inject)
      .then(loadRules)
      .catch(fail("Couldn't save the rule"));

  const remove = (bundleID: string) =>
    RemoveAppRule(bundleID)
      .then(loadRules)
      .catch(fail("Couldn't remove the rule"));

  const hasCurrent =
    !!current?.bundle_id && !rules?.some((r) => r.bundle_id === current.bundle_id);

  return (
    <SettingsSection
      title="Apps"
      description="Choose how VoxFlow behaves in specific apps."
    >
      <div className="settings-row">
        <p className="text-[13px] text-secondary">
          {current?.name
            ? `Last app you used: ${current.name}`
            : "Switch to an app, then come back here to add it."}
        </p>
        <button
          type="button"
          className="btn btn-secondary shrink-0"
          disabled={!hasCurrent}
          onClick={() => current && save(current.bundle_id, "", "paste")}
        >
          {current?.name ? `Add ${current.name}` : "Add current app"}
        </button>
      </div>

      {rules === null ? (
        <p className="settings-pad text-[13px] text-tertiary">Loading…</p>
      ) : rules.length === 0 ? (
        <p className="settings-pad text-[13px] text-tertiary">
          No app rules yet. Every app uses your General settings.
        </p>
      ) : (
        rules.map((rule) => {
          const name = rule.app_name || rule.bundle_id;
          const mode = rule.refinement_mode ?? "";
          const inject = rule.inject_method || "paste";
          return (
            <div key={rule.bundle_id} className="settings-pad space-y-2">
              <div className="flex items-center justify-between gap-3">
                <p className="text-[13px] font-medium text-text truncate" title={rule.bundle_id}>
                  {name}
                </p>
                <button
                  type="button"
                  className="btn btn-ghost !px-2 !py-1 !text-danger"
                  aria-label={`Remove rule for ${name}`}
                  onClick={() => remove(rule.bundle_id)}
                >
                  Remove
                </button>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                <label className="text-xs text-secondary">
                  After you speak
                  <select
                    className="select mt-1"
                    value={mode}
                    onChange={(e) => save(rule.bundle_id, e.target.value, inject)}
                  >
                    <option value="">Use my default</option>
                    {MODES.map((m) => (
                      <option key={m.id} value={m.id}>
                        {m.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="text-xs text-secondary">
                  Insert text by
                  <select
                    className="select mt-1"
                    value={inject}
                    onChange={(e) => save(rule.bundle_id, mode, e.target.value)}
                  >
                    {INJECT_METHODS.map((m) => (
                      <option key={m.id} value={m.id}>
                        {m.name}
                      </option>
                    ))}
                  </select>
                </label>
              </div>
            </div>
          );
        })
      )}
    </SettingsSection>
  );
}
