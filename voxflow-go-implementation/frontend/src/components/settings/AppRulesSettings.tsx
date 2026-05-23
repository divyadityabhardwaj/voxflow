import { useCallback, useEffect, useState } from "react";
import {
  GetAppRules,
  GetFrontmostApp,
  RemoveAppRule,
  SetAppRule,
} from "../../../wailsjs/go/main/App";
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
    <section className="card p-6 brutal-card mt-6">
      <h3 className="font-black text-xl uppercase tracking-tighter text-primary mb-2">
        Per-app rules
      </h3>
      <p className="text-sm text-tertiary mb-6 font-bold">
        Override refinement or injection for specific apps. Focus the target app,
        then add a rule below.
      </p>

      <div className="grid grid-cols-2 gap-3 mb-4">
        <div>
          <label className="block text-xs font-black uppercase text-tertiary mb-1">
            Refinement
          </label>
          <select
            className="input w-full"
            value={refinementMode}
            onChange={(e) => setRefinementMode(e.target.value)}
          >
            <option value="">Use global default</option>
            <option value="refine">Refine</option>
            <option value="raw">Raw</option>
            <option value="copy-only">Copy only</option>
          </select>
        </div>
        <div>
          <label className="block text-xs font-black uppercase text-tertiary mb-1">
            Injection
          </label>
          <select
            className="input w-full"
            value={injectMethod}
            onChange={(e) => setInjectMethod(e.target.value)}
          >
            <option value="paste">Paste (Cmd+V)</option>
            <option value="clipboard">Clipboard only</option>
          </select>
        </div>
      </div>

      <button
        type="button"
        className="btn-primary w-full mb-4"
        disabled={saving}
        onClick={addRuleForFrontmost}
      >
        {saving ? "Saving…" : "Add rule for active app"}
      </button>

      {error && (
        <p className="text-xs text-red-500 font-bold mb-3">{error}</p>
      )}

      {loading ? (
        <p className="text-xs text-tertiary font-bold">Loading rules…</p>
      ) : rules.length === 0 ? (
        <p className="text-xs text-tertiary font-bold">No per-app rules yet.</p>
      ) : (
        <ul className="space-y-2">
          {rules.map((rule) => (
            <li
              key={rule.bundle_id}
              className="flex items-center justify-between gap-3 p-3 border-2 border-border rounded-lg bg-secondary"
            >
              <div className="min-w-0">
                <p className="text-sm font-bold truncate">{rule.bundle_id}</p>
                <p className="text-[10px] text-tertiary font-bold">
                  {rule.refinement_mode || "default"} ·{" "}
                  {rule.inject_method || "paste"}
                </p>
              </div>
              <button
                type="button"
                className="text-xs font-bold text-red-500 shrink-0"
                disabled={saving}
                onClick={() => removeRule(rule.bundle_id)}
              >
                Remove
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
