export const MODES = [
  {
    id: "refine",
    name: "Clean up and paste",
    desc: "Removes filler words and fixes punctuation with your AI clean-up service.",
  },
  {
    id: "raw",
    name: "Paste exactly what I said",
    desc: "Nothing leaves your Mac.",
  },
  {
    id: "copy-only",
    name: "Copy only — I'll paste myself",
    desc: "Puts the text on the clipboard. Nothing leaves your Mac.",
  },
] as const;
