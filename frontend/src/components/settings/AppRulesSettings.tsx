import { useCallback, useEffect, useState } from "react";
import {
  GetAppRules,
  GetFrontmostApp,
  RemoveAppRule,
  SetAppRule,
} from "../../../wailsjs/go/main/App";
import SettingsSection from "../ui/SettingsSection";
import { MODES } from "./PipelineSettings";

const INJECT_METHODS = [
  { id: "paste", name: "Pasting (⌘V)" },
  { id: "clipboard", name: "Copying to the clipboard only" },
  { id: "type", name: "Typing it out (for apps that block paste)" },
];

interface AppRuleRow {
  bundle_id: string;
  app_name?: string;
  refinement_mode?: string;
  inject_method?: string;
}

export default function AppRulesSettings() {
  const [rules, setRules] = useState<AppRuleRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [refinementMode, setRefinementMode] = useState("refine");
  const [injectMethod, setInjectMethod] = useState("paste");

  const loadRules = useCallback(async () => {
    setLoading(true);
    try {
      const list = await GetAppRules();
      setRules(list ?? []);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadRules();
  }, [loadRules]);

  const addRuleForFrontmost = async () => {
    setSaving(true);
    setError(null);
    try {
      const app = await GetFrontmostApp();
      if (!app?.bundle_id) {
        setError("Could not detect the active application.");
        return;
      }
      await SetAppRule(app.bundle_id, refinementMode, injectMethod);
      await loadRules();
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  const removeRule = async (bundleID: string) => {
    setSaving(true);
    try {
      await RemoveAppRule(bundleID);
      await loadRules();
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <SettingsSection
      title="Per-app rules"
      description="Change what happens after you speak in specific apps. Switch to the app, then add a rule."
    >
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-4">
        <div>
          <label className="label" htmlFor="rule-refinement">
            After you speak
          </label>
          <select
            id="rule-refinement"
            className="select"
            value={refinementMode}
            onChange={(e) => setRefinementMode(e.target.value)}
          >
            <option value="">Use my default</option>
            {MODES.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="label" htmlFor="rule-inject">
            Insert text by
          </label>
          <select
            id="rule-inject"
            className="select"
            value={injectMethod}
            onChange={(e) => setInjectMethod(e.target.value)}
          >
            {INJECT_METHODS.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
          </select>
        </div>
      </div>

      <button
        type="button"
        className="btn btn-primary w-full mb-4"
        disabled={saving}
        onClick={addRuleForFrontmost}
      >
        {saving ? "Saving…" : "Add rule for active app"}
      </button>

      {error && (
        <p className="text-sm text-[var(--danger)] mb-3">{error}</p>
      )}

      {loading ? (
        <p className="text-sm text-tertiary">Loading rules…</p>
      ) : rules.length === 0 ? (
        <p className="text-sm text-tertiary">No per-app rules yet.</p>
      ) : (
        <ul className="space-y-2">
          {rules.map((rule) => (
            <li
              key={rule.bundle_id}
              className="flex items-center justify-between gap-3 p-3 rounded-md border border-border bg-surface"
            >
              <div className="min-w-0">
                <p className="text-sm font-medium truncate text-text">
                  {rule.app_name || rule.bundle_id}
                </p>
                <p className="text-xs text-tertiary mt-0.5">
                  {MODES.find((m) => m.id === rule.refinement_mode)?.name ??
                    "Default"}{" "}
                  ·{" "}
                  {INJECT_METHODS.find(
                    (m) => m.id === (rule.inject_method || "paste"),
                  )?.name ?? rule.inject_method}
                </p>
              </div>
              <button
                type="button"
                className="btn btn-ghost text-[var(--danger)] shrink-0 !py-1.5 !px-2"
                disabled={saving}
                aria-label={`Remove rule for ${rule.app_name || rule.bundle_id}`}
                onClick={() => removeRule(rule.bundle_id)}
              >
                Remove
              </button>
            </li>
          ))}
        </ul>
      )}
    </SettingsSection>
  );
}
