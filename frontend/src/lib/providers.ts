export const PROVIDERS = [
  { id: "gemini", label: "Gemini", sub: "Google, free key, recommended" },
  { id: "openrouter", label: "OpenRouter", sub: "Many models with one key" },
  { id: "groq", label: "Groq", sub: "Fast cloud models" },
  { id: "cerebras", label: "Cerebras", sub: "Fast cloud models" },
  { id: "local", label: "Your own server", sub: "Ollama or LM Studio, text stays on this Mac" },
] as const;

export const KEY_URLS: Record<string, string> = {
  gemini: "https://aistudio.google.com/apikey",
  openrouter: "https://openrouter.ai/settings/keys",
  groq: "https://console.groq.com/keys",
  cerebras: "https://cloud.cerebras.ai",
};

export const providerLabel = (id: string) =>
  PROVIDERS.find((p) => p.id === id)?.label ?? id;
