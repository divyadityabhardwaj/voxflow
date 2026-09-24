import { useState, useEffect, useRef, useCallback, memo, type KeyboardEvent } from "react";
import {
  DeleteTranscript,
  ClearAllHistory,
  GetHistory,
  GetHistoryPage,
  SearchHistoryPage,
  RetryRefinement,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { useConfirmModal } from "./ConfirmModal";
import { useToast } from "../contexts/ToastContext";
import { Events } from "../constants/events";
import { pasteLabel, usePasteTarget, useTextActions } from "../hooks/useDictation";
import VersionToggle, { type Version } from "./dictation/VersionToggle";
import { clockTime, dateGroup } from "./dictation/time";

interface Transcript {
  id: number;
  timestamp: string;
  app_name: string;
  raw_text: string;
  polished_text: string;
  mode: string;
  llm_provider?: string;
  llm_model?: string;
  translation_time_ms?: number;
  tokens_per_second?: number;
  words_per_second?: number;
}

const escapeRegExp = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

const highlightText = (text: string, highlight: string) => {
  if (!highlight.trim()) return <span>{text}</span>;
  const parts = text.split(new RegExp(`(${escapeRegExp(highlight)})`, "gi"));
  return (
    <span>
      {parts.map((part, i) =>
        part.toLowerCase() === highlight.toLowerCase() ? (
          <mark key={i} className="bg-accent-soft text-primary px-0.5 rounded font-medium">
            {part}
          </mark>
        ) : (
          part
        ),
      )}
    </span>
  );
};

const fullDate = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" });
const formatFullDate = (timestamp: string) => {
  const d = new Date(timestamp);
  return isNaN(d.getTime()) ? timestamp : fullDate.format(d);
};

const UNDO_MS = 6000;
// ponytail: counting by fetching; add a CountHistory binding if histories get huge.
const COUNT_LIMIT = 10000;

interface HistoryItemProps {
  transcript: Transcript;
  isSelected: boolean;
  tabbable: boolean;
  searchQuery: string;
  onSelect: (id: number) => void;
}

const HistoryItem = memo(function HistoryItem({ transcript, isSelected, tabbable, searchQuery, onSelect }: HistoryItemProps) {
  return (
    <button
      type="button"
      data-id={transcript.id}
      onClick={() => onSelect(transcript.id)}
      aria-current={isSelected ? "true" : undefined}
      tabIndex={tabbable ? 0 : -1}
      className={`w-full px-4 py-2.5 text-left transition-colors border-l-2 ${
        isSelected ? "bg-accent-soft border-l-primary" : "hover:bg-secondary border-l-transparent"
      }`}
    >
      <p className="text-xs text-tertiary mb-0.5 truncate">
        {[transcript.app_name, clockTime(transcript.timestamp)].filter(Boolean).join(" · ")}
      </p>
      <p className="text-[13px] text-text line-clamp-2">
        {highlightText(transcript.polished_text || transcript.raw_text, searchQuery)}
      </p>
    </button>
  );
});

const PAGE_SIZE = 50;

// A dictation can finish while a page is in flight, so pages and refreshes overlap.
const withoutKnown = (prev: Transcript[], items: Transcript[]) => {
  const known = new Set(prev.map((t) => t.id));
  return items.filter((t) => !known.has(t.id));
};

interface PageState {
  query: string;
  cursorTS: string;
  cursorID: number;
  loading: boolean;
  hasMore: boolean;
}

const freshPage = (query: string): PageState => ({
  query,
  cursorTS: "",
  cursorID: 0,
  loading: false,
  hasMore: true,
});

const TRASH_PATH =
  "M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16";

export default function HistoryView() {
  const [transcripts, setTranscripts] = useState<Transcript[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [loading, setLoading] = useState<"first" | "more" | null>("first");
  // Ref + object identity so stale in-flight pages drop after query change.
  const page = useRef<PageState>(freshPage(""));
  const searchRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const [version, setVersion] = useState<Version>("cleaned");
  const [instruction, setInstruction] = useState("");
  const [rewriting, setRewriting] = useState(false);

  // Deleted rows hide at once and are removed for real when Undo times out.
  const [hidden, setHidden] = useState<Set<number>>(new Set());
  const pendingDeletes = useRef(new Map<number, ReturnType<typeof setTimeout>>());

  const { confirm, ConfirmModalComponent } = useConfirmModal();
  const { showToast } = useToast();
  const pasteApp = usePasteTarget();
  const { copy, paste } = useTextActions(pasteApp);

  const loadNextPage = useCallback(async () => {
    const p = page.current;
    if (p.loading || !p.hasMore) return;
    p.loading = true;
    setLoading(p.cursorID === 0 ? "first" : "more");
    try {
      const res = p.query
        ? await SearchHistoryPage(p.query, p.cursorTS, p.cursorID, PAGE_SIZE)
        : await GetHistoryPage(p.cursorTS, p.cursorID, PAGE_SIZE);
      if (page.current !== p) return;
      const items: Transcript[] = res.transcripts || [];
      setTranscripts((prev) => [...prev, ...withoutKnown(prev, items)]);
      p.cursorTS = res.next_cursor_ts || "";
      p.cursorID = res.next_cursor_id || 0;
      p.hasMore = items.length === PAGE_SIZE;
    } catch (err) {
      console.error("Failed to load history:", err);
    } finally {
      p.loading = false;
      if (page.current === p) setLoading(null);
    }
  }, []);

  useEffect(() => {
    page.current = freshPage(searchQuery);
    setTranscripts([]);
    setLoading("first");
    const t = setTimeout(loadNextPage, 200);
    return () => clearTimeout(t);
  }, [searchQuery, loadNextPage]);

  useEffect(
    () =>
      EventsOn(Events.ProcessingComplete, async () => {
        if (page.current.query) return;
        try {
          const res = await GetHistoryPage("", 0, PAGE_SIZE);
          const items: Transcript[] = res.transcripts || [];
          setTranscripts((prev) => [...withoutKnown(prev, items), ...prev]);
        } catch (err) {
          console.error("Failed to refresh history:", err);
        }
      }),
    [],
  );

  useEffect(() => {
    const onKey = (e: globalThis.KeyboardEvent) => {
      if (e.metaKey && e.key.toLowerCase() === "f") {
        e.preventDefault();
        searchRef.current?.focus();
        searchRef.current?.select();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  // Leaving the view deletes right away whatever is still waiting for Undo.
  useEffect(() => {
    const pending = pendingDeletes.current;
    return () => {
      pending.forEach((timer, id) => {
        clearTimeout(timer);
        DeleteTranscript(id).catch(() => {});
      });
    };
  }, []);

  // Callback ref: sentinel mounts after first list paint; plain ref+effect misses it.
  const observer = useRef<IntersectionObserver | null>(null);
  const sentinelRef = useCallback(
    (el: HTMLDivElement | null) => {
      observer.current?.disconnect();
      if (!el) return;
      observer.current = new IntersectionObserver(([entry]) => {
        if (entry.isIntersecting) loadNextPage();
      });
      observer.current.observe(el);
    },
    [loadNextPage],
  );

  const visible = transcripts.filter((t) => !hidden.has(t.id));
  const selected = visible.find((t) => t.id === selectedId);

  const select = useCallback((id: number) => {
    setSelectedId(id);
    setVersion("cleaned");
    setInstruction("");
  }, []);

  const handleListKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const step = e.key === "ArrowDown" ? 1 : e.key === "ArrowUp" ? -1 : 0;
    if (!step || visible.length === 0) return;
    e.preventDefault();
    const i = visible.findIndex((t) => t.id === selectedId);
    const next = visible[Math.max(0, Math.min(visible.length - 1, i < 0 ? 0 : i + step))];
    select(next.id);
    listRef.current?.querySelector<HTMLElement>(`[data-id="${next.id}"]`)?.focus();
  };

  const unhide = (id: number) =>
    setHidden((prev) => {
      const s = new Set(prev);
      s.delete(id);
      return s;
    });

  const handleDelete = (t: Transcript) => {
    const i = visible.findIndex((x) => x.id === t.id);
    const neighbour = visible[i + 1] ?? visible[i - 1];
    setHidden((prev) => new Set(prev).add(t.id));
    if (selectedId === t.id) setSelectedId(neighbour?.id ?? null);

    const commit = () => {
      pendingDeletes.current.delete(t.id);
      DeleteTranscript(t.id)
        .then(() => setTranscripts((prev) => prev.filter((x) => x.id !== t.id)))
        .catch(() => {
          unhide(t.id);
          showToast("Couldn't delete that dictation", "error");
        });
    };
    pendingDeletes.current.set(t.id, setTimeout(commit, UNDO_MS));

    const preview = (t.polished_text || t.raw_text).slice(0, 40);
    showToast(`Deleted “${preview}${preview.length === 40 ? "…" : ""}”`, "info", {
      ttl: UNDO_MS,
      action: {
        label: "Undo",
        onClick: () => {
          clearTimeout(pendingDeletes.current.get(t.id));
          pendingDeletes.current.delete(t.id);
          unhide(t.id);
        },
      },
    });
  };

  const handleClearAll = async () => {
    const count = (await GetHistory(COUNT_LIMIT).catch(() => [])).length;
    if (count === 0) return;
    const n = count >= COUNT_LIMIT ? `${COUNT_LIMIT.toLocaleString()}+` : count.toLocaleString();
    const confirmed = await confirm({
      title: "Clear history",
      message: `Delete all ${n} ${count === 1 ? "dictation" : "dictations"}? This can't be undone.`,
      confirmText: "Delete all",
      isDestructive: true,
    });
    if (!confirmed) return;

    try {
      await ClearAllHistory();
      pendingDeletes.current.forEach((timer) => clearTimeout(timer));
      pendingDeletes.current.clear();
      setTranscripts([]);
      setHidden(new Set());
      setSelectedId(null);
    } catch (err) {
      showToast(`Couldn't clear history: ${err}`, "error");
    }
  };

  const handleRewrite = async (id: number, instr: string) => {
    if (rewriting) return;
    setRewriting(true);
    try {
      const polished = await RetryRefinement(id, instr);
      setTranscripts((prev) => prev.map((t) => (t.id === id ? { ...t, polished_text: polished } : t)));
      setVersion("cleaned");
      setInstruction("");
    } catch (err) {
      showToast(`Couldn't rewrite it: ${err}`, "error");
    } finally {
      setRewriting(false);
    }
  };

  const hasRaw = !!selected?.raw_text && selected.raw_text !== selected.polished_text;
  const shownText = selected ? (version === "raw" && hasRaw ? selected.raw_text : selected.polished_text) : "";

  const rows: JSX.Element[] = [];
  let lastGroup = "";
  const now = new Date();
  for (const t of visible) {
    const group = dateGroup(t.timestamp, now);
    if (group !== lastGroup) {
      rows.push(
        <h3
          key={`g-${t.id}`}
          className="sticky top-0 z-10 px-4 pt-3 pb-1 text-xs font-semibold text-secondary bg-surface"
        >
          {group}
        </h3>,
      );
      lastGroup = group;
    }
    rows.push(
      <HistoryItem
        key={t.id}
        transcript={t}
        isSelected={selectedId === t.id}
        tabbable={selected ? selected.id === t.id : t === visible[0]}
        searchQuery={searchQuery}
        onSelect={select}
      />,
    );
  }

  return (
    <div className="flex h-full min-h-0 animate-fade-in">
      <div className="w-72 shrink-0 border-r border-border flex flex-col bg-surface">
        <div className="p-4 pb-3 flex items-center justify-between">
          <h2 className="text-base font-semibold text-text">History</h2>
          {visible.length > 0 && (
            <button type="button" onClick={handleClearAll} className="text-xs text-secondary hover:text-[var(--danger)]">
              Clear history…
            </button>
          )}
        </div>

        <div className="px-4 pb-3 border-b border-border">
          <div className="relative">
            <svg
              className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-tertiary pointer-events-none"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              aria-hidden="true"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <input
              ref={searchRef}
              type="search"
              placeholder="Search your dictations"
              aria-label="Search your dictations"
              aria-keyshortcuts="Meta+F"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "ArrowDown" && visible.length > 0) {
                  e.preventDefault();
                  const id = selectedId ?? visible[0].id;
                  select(id);
                  listRef.current?.querySelector<HTMLElement>(`[data-id="${id}"]`)?.focus();
                }
              }}
              className="input w-full pl-10 !text-[13px]"
            />
          </div>
        </div>

        <div
          ref={listRef}
          className="flex-1 overflow-y-auto"
          role="group"
          aria-label="Dictations — use the arrow keys to move"
          onKeyDown={handleListKeyDown}
        >
          {loading === "first" ? (
            <div className="p-4 text-center text-[13px] text-tertiary">Loading…</div>
          ) : visible.length === 0 ? (
            <div className="p-8 text-center text-[13px] text-secondary">
              {searchQuery ? "No dictations match that search" : "No dictations yet"}
            </div>
          ) : (
            <div>
              {rows}
              {loading === "more" && <div className="p-3 text-center text-xs text-tertiary">Loading more…</div>}
              <div ref={sentinelRef} />
            </div>
          )}
        </div>
      </div>

      <div className="flex-1 min-w-0 flex flex-col bg-background">
        {selected ? (
          <>
            <div className="px-6 py-4 border-b border-border flex items-center justify-between gap-4">
              <p className="text-[13px] text-secondary">
                {[selected.app_name, formatFullDate(selected.timestamp)].filter(Boolean).join(" · ")}
              </p>
              <button
                type="button"
                onClick={() => handleDelete(selected)}
                title="Delete this dictation"
                aria-label="Delete this dictation"
                className="p-2 text-tertiary hover:text-[var(--danger)] hover:bg-danger/10 rounded-lg transition-colors"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={TRASH_PATH} />
                </svg>
              </button>
            </div>

            <div className="flex-1 overflow-y-auto p-6 space-y-4">
              {hasRaw && <VersionToggle value={version} onChange={setVersion} />}

              <div className="card p-4">
                <p className="text-sm text-text whitespace-pre-wrap leading-relaxed select-text">
                  {highlightText(shownText, searchQuery)}
                </p>
              </div>

              <div className="flex gap-2">
                <button type="button" className="btn-secondary !py-1.5 !px-3 !text-xs" onClick={() => copy(shownText)}>
                  Copy
                </button>
                <button type="button" className="btn-secondary !py-1.5 !px-3 !text-xs" onClick={() => paste(shownText)}>
                  {pasteLabel(pasteApp)}
                </button>
              </div>

              <form
                className="flex gap-2 pt-2"
                onSubmit={(e) => {
                  e.preventDefault();
                  handleRewrite(selected.id, instruction.trim());
                }}
              >
                <input
                  type="text"
                  placeholder="Ask for a change — e.g. “shorter” or “make it friendlier”"
                  aria-label="Ask for a change"
                  value={instruction}
                  onChange={(e) => setInstruction(e.target.value)}
                  disabled={rewriting}
                  className="input flex-1 min-w-0 !text-[13px]"
                />
                <button type="submit" disabled={rewriting || !instruction.trim()} className="btn-primary shrink-0">
                  {rewriting ? "Rewriting…" : "Rewrite"}
                </button>
                <button
                  type="button"
                  onClick={() => handleRewrite(selected.id, "")}
                  disabled={rewriting}
                  title="Run the standard clean-up on what you said again"
                  className="btn-secondary shrink-0"
                >
                  Clean up again
                </button>
              </form>

              {(selected.llm_provider || !!selected.words_per_second) && (
                <details className="text-xs text-secondary">
                  <summary className="cursor-pointer select-none">Details</summary>
                  <dl className="mt-2 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
                    {selected.llm_provider && (
                      <>
                        <dt>Clean-up</dt>
                        <dd className="text-text">
                          {selected.llm_provider}
                          {selected.llm_model && ` · ${selected.llm_model}`}
                        </dd>
                      </>
                    )}
                    {!!selected.words_per_second && (
                      <>
                        <dt>Speed</dt>
                        <dd className="text-text">{Math.round(selected.words_per_second * 60)} words per minute</dd>
                      </>
                    )}
                    {!!selected.translation_time_ms && (
                      <>
                        <dt>Clean-up time</dt>
                        <dd className="text-text">{(selected.translation_time_ms / 1000).toFixed(1)} s</dd>
                      </>
                    )}
                  </dl>
                </details>
              )}
            </div>
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center text-[13px] text-secondary">
            Select a dictation to see it here
          </div>
        )}
      </div>

      <ConfirmModalComponent />
    </div>
  );
}
