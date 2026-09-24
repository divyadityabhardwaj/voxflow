import { useEffect, useState } from "react";
import { GetConfig, SetVocabulary } from "../../../wailsjs/go/main/App";
import { useToast } from "../../contexts/ToastContext";
import SettingsSection from "../ui/SettingsSection";

const splitWords = (v: string) =>
  v
    .split(/[,\n]/)
    .map((w) => w.trim())
    .filter(Boolean);

export default function DictionarySettings() {
  const { showToast } = useToast();
  const [words, setWords] = useState<string[] | null>(null);
  const [draft, setDraft] = useState("");

  useEffect(() => {
    GetConfig()
      .then((c) => setWords(splitWords(c.vocabulary || "")))
      .catch((err) => showToast(`Couldn't load your dictionary: ${String(err)}`, "error"));
  }, []);

  if (!words) {
    return <p className="text-sm text-tertiary">Loading…</p>;
  }

  const save = async (next: string[]) => {
    const previous = words;
    setWords(next);
    try {
      await SetVocabulary(next.join(", "));
    } catch (err) {
      setWords(previous);
      showToast(`Couldn't save your dictionary: ${String(err)}`, "error");
    }
  };

  const add = () => {
    const fresh = splitWords(draft).filter(
      (w) => !words.some((x) => x.toLowerCase() === w.toLowerCase()),
    );
    setDraft("");
    if (fresh.length) save([...words, ...fresh]);
  };

  return (
    <SettingsSection
      title="Dictionary"
      description="Names, brands and jargon VoxFlow should always spell your way. Adding the words you use measurably improves accuracy."
    >
      <div className="settings-pad space-y-3">
        <div className="flex gap-2">
          <input
            aria-label="Add a word"
            className="input flex-1 min-w-0"
            placeholder="Add a name, brand or term — e.g. Priya, Figma, OKR"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                add();
              }
            }}
          />
          <button
            type="button"
            className="btn btn-primary"
            disabled={!draft.trim()}
            onClick={add}
          >
            Add
          </button>
        </div>
        {words.length === 0 ? (
          <p className="text-[13px] text-tertiary">No words yet.</p>
        ) : (
          <ul className="flex flex-wrap gap-1.5" aria-label="Your words">
            {words.map((w) => (
              <li key={w} className="chip">
                {w}
                <button
                  type="button"
                  className="w-5 h-5 inline-flex items-center justify-center rounded-full text-secondary hover:text-text hover:bg-surface-hover"
                  aria-label={`Remove ${w}`}
                  onClick={() => save(words.filter((x) => x !== w))}
                >
                  <svg className="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.5}>
                    <path strokeLinecap="round" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </SettingsSection>
  );
}
