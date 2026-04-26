import { useState, useEffect } from "react";
import {
  GetHistory,
  SearchHistory,
  DeleteTranscript,
  ClearAllHistory,
  CopyToClipboard,
} from "../../wailsjs/go/main/App";
import { useConfirmModal } from "./ConfirmModal";

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

export default function HistoryView() {
  const [transcripts, setTranscripts] = useState<Transcript[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [loading, setLoading] = useState(true);

  const { confirm, ConfirmModalComponent } = useConfirmModal();

  const loadTranscripts = async () => {
    setLoading(true);
    try {
      const data = searchQuery
        ? await SearchHistory(searchQuery, 100)
        : await GetHistory(100);
      setTranscripts(data || []);
    } catch (err) {
      console.error("Failed to load history:", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadTranscripts();
  }, []);

  useEffect(() => {
    const debounce = setTimeout(() => {
      loadTranscripts();
    }, 300);
    return () => clearTimeout(debounce);
  }, [searchQuery]);

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
      setTranscripts(transcripts.filter((t) => t.id !== id));
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
    } catch (err) {
      console.error("Failed to copy:", err);
    }
  };

  const formatDate = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const truncate = (text: string, length: number) => {
    if (text.length <= length) return text;
    return text.substring(0, length) + "...";
  };

  return (
    <div className="flex h-screen animate-fade-in">
      {/* Sidebar list */}
      <div className="w-80 border-r-2 border-border flex flex-col bg-background">
        {/* Header */}
        <div className="p-4 border-b-2 border-border flex items-center justify-between">
          <h2 className="font-serif text-lg font-bold text-text">History</h2>
          {transcripts.length > 0 && (
            <button
              onClick={handleClearAll}
              title="Delete all transcripts"
              className="p-1.5 text-tertiary hover:text-red-500 hover:bg-red-500/10 rounded-lg transition-colors"
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

        {/* Search */}
        <div className="px-4 py-3 border-b-2 border-border">
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
              placeholder=""
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="input w-full pl-10 text-sm"
            />
          </div>
        </div>

        {/* Transcript list */}
        <div className="flex-1 overflow-y-auto">
          {loading ? (
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
                <button
                  key={t.id}
                  onClick={() => setSelectedId(t.id)}
                  className={`w-full p-4 text-left transition-all border-b-2 border-border ${
                    selectedId === t.id
                      ? "bg-accent-soft border-l-4 border-l-primary"
                      : "hover:bg-secondary border-l-4 border-l-transparent"
                  }`}
                >
                  <p className="text-xs text-tertiary mb-1 font-medium">
                    {formatDate(t.timestamp)}
                  </p>
                  <p className="text-sm text-text font-medium line-clamp-2">
                    {truncate(t.polished_text || t.raw_text, 80)}
                  </p>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Detail pane */}
      <div className="flex-1 flex flex-col bg-background">
        {selectedTranscript ? (
          <>
            {/* Header */}
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
                          className="px-2 py-0.5 rounded-lg bg-green-500/10 text-green-500 text-xs font-bold"
                        >
                          ⚡ {selectedTranscript.tokens_per_second.toFixed(1)}{" "}
                          t/s
                        </span>
                      )}
                    {selectedTranscript.words_per_second !== undefined &&
                      selectedTranscript.words_per_second > 0 && (
                        <span
                          title="End-to-end transcription speed"
                          className="px-2 py-0.5 rounded-lg bg-blue-500/10 text-blue-500 text-xs font-bold"
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

            {/* Content */}
            <div className="flex-1 overflow-y-auto p-6 space-y-6">
              {selectedTranscript.raw_text &&
                selectedTranscript.raw_text !==
                  selectedTranscript.polished_text && (
                  <div>
                    <div className="flex items-center justify-between mb-3">
                      <h3 className="text-xs font-black text-text uppercase tracking-tighter">
                        Original
                      </h3>
                      <button
                        onClick={() => handleCopy(selectedTranscript.raw_text)}
                        title="Copy original text to clipboard"
                        className="text-xs text-tertiary hover:text-primary transition-colors font-bold"
                      >
                        Copy
                      </button>
                    </div>
                    <div className="card p-4">
                      <p className="text-secondary whitespace-pre-wrap leading-relaxed font-medium">
                        {selectedTranscript.raw_text}
                      </p>
                    </div>
                  </div>
                )}

              <div>
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-xs font-black text-text uppercase tracking-tighter">
                    {selectedTranscript.raw_text &&
                    selectedTranscript.raw_text !==
                      selectedTranscript.polished_text
                      ? "Polished"
                      : "Result"}
                  </h3>
                  <button
                    onClick={() => handleCopy(selectedTranscript.polished_text)}
                    title="Copy polished text to clipboard"
                    className="text-xs text-tertiary hover:text-primary transition-colors font-bold"
                  >
                    Copy
                  </button>
                </div>
                <div className="card p-4">
                  <p className="text-text whitespace-pre-wrap leading-relaxed font-medium">
                    {selectedTranscript.polished_text}
                  </p>
                </div>
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

      {/* Confirm Modal */}
      <ConfirmModalComponent />
    </div>
  );
}
