import { useState, useEffect, useRef, useCallback, memo } from "react";
import {
  DeleteTranscript,
  ClearAllHistory,
  CopyToClipboard,
  InjectText,
  GetHistoryPage,
  SearchHistoryPage,
  RetryRefinement,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { useConfirmModal } from "./ConfirmModal";
import { useToast } from "../contexts/ToastContext";
import { Events } from "../constants/events";

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

const escapeRegExp = (string: string) => {
  return string.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
};

const highlightText = (text: string, highlight: string) => {
  if (!highlight.trim()) {
    return <span>{text}</span>;
  }
  const regex = new RegExp(`(${escapeRegExp(highlight)})`, "gi");
  const parts = text.split(regex);
  return (
    <span>
      {parts.map((part, i) =>
        part.toLowerCase() === highlight.toLowerCase() ? (
          <mark
            key={i}
            className="bg-accent-soft text-primary px-0.5 rounded font-medium"
          >
            {part}
          </mark>
        ) : (
          part
        )
      )}
    </span>
  );
};

const PASTE_HINT = "Paste into the app you were using";

const dateFormatter = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});

const formatDate = (timestamp: string) => {
  try {
    return dateFormatter.format(new Date(timestamp));
  } catch {
    return timestamp;
  }
};

const truncate = (text: string, length: number) => {
  if (!text) return "";
  if (text.length <= length) return text;
  return text.substring(0, length) + "...";
};

interface HistoryItemProps {
  transcript: Transcript;
  isSelected: boolean;
  searchQuery: string;
  onClick: () => void;
}

