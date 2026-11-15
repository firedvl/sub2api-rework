import defaultColors from 'tailwindcss/colors'

/**
 * Text utilities for status, accent, and muted hues resolve to theme-aware
 * tokens from src/style.css. Authors can keep writing `text-green-600` or
 * `dark:text-gray-400`; the rendered color always meets WCAG AA on the active
 * theme's surfaces. Backgrounds and borders keep the raw ramps below.
 */
const themeText = (token) => `rgb(var(--operator-${token}-rgb) / <alpha-value>)`

const withThemeText = (ramp, token, shades) => ({
  ...ramp,
  ...Object.fromEntries(shades.map((shade) => [shade, themeText(token)])),
})

// 300 shades stay raw: they are used on dark surfaces, where they already read well.
const statusShades = [400, 500, 600, 700]
const mutedShades = [400, 500, 600]

// Neutral ramp for light surfaces and text (cool graphite).
const gray = {
  50: '#f6f7f9',
  100: '#f1f3f6',
  200: '#e3e6ec',
  300: '#cfd4dd',
  400: '#9aa1b0',
  500: '#6b7383',
  600: '#4d5463',
  700: '#363c49',
  800: '#232733',
  900: '#171a22',
  950: '#0b0d12'
}

// Compressed neutral ramp for dark surfaces: 950 page, 900 surface,
// 800 raised, 700 border and hover, 600 strong border, 400 muted text.
const dark = {
  50: '#f4f5f8',
  100: '#e9ebf1',
  200: '#d5d9e2',
  300: '#b9bfcc',
  400: '#a3a9b8',
  500: '#4e5567',
  600: '#323a4d',
  700: '#232836',
  800: '#181c25',
  900: '#11141a',
  950: '#0b0d12'
}

// One accent (iris) for the primary action, selection, focus, and links.
const primary = {
  50: '#f1f0ff',
  100: '#e6e3ff',
  200: '#cfcaff',
  300: '#b0a9ff',
  400: '#8f85ff',
  500: '#6d5efc',
  600: '#5b4ee6',
  700: '#4a3fd0',
  800: '#3c33a8',
  900: '#322c84',
  950: '#1e1a52'
}

/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        gray,
        dark,
        primary,
        accent: gray
      },
      textColor: {
        gray: withThemeText(gray, 'muted-foreground', mutedShades),
        dark: withThemeText(dark, 'muted-foreground', mutedShades),
        slate: withThemeText(defaultColors.slate, 'muted-foreground', mutedShades),
        primary: withThemeText(primary, 'accent-text', statusShades),
        blue: withThemeText(defaultColors.blue, 'accent-text', statusShades),
        green: withThemeText(defaultColors.green, 'success', statusShades),
        emerald: withThemeText(defaultColors.emerald, 'success', statusShades),
        amber: withThemeText(defaultColors.amber, 'warning', statusShades),
        yellow: withThemeText(defaultColors.yellow, 'warning', statusShades),
        orange: withThemeText(defaultColors.orange, 'warning', statusShades),
        red: withThemeText(defaultColors.red, 'destructive', statusShades),
        rose: withThemeText(defaultColors.rose, 'destructive', statusShades)
      },
      fontFamily: {
        sans: [
          '"Geist Sans"',
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          '"Segoe UI"',
          'Roboto',
          '"Helvetica Neue"',
          'Arial',
          '"PingFang SC"',
          '"Hiragino Sans GB"',
          '"Microsoft YaHei"',
          'sans-serif'
        ],
        mono: [
          '"JetBrains Mono"',
          'ui-monospace',
          'SFMono-Regular',
          'Menlo',
          'Monaco',
          'Consolas',
          'monospace'
        ]
      },
      // Every shadow utility resolves to the four ordered elevation tokens, so
      // controls, cards, popovers, and dialogs keep one consistent depth scale.
      boxShadow: {
        sm: 'var(--operator-shadow-xs)',
        DEFAULT: 'var(--operator-shadow-sm)',
        md: 'var(--operator-shadow-sm)',
        lg: 'var(--operator-shadow-md)',
        xl: 'var(--operator-shadow-md)',
        '2xl': 'var(--operator-shadow-lg)',
        glass: 'var(--operator-shadow-md)',
        'glass-sm': 'var(--operator-shadow-sm)',
        glow: 'none',
        'glow-lg': 'none',
        card: 'var(--operator-shadow-xs)',
        'card-hover': 'var(--operator-shadow-sm)',
        'inner-glow': 'none',
        popover: 'var(--operator-shadow-md)',
        overlay: 'var(--operator-shadow-lg)'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))'
      },
      // One shape language: 6px chips, 8px controls, 10-12px panels.
      borderRadius: {
        xl: '0.625rem',
        '2xl': '0.75rem',
        '3xl': '1rem',
        '4xl': '1.25rem'
      },
      transitionTimingFunction: {
        'ui-out': 'cubic-bezier(0.2, 0.9, 0.3, 1)',
        'ui-in': 'cubic-bezier(0.4, 0, 1, 1)'
      },
      transitionDuration: {
        fast: '120ms',
        normal: '180ms',
        slow: '240ms'
      },
      animation: {
        'fade-in': 'fadeIn 180ms cubic-bezier(0.2, 0.9, 0.3, 1)',
        'slide-up': 'slideUp 200ms cubic-bezier(0.2, 0.9, 0.3, 1)',
        'slide-down': 'slideDown 200ms cubic-bezier(0.2, 0.9, 0.3, 1)',
        'slide-in-right': 'slideInRight 200ms cubic-bezier(0.2, 0.9, 0.3, 1)',
        'scale-in': 'scaleIn 160ms cubic-bezier(0.2, 0.9, 0.3, 1)',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 1.6s linear infinite'
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' }
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(6px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-6px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(12px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.97)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' }
        }
      }
    }
  },
  plugins: []
}
