import { useState, useEffect } from "react";
import {
  GetHistory,
  SearchHistory,
  DeleteTranscript,
  ClearAllHistory,
  RetryWithGemini,
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
}

export default function HistoryView() {
  const [transcripts, setTranscripts] = useState<Transcript[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [retryInstruction, setRetryInstruction] = useState("");
  const [retrying, setRetrying] = useState(false);

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

  const handleRetry = async (id: number) => {
    setRetrying(true);
    try {
      const newPolished = await RetryWithGemini(id, retryInstruction);
      setTranscripts(
        transcripts.map((t) =>
          t.id === id ? { ...t, polished_text: newPolished } : t
        )
      );
      setRetryInstruction("");
    } catch (err) {
      console.error("Failed to retry:", err);
    } finally {
      setRetrying(false);
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
      <div className="w-80 border-r border-[var(--border)] flex flex-col bg-secondary">
        {/* Header */}
        <div className="p-4 border-b border-[var(--border)]">
          <h2 className="font-serif text-xl font-medium text-primary mb-3">
            History
          </h2>
          {/* Search */}
          <div className="relative">
            <input
              type="text"
              placeholder="Search transcripts..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="input w-full pl-9 text-sm"
            />
            <svg
              className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-tertiary"
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
          </div>
        </div>

        {/* Clear All button */}
        {transcripts.length > 0 && (
          <div className="px-4 py-2 border-b border-[var(--border)]">
            <button
              onClick={handleClearAll}
              className="w-full px-3 py-2 text-sm text-red-500 hover:bg-red-500/10 rounded-xl transition-colors flex items-center justify-center gap-2"
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
              Clear All
            </button>
          </div>
        )}

        {/* Transcript list */}
        <div className="flex-1 overflow-y-auto">
          {loading ? (
            <div className="p-4 text-center text-tertiary">Loading...</div>
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
              <p className="text-sm">
                {searchQuery ? "No results found" : "No transcripts yet"}
              </p>
            </div>
          ) : (
            <div>
              {transcripts.map((t) => (
                <button
                  key={t.id}
                  onClick={() => setSelectedId(t.id)}
                  className={`w-full p-4 text-left transition-all border-b border-[var(--border)] ${
                    selectedId === t.id
                      ? "bg-[var(--accent)]/5 border-l-2 border-l-[var(--accent)]"
                      : "hover:bg-tertiary border-l-2 border-l-transparent"
                  }`}
                >
                  <p className="text-xs text-tertiary mb-1">
                    {formatDate(t.timestamp)}
                  </p>
                  <p className="text-sm text-primary line-clamp-2">
                    {truncate(t.polished_text || t.raw_text, 80)}
                  </p>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Detail pane */}
      <div className="flex-1 flex flex-col bg-primary">
        {selectedTranscript ? (
          <>
            {/* Header */}
            <div className="p-6 border-b border-[var(--border)] flex items-center justify-between">
              <div>
                <p className="text-xs text-tertiary">
                  {formatDate(selectedTranscript.timestamp)}
                </p>
                <p className="text-sm text-secondary mt-1">
                  Mode:{" "}
                  <span className="capitalize">
                    {selectedTranscript.mode || "casual"}
                  </span>
                </p>
              </div>
              <button
                onClick={() => handleDelete(selectedTranscript.id)}
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
              {/* Raw text */}
              <div>
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-xs font-medium text-tertiary uppercase tracking-wider">
                    Raw Transcription
                  </h3>
                  <button
                    onClick={() => handleCopy(selectedTranscript.raw_text)}
                    className="text-xs text-tertiary hover:text-[var(--accent)] transition-colors"
                  >
                    Copy
                  </button>
                </div>
                <div className="card p-4">
                  <p className="text-sm text-secondary whitespace-pre-wrap leading-relaxed">
                    {selectedTranscript.raw_text}
                  </p>
                </div>
              </div>

              {/* Polished text */}
              <div>
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-xs font-medium text-tertiary uppercase tracking-wider">
                    Polished Result
                  </h3>
                  <button
                    onClick={() => handleCopy(selectedTranscript.polished_text)}
                    className="text-xs text-tertiary hover:text-[var(--accent)] transition-colors"
                  >
                    Copy
                  </button>
                </div>
                <div className="card p-4">
                  <p className="text-primary whitespace-pre-wrap leading-relaxed">
                    {selectedTranscript.polished_text}
                  </p>
                </div>
              </div>

              {/* Retry with Gemini */}
              <div>
                <h3 className="text-xs font-medium text-tertiary uppercase tracking-wider mb-3">
                  Retry with Gemini
                </h3>
                <div className="flex gap-3">
                  <input
                    type="text"
                    placeholder="Enter instruction (e.g., 'make it more formal')"
                    value={retryInstruction}
                    onChange={(e) => setRetryInstruction(e.target.value)}
                    className="input flex-1 text-sm"
                  />
                  <button
                    onClick={() => handleRetry(selectedTranscript.id)}
                    disabled={retrying}
                    className="btn-primary disabled:opacity-50"
                  >
                    {retrying ? "Retrying..." : "Retry"}
                  </button>
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
              <p>Select a transcript to view details</p>
            </div>
          </div>
        )}
      </div>

      {/* Confirm Modal */}
      <ConfirmModalComponent />
    </div>
  );
}
