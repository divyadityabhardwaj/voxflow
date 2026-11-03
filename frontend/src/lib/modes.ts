export const MODES = [
  {
    id: "refine",
    name: "Clean up and paste",
    desc: "Removes filler words and fixes punctuation with your AI clean-up service.",
  },
  {
    id: "raw",
    name: "Paste exactly what I said",
    desc: "Pastes the transcript without AI clean-up.",
  },
  {
    id: "copy-only",
    name: "Copy only — I'll paste myself",
    desc: "Copies the transcript without AI clean-up.",
  },
] as const;
