/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Using CSS variables for theme support
        primary: "var(--bg-primary)",
        secondary: "var(--bg-secondary)",
        tertiary: "var(--bg-tertiary)",
        elevated: "var(--bg-elevated)",
        accent: {
          DEFAULT: "var(--accent)",
          hover: "var(--accent-hover)",
          soft: "var(--accent-soft)",
        },
        // Status colors
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
        // Text colors
        txt: {
          primary: "var(--text-primary)",
          secondary: "var(--text-secondary)",
          tertiary: "var(--text-tertiary)",
        },
        // Border
        border: {
          DEFAULT: "var(--border)",
          hover: "var(--border-hover)",
        },
      },
      fontFamily: {
        serif: ["Playfair Display", "Georgia", "serif"],
        sans: ["DM Sans", "-apple-system", "BlinkMacSystemFont", "Segoe UI", "sans-serif"],
      },
      boxShadow: {
        'soft-sm': 'var(--shadow-sm)',
        'soft-md': 'var(--shadow-md)',
        'soft-lg': 'var(--shadow-lg)',
      },
      borderRadius: {
        'sm': 'var(--radius-sm)',
        'md': 'var(--radius-md)',
        'lg': 'var(--radius-lg)',
        'xl': 'var(--radius-xl)',
      },
      animation: {
        "fade-in": "fade-in 0.3s ease-out",
        "pulse-soft": "pulse-soft 2s ease-in-out infinite",
        "recording-ring": "recording-ring 1.5s ease-out infinite",
        "spin-slow": "spin-slow 2s linear infinite",
      },
      spacing: {
        'sidebar': '64px',
      },
    },
  },
  plugins: [],
};
