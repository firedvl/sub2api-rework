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

// Neutral ramp for light surfaces and text (zinc).
const gray = {
  50: '#fafafa',
  100: '#f4f4f5',
  200: '#e4e4e7',
  300: '#d4d4d8',
  400: '#a1a1aa',
  500: '#71717a',
  600: '#52525b',
  700: '#3f3f46',
  800: '#27272a',
  900: '#18181b',
  950: '#09090b'
}

// Compressed neutral ramp for dark surfaces: 950 page, 900 surface,
// 800 raised, 700 border and hover, 600 strong border, 400 muted text.
const dark = {
  50: '#f6f6f7',
  100: '#ececef',
  200: '#dcdce1',
  300: '#c4c4cc',
  400: '#a1a1aa',
  500: '#52525b',
  600: '#34343b',
  700: '#26262b',
  800: '#1c1c20',
  900: '#141417',
  950: '#0b0b0d'
}

// One accent for selection, focus, links, and information.
const primary = { ...defaultColors.blue }

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
