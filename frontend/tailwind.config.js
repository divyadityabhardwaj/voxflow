/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        background: "var(--background)",
        surface: "var(--surface)",
        "surface-hover": "var(--surface-hover)",
        text: "var(--text)",
        primary: "var(--primary)",
        secondary: "var(--secondary)",
        accent: {
          DEFAULT: "var(--accent)",
          hover: "var(--accent-hover)",
          soft: "var(--accent-soft)",
        },
        recording: {
          DEFAULT: "var(--recording)",
          bg: "var(--recording-bg)",
        },
        processing: {
          DEFAULT: "var(--processing)",
          bg: "var(--processing-bg)",
        },
        idle: {
          DEFAULT: "var(--idle)",
          bg: "var(--idle-bg)",
        },
        border: {
          DEFAULT: "var(--border)",
          hover: "var(--border-hover)",
          strong: "var(--border-strong)",
        },
      },
      fontFamily: {
        sans: ["Inter", "-apple-system", "BlinkMacSystemFont", "Segoe UI", "sans-serif"],
      },
      boxShadow: {
        "soft-sm": "var(--shadow-sm)",
        "soft-md": "var(--shadow-md)",
        "soft-lg": "var(--shadow-lg)",
      },
      borderRadius: {
        sm: "var(--radius-sm)",
        md: "var(--radius-md)",
        lg: "var(--radius-lg)",
        xl: "var(--radius-xl)",
      },
      spacing: {
        sidebar: "56px",
      },
    },
  },
  plugins: [],
};
