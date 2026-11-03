import { useState } from "react";
import type { DictationReadiness } from "../../hooks/useDictationReadiness";
import { useToast } from "../../contexts/ToastContext";

// One next action, rather than a list of settings to work through.
export default function SetupChecklist({ readiness, busy }: { readiness: DictationReadiness; busy: boolean }) {
  const { setup, error, fix } = readiness;
  const [fixing, setFixing] = useState(false);
  const { showToast } = useToast();
  if (!setup?.issue && !error) return null;
  const act = async () => {
    setFixing(true);
    try { await fix(); }
    catch (err) { showToast(`Couldn't open setup: ${String(err)}`, "error"); }
    finally { setFixing(false); }
  };
  return (
    <section aria-label="Dictation setup" className="card p-4 w-full flex items-center gap-4">
      <div className="flex-1 min-w-0">
        <h2 className="text-[13px] font-semibold text-text">
          {error ? "Couldn't check your setup" : setup?.issue?.title}
        </h2>
        <p id="setup-problem" className="text-xs text-secondary mt-1">
          {error ? "Try checking again before starting a dictation." : setup?.issue?.description}
        </p>
      </div>
      <button type="button" className="btn-secondary shrink-0" disabled={busy || fixing} onClick={act}>
        {fixing ? "Opening…" : error ? "Try again" : "Fix"}
      </button>
    </section>
  );
}