const HistoryItem = memo(function HistoryItem({
  transcript,
  isSelected,
  searchQuery,
  onClick,
}: HistoryItemProps) {
  return (
    <button
      onClick={onClick}
      className={`w-full p-4 text-left transition-all border-b border-border ${
        isSelected
          ? "bg-accent-soft border-l-4 border-l-primary"
          : "hover:bg-secondary border-l-4 border-l-transparent"
      }`}
    >
      <p className="text-xs text-tertiary mb-1 font-medium truncate">
        {transcript.app_name && `${transcript.app_name} · `}
        {formatDate(transcript.timestamp)}
      </p>
      <p className="text-sm text-text font-medium line-clamp-2">
        {highlightText(truncate(transcript.polished_text || transcript.raw_text, 80), searchQuery)}
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

export default function HistoryView() {
  const [transcripts, setTranscripts] = useState<Transcript[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [loading, setLoading] = useState<"first" | "more" | null>("first");
  // Ref + object identity so stale in-flight pages drop after query change.
  const page = useRef<PageState>(freshPage(""));

  const [instruction, setInstruction] = useState("");
  const [rewriting, setRewriting] = useState(false);

  const { confirm, ConfirmModalComponent } = useConfirmModal();
  const { showToast } = useToast();

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

  const selectedTranscript = transcripts.find((t) => t.id === selectedId);

  const handleDelete = async (id: number) => {
    const confirmed = await confirm({
      title: "Delete Transcript",
      message: "Are you sure you want to delete this transcript?",
      confirmText: "Delete",
      isDestructive: true,
    });
    if (!confirmed) return;

    try {
      await DeleteTranscript(id);
      setTranscripts((prev) => prev.filter((t) => t.id !== id));
      if (selectedId === id) setSelectedId(null);
    } catch (err) {
      console.error("Failed to delete:", err);
    }
  };

  const handleClearAll = async () => {
    const confirmed = await confirm({
      title: "Clear All History",
      message:
        "Are you sure you want to delete ALL transcripts? This cannot be undone.",
      confirmText: "Delete All",
      isDestructive: true,
    });
    if (!confirmed) return;

    try {
      await ClearAllHistory();
      setTranscripts([]);
      setSelectedId(null);
    } catch (err) {
      console.error("Failed to clear history:", err);
    }
  };

  const handleCopy = async (text: string) => {
    try {
      await CopyToClipboard(text);
      showToast("Copied", "success");
    } catch (err) {
      console.error("Failed to copy:", err);
      showToast("Couldn't copy the text", "error");
    }
  };

  const handlePaste = async (text: string) => {
    try {
      await InjectText(text);
      showToast("Pasted", "success");
    } catch (err) {
      console.error("Failed to paste:", err);
      showToast("Couldn't paste the text", "error");
    }
  };

  const handleRewrite = async (id: number, instr: string) => {
    if (rewriting) return;
    setRewriting(true);
    try {
      const polished = await RetryRefinement(id, instr);
      setTranscripts((prev) =>
        prev.map((t) => (t.id === id ? { ...t, polished_text: polished } : t)),
      );
      setInstruction("");
    } catch (err) {
      showToast(`Rewrite failed: ${err}`);
    } finally {
      setRewriting(false);
    }
  };

  return (
    <div className="flex h-full min-h-0 animate-fade-in">
      <div className="w-72 shrink-0 border-r border-border flex flex-col bg-surface">
        <div className="p-4 border-b border-border flex items-center justify-between">
          <h2 className="text-base font-semibold text-text">History</h2>
          {transcripts.length > 0 && (
            <button
              onClick={handleClearAll}
              title="Delete all transcripts"
              aria-label="Delete all dictations"
              className="p-1.5 text-tertiary hover:text-[var(--danger)] hover:bg-[var(--danger)]/10 rounded-lg transition-colors"
            >
              <svg
                className="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                />
              </svg>
            </button>
          )}
        </div>

        <div className="px-4 py-3 border-b border-border">
          <div className="relative">
            <svg
              className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-tertiary pointer-events-none"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
              />
            </svg>
            <input
              type="text"
              placeholder="Search your dictations"
              aria-label="Search your dictations"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="input w-full pl-10 text-sm"
            />
          </div>
        </div>

        <div className="flex-1 overflow-y-auto">
          {loading === "first" ? (
            <div className="p-4 text-center text-tertiary font-medium">
              Loading...
            </div>
          ) : transcripts.length === 0 ? (
            <div className="p-8 text-center text-tertiary">
              <svg
                className="w-12 h-12 mx-auto mb-3 opacity-50"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
                strokeWidth={1}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
                />
              </svg>
              <p className="text-sm font-medium">
                {searchQuery ? "No results found" : "No transcripts yet"}
              </p>
            </div>
          ) : (
            <div>
              {transcripts.map((t) => (
                <HistoryItem
                  key={t.id}
                  transcript={t}
                  isSelected={selectedId === t.id}
                  searchQuery={searchQuery}
                  onClick={() => setSelectedId(t.id)}
                />
              ))}
              {loading === "more" && (
                <div className="p-3 text-center text-xs text-tertiary font-medium">
                  Loading more…
                </div>
              )}
              <div ref={sentinelRef} />
            </div>
          )}
        </div>
      </div>

      <div className="flex-1 flex flex-col bg-background">
        {selectedTranscript ? (
          <>
            <div className="p-5 border-b-2 border-border flex items-center justify-between">
              <div>
                <p className="text-xs text-tertiary font-medium">
                  {formatDate(selectedTranscript.timestamp)}
                </p>
                {selectedTranscript.llm_provider && (
                  <p className="text-sm text-tertiary mt-2 flex items-center gap-2">
                    <span
                      title="AI Model used"
                      className="px-2 py-0.5 rounded-lg bg-accent-soft text-primary text-xs font-bold"
                    >
                      {" "}
                      {selectedTranscript.llm_provider === "local"
                        ? "Local"
                        : selectedTranscript.llm_provider}
                      {selectedTranscript.llm_model &&
                        ` • ${selectedTranscript.llm_model}`}
                    </span>
                    {selectedTranscript.tokens_per_second !== undefined &&
                      selectedTranscript.tokens_per_second > 0 && (
                        <span
                          title="Generation speed"
                          className="px-2 py-0.5 rounded-md bg-accent-soft text-secondary text-xs font-medium"
                        >
                          ⚡ {selectedTranscript.tokens_per_second.toFixed(1)}{" "}
                          t/s
                        </span>
                      )}
                    {selectedTranscript.words_per_second !== undefined &&
                      selectedTranscript.words_per_second > 0 && (
                        <span
                          title="End-to-end transcription speed"
                          className="px-2 py-0.5 rounded-lg bg-accent-soft text-secondary text-xs font-bold"
                        >
                          {(selectedTranscript.words_per_second * 60).toFixed(
                            0,
                          )}{" "}
                          WPM
                        </span>
                      )}
                  </p>
                )}
              </div>
              <button
                onClick={() => handleDelete(selectedTranscript.id)}
                title="Delete this transcript permanently"
                aria-label="Delete this dictation"
                className="p-2 text-tertiary hover:text-red-500 hover:bg-red-500/10 rounded-xl transition-colors"
              >
                <svg
                  className="w-5 h-5"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                  />
                </svg>
              </button>
            </div>

            <div className="flex-1 overflow-y-auto p-6 space-y-6">
              {selectedTranscript.raw_text &&
                selectedTranscript.raw_text !==
                  selectedTranscript.polished_text && (
                  <div>
                    <div className="flex items-center justify-between mb-3">
                      <h3 className="text-xs font-medium text-secondary uppercase tracking-wide">
                        As you said it
                      </h3>
                      <div className="flex items-center gap-3">
                        <button
                          onClick={() => handleCopy(selectedTranscript.raw_text)}
                          title="Copy original text to clipboard"
                          className="text-xs text-tertiary hover:text-primary transition-colors font-bold"
                        >
                          Copy
                        </button>
                        <span className="text-border">|</span>
                        <button
                          onClick={() => handlePaste(selectedTranscript.raw_text)}
                          title={PASTE_HINT}
                          className="text-xs text-tertiary hover:text-primary transition-colors font-bold flex items-center gap-1"
                        >
                          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M8 4H6a2 2 0 00-2 2v12a2 2 0 002 2h12a2 2 0 002-2V6a2 2 0 00-2-2h-2m-4-1v8m0 0l3-3m-3 3L9 8m-5 5h2.586a1 1 0 01.707.293l2.414 2.414a1 1 0 00.707.293h3.172a1 1 0 00.707-.293l2.414-2.414a1 1 0 01.707-.293H20" />
                          </svg>
                          Paste
                        </button>
                      </div>
                    </div>
                    <div className="card p-4">
                      <p className="text-secondary whitespace-pre-wrap leading-relaxed font-medium">
                        {highlightText(selectedTranscript.raw_text, searchQuery)}
                      </p>
                    </div>
                  </div>
                )}

              <div>
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-xs font-medium text-secondary uppercase tracking-wide">
                    {selectedTranscript.raw_text &&
                    selectedTranscript.raw_text !==
                      selectedTranscript.polished_text
                      ? "Cleaned up"
                      : "Result"}
                  </h3>
                  <div className="flex items-center gap-3">
                    <button
                      onClick={() => handleCopy(selectedTranscript.polished_text)}
                      title="Copy polished text to clipboard"
                      className="text-xs text-tertiary hover:text-primary transition-colors font-bold"
                    >
                      Copy
                    </button>
                    <span className="text-border">|</span>
                    <button
                      onClick={() => handlePaste(selectedTranscript.polished_text)}
                      title={PASTE_HINT}
                      className="text-xs text-tertiary hover:text-primary transition-colors font-bold flex items-center gap-1"
                    >
                      <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M8 4H6a2 2 0 00-2 2v12a2 2 0 002 2h12a2 2 0 002-2V6a2 2 0 00-2-2h-2m-4-1v8m0 0l3-3m-3 3L9 8m-5 5h2.586a1 1 0 01.707.293l2.414 2.414a1 1 0 00.707.293h3.172a1 1 0 00.707-.293l2.414-2.414a1 1 0 01.707-.293H20" />
                      </svg>
                      Paste
                    </button>
                  </div>
                </div>
                <div className="card p-4">
                  <p className="text-text whitespace-pre-wrap leading-relaxed font-medium">
                    {highlightText(selectedTranscript.polished_text, searchQuery)}
                  </p>
                </div>
                <form
                  className="flex gap-2 mt-3"
                  onSubmit={(e) => {
                    e.preventDefault();
                    handleRewrite(selectedTranscript.id, instruction.trim());
                  }}
                >
                  <input
                    type="text"
                    placeholder="Rewrite instruction, e.g. make it formal"
                    value={instruction}
                    onChange={(e) => setInstruction(e.target.value)}
                    disabled={rewriting}
                    className="input flex-1 min-w-0 text-sm"
                  />
                  <button
                    type="submit"
                    disabled={rewriting || !instruction.trim()}
                    className="btn btn-primary shrink-0"
                  >
                    {rewriting ? "Rewriting…" : "Rewrite"}
                  </button>
                  <button
                    type="button"
                    onClick={() => handleRewrite(selectedTranscript.id, "")}
                    disabled={rewriting}
                    title="Re-run the standard clean-up on what you said"
                    className="btn btn-secondary shrink-0"
                  >
                    Clean up again
                  </button>
                </form>
              </div>
            </div>
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center text-tertiary">
            <div className="text-center">
              <svg
                className="w-16 h-16 mx-auto mb-4 opacity-30"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
                strokeWidth={1}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                />
              </svg>
              <p className="font-medium">Select a transcript to view details</p>
            </div>
          </div>
        )}
      </div>

      <ConfirmModalComponent />
    </div>
  );
}
