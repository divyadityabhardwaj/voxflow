/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        background: "var(--background)",
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
        serif: ["Playfair Display", "Georgia", "serif"],
        sans: ["DM Sans", "-apple-system", "BlinkMacSystemFont", "Segoe UI", "sans-serif"],
      },
      boxShadow: {
        'soft-sm': 'var(--shadow-sm)',
        'soft-md': 'var(--shadow-md)',
        'soft-lg': 'var(--shadow-lg)',
        'cartoon': '6px 6px 0px var(--primary)',
        'cartoon-sm': '3px 3px 0px var(--primary)',
        'cartoon-hover': '3px 3px 0px var(--primary)',
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
        "wave": "wave 0.8s ease-in-out infinite",
        "slide-up-fade": "slide-up-fade 0.3s cubic-bezier(0.16, 1, 0.3, 1)",
        "scale-in": "scale-in 0.25s cubic-bezier(0.34, 1.56, 0.64, 1)",
        "bounce-in": "bounce-in 0.4s cubic-bezier(0.34, 1.56, 0.64, 1)",
        "float": "float 3s ease-in-out infinite",
      },
      spacing: {
        'sidebar': '64px',
      },
    },
  },
  plugins: [],
};