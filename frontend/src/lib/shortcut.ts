// Mirrors internal/hotkey/hotkey.go parseHotkey/parseKey: anything outside
// these sets is rejected by the backend with "unknown key"/"unknown modifier".
const MODIFIER_ALIASES: Record<string, string> = {
  cmd: "cmd",
  command: "cmd",
  super: "cmd",
  ctrl: "ctrl",
  control: "ctrl",
  shift: "shift",
  alt: "alt",
  option: "alt",
  opt: "alt",
};

const KEY_ALIASES: Record<string, string> = {
  enter: "return",
  esc: "escape",
};

const SUPPORTED_KEYS = new Set([
  ..."abcdefghijklmnopqrstuvwxyz0123456789",
  "space",
  "return",
  "escape",
  "tab",
]);

const MODIFIER_ORDER = ["ctrl", "alt", "shift", "cmd"] as const;
const GLYPHS: Record<string, string> = {
  ctrl: "⌃",
  alt: "⌥",
  shift: "⇧",
  cmd: "⌘",
};
const KEY_LABELS: Record<string, string> = {
  space: "Space",
  return: "Return",
  escape: "Esc",
  tab: "Tab",
};

const RESERVED = new Set([
  "cmd+space",
  "cmd+tab",
  "cmd+q",
  "cmd+w",
  "cmd+c",
  "cmd+v",
  "cmd+x",
  "cmd+z",
]);

const canon = (part: string) =>
  MODIFIER_ALIASES[part] ?? KEY_ALIASES[part] ?? part;

const isModifier = (part: string) => part in GLYPHS;

function split(s: string): { mods: string[]; key: string } {
  const parts = s.toLowerCase().split("+").filter(Boolean).map(canon);
  const key = parts.length && !isModifier(parts[parts.length - 1])
    ? parts[parts.length - 1]
    : "";
  const mods = MODIFIER_ORDER.filter((m) => parts.includes(m));
  return { mods, key };
}

// Physical key from KeyboardEvent.code, so ⌥/⇧ combos and non-QWERTY layouts
// record the key position the Carbon hotkey (an ANSI keycode) will match.
export function keyFromCode(code: string): string | null {
  if (/^Key[A-Z]$/.test(code)) return code.slice(3).toLowerCase();
  if (/^Digit[0-9]$/.test(code)) return code.slice(5);
  const named: Record<string, string> = {
    Space: "space",
    Enter: "return",
    Escape: "escape",
    Tab: "tab",
  };
  return named[code] ?? null;
}

export function modifiersOf(e: {
  ctrlKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
  metaKey: boolean;
}): string[] {
  const held = { ctrl: e.ctrlKey, alt: e.altKey, shift: e.shiftKey, cmd: e.metaKey };
  return MODIFIER_ORDER.filter((m) => held[m]);
}

export function normalizeShortcut(s: string): string {
  const { mods, key } = split(s);
  return [...mods, key].filter(Boolean).join("+");
}

export function hasShortcutKey(s: string): boolean {
  return split(s).key !== "";
}

// "cmd+shift+space" → "⇧⌘Space"
export function formatShortcut(s: string): string {
  const { mods, key } = split(s);
  const label = KEY_LABELS[key] ?? key.toUpperCase();
  return mods.map((m) => GLYPHS[m]).join("") + label;
}

export function validateShortcut(s: string, otherShortcuts: string[] = []): string | null {
  const { mods, key } = split(s);
  if (!key) return "Now press a letter, number, Space, Return, Esc or Tab.";
  if (!SUPPORTED_KEYS.has(key)) {
    return `${key.toUpperCase()} can't be used. Pick a letter, number, Space, Return, Esc or Tab.`;
  }
  if (mods.length === 0) return "Add at least one of ⌃ ⌥ ⇧ ⌘.";
  const normalized = normalizeShortcut(s);
  if (otherShortcuts.some((o) => o && normalized === normalizeShortcut(o))) {
    return "That's already one of your other VoxFlow shortcuts.";
  }
  if (RESERVED.has(normalized)) {
    return `${formatShortcut(s)} is a macOS system shortcut. Pick another.`;
  }
  return null;
}
