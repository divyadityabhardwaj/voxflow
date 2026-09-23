/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        background: "var(--background)",
        surface: "var(--surface)",
        "surface-hover": "rgb(var(--surface-hover-rgb) / <alpha-value>)",
        text: "var(--text)",
        primary: "var(--primary)",
        secondary: "var(--secondary)",
        accent: {
          DEFAULT: "var(--accent)",
          hover: "var(--accent-hover)",
          soft: "var(--accent-soft)",
        },
        recording: {
          DEFAULT: "rgb(var(--recording-rgb) / <alpha-value>)",
          bg: "var(--recording-bg)",
        },
        processing: {
          DEFAULT: "rgb(var(--processing-rgb) / <alpha-value>)",
          bg: "var(--processing-bg)",
        },
        idle: {
          DEFAULT: "var(--idle)",
          bg: "var(--idle-bg)",
        },
        danger: "rgb(var(--danger-rgb) / <alpha-value>)",
        border: {
          DEFAULT: "rgb(var(--border-rgb) / <alpha-value>)",
          hover: "var(--border-hover)",
          strong: "var(--border-strong)",
        },
      },
      fontFamily: {
        sans: [
          "Outfit",
          "ui-sans-serif",
          "system-ui",
          "sans-serif",
        ],
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
