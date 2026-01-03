import { useState, useEffect } from "react";
import {
  GetHistory,
  SearchHistory,
  DeleteTranscript,
  RetryWithGemini,
  CopyToClipboard,
} from "../../wailsjs/go/main/App";

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
    if (!confirm("Are you sure you want to delete this transcript?")) return;
    try {
      await DeleteTranscript(id);
      setTranscripts(transcripts.filter((t) => t.id !== id));
      if (selectedId === id) setSelectedId(null);
    } catch (err) {
      console.error("Failed to delete:", err);
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
    <div className="flex h-[calc(100vh-3rem)]">
      {/* Sidebar */}
      <div className="w-72 border-r border-dark-800 flex flex-col bg-dark-900/50">
        {/* Search */}
        <div className="p-3 border-b border-dark-800">
          <div className="relative">
            <input
              type="text"
              placeholder="Search transcripts..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full px-3 py-2 pl-9 text-sm bg-dark-800 border border-dark-700 rounded-lg
                       text-dark-200 placeholder-dark-500
                       focus:outline-none focus:ring-2 focus:ring-accent-600 focus:border-transparent"
            />
            <svg
              className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-dark-500"
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

        {/* Transcript list */}
        <div className="flex-1 overflow-y-auto">
          {loading ? (
            <div className="p-4 text-center text-dark-500">Loading...</div>
          ) : transcripts.length === 0 ? (
            <div className="p-4 text-center text-dark-500">
              {searchQuery ? "No results found" : "No transcripts yet"}
            </div>
          ) : (
            <div className="divide-y divide-dark-800">
              {transcripts.map((t) => (
                <button
                  key={t.id}
                  onClick={() => setSelectedId(t.id)}
                  className={`w-full p-3 text-left transition-colors ${
                    selectedId === t.id
                      ? "bg-accent-600/20 border-l-2 border-accent-500"
                      : "hover:bg-dark-800 border-l-2 border-transparent"
                  }`}
                >
                  <p className="text-xs text-dark-500">
                    {formatDate(t.timestamp)}
                  </p>
                  <p className="text-sm text-dark-200 mt-1 line-clamp-2">
                    {truncate(t.polished_text || t.raw_text, 80)}
                  </p>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Detail pane */}
      <div className="flex-1 flex flex-col bg-dark-950">
        {selectedTranscript ? (
          <>
            {/* Header */}
            <div className="p-4 border-b border-dark-800 flex items-center justify-between">
              <div>
                <p className="text-xs text-dark-500">
                  {formatDate(selectedTranscript.timestamp)}
                </p>
                <p className="text-sm text-dark-400 mt-0.5">
                  Mode: {selectedTranscript.mode || "casual"}
                </p>
              </div>
              <button
                onClick={() => handleDelete(selectedTranscript.id)}
                className="p-2 text-dark-500 hover:text-red-400 hover:bg-red-900/20 rounded-lg transition-colors"
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
            <div className="flex-1 overflow-y-auto p-4 space-y-6">
              {/* Raw text */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <h3 className="text-xs font-medium text-dark-500 uppercase tracking-wider">
                    Raw Transcription
                  </h3>
                  <button
                    onClick={() => handleCopy(selectedTranscript.raw_text)}
                    className="text-xs text-dark-500 hover:text-accent-400"
                  >
                    Copy
                  </button>
                </div>
                <div className="p-4 bg-dark-900 rounded-lg border border-dark-800">
                  <p className="text-sm text-dark-300 whitespace-pre-wrap">
                    {selectedTranscript.raw_text}
                  </p>
                </div>
              </div>

              {/* Polished text */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <h3 className="text-xs font-medium text-dark-500 uppercase tracking-wider">
                    Polished Result
                  </h3>
                  <button
                    onClick={() => handleCopy(selectedTranscript.polished_text)}
                    className="text-xs text-dark-500 hover:text-accent-400"
                  >
                    Copy
                  </button>
                </div>
                <div className="p-4 bg-dark-900 rounded-lg border border-dark-800">
                  <p className="text-dark-200 whitespace-pre-wrap">
                    {selectedTranscript.polished_text}
                  </p>
                </div>
              </div>

              {/* Retry with Gemini */}
              <div>
                <h3 className="text-xs font-medium text-dark-500 uppercase tracking-wider mb-2">
                  Retry with Gemini
                </h3>
                <div className="flex gap-2">
                  <input
                    type="text"
                    placeholder="Enter instruction (e.g., 'make it more formal')"
                    value={retryInstruction}
                    onChange={(e) => setRetryInstruction(e.target.value)}
                    className="flex-1 px-3 py-2 text-sm bg-dark-900 border border-dark-800 rounded-lg
                             text-dark-200 placeholder-dark-500
                             focus:outline-none focus:ring-2 focus:ring-accent-600"
                  />
                  <button
                    onClick={() => handleRetry(selectedTranscript.id)}
                    disabled={retrying}
                    className="px-4 py-2 bg-accent-600 hover:bg-accent-500 disabled:opacity-50 
                             text-white text-sm rounded-lg transition-colors"
                  >
                    {retrying ? "Retrying..." : "Retry"}
                  </button>
                </div>
              </div>
            </div>
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center text-dark-500">
            Select a transcript to view details
          </div>
        )}
      </div>
    </div>
  );
}
