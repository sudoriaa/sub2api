/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // 主色调 - 二次元简约樱花粉
        primary: {
          50: '#fff1f5',
          100: '#ffe1eb',
          200: '#fbd5e2',
          300: '#f9a8c4',
          400: '#f68ab0',
          500: '#f4729b',
          600: '#e55b87',
          700: '#d64576',
          800: '#b33460',
          900: '#8f294d',
          950: '#4f1328'
        },
        // 辅助色 - 二次元清透浅天蓝
        accent: {
          50: '#eef8ff',
          100: '#dcf0ff',
          200: '#cde9fd',
          300: '#9cd4fb',
          400: '#7cc2f8',
          500: '#5eb5f7',
          600: '#42a3ee',
          700: '#2084d4',
          800: '#1a6bb0',
          900: '#18548c',
          950: '#122f4d'
        },
        // 深色模式背景 - 干净纯色靛蓝灰
        dark: {
          50: '#f4f7fc',
          100: '#e8edf7',
          200: '#d0d8e8',
          300: '#a5b0c7',
          400: '#7e8aa8',
          500: '#535f80',
          600: '#37405c',
          700: '#2a3147',
          800: '#1b1f2e',
          900: '#171b28',
          950: '#141722'
        }
      },
      fontFamily: {
        sans: [
          'PingFang SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          'Segoe UI',
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          'Roboto',
          'Helvetica Neue',
          'Arial',
          'sans-serif'
        ],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace']
      },
      boxShadow: {
        glass: '0 2px 8px rgba(37, 43, 59, 0.05)',
        'glass-sm': '0 1px 3px rgba(37, 43, 59, 0.04)',
        glow: '0 1px 3px rgba(244, 114, 155, 0.16)',
        'glow-lg': '0 2px 10px rgba(244, 114, 155, 0.2)',
        card: '0 1px 2px rgba(37, 43, 59, 0.04)',
        'card-hover': '0 2px 8px rgba(37, 43, 59, 0.06)',
        'inner-glow': 'inset 0 1px 0 rgba(255, 255, 255, 0.6)'
      },
      backgroundImage: {
        'gradient-radial': 'none',
        'gradient-primary': 'none',
        'gradient-dark': 'none',
        'gradient-glass': 'none',
        'mesh-gradient': 'none'
      },
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 2s linear infinite',
        glow: 'glow 2s ease-in-out infinite alternate'
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' }
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' }
        },
        glow: {
          '0%': { boxShadow: '0 0 0 1px rgba(244, 114, 155, 0.25)' },
          '100%': { boxShadow: '0 0 0 2px rgba(244, 114, 155, 0.4)' }
        }
      },
      backdropBlur: {
        xs: '2px'
      },
      borderRadius: {
        '4xl': '2rem'
      }
    }
  },
  plugins: []
}
