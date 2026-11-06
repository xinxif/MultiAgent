/**
 * Tailwind 设计系统 —— Agent 暗色控制台。
 *
 * 统一设计语言：
 * 底色四级黑 + 酸性荧光绿主色，低饱和灰阶做层级。
 */
import type { Config } from 'tailwindcss';

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        /* ---- 底色四级黑（由深到浅） ---- */
        void: '#08090a', // 页面最底
        carbon: '#0f1011', // 面板
        obsidian: '#161718', // 卡片
        graphite: '#23252a', // 悬浮/输入框
        /* ---- 灰阶（由深到浅） ---- */
        smoke: '#383b3f', // 分隔线
        ash: '#62666d', // 最弱文字
        fog: '#8a8f98', // 次要文字
        mist: '#d0d6e0', // 正文
        bone: '#e5e5e6', // 标题
        paper: '#ffffff',
        /* ---- 功能色 ---- */
        lime: '#e4f222', // 主色：品牌荧光绿
        teal: '#02b8cc', // 回顾/信息
        coral: '#eb5757', // 错误/危险
        pulse: '#27a644', // 成功
        violet: '#6366f1', // 多智能体/特殊
      },
      fontFamily: {
        sans: [
          'Inter',
          'PingFang SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          '-apple-system',
          'sans-serif',
        ],
        mono: ['JetBrains Mono', 'Menlo', 'Consolas', 'monospace'],
      },
      /* 动效曲线：快进缓出，交互丝滑的底味 */
      transitionTimingFunction: {
        'out-expo': 'cubic-bezier(0.16, 1, 0.3, 1)',
      },
      keyframes: {
        'blink-caret': {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0' },
        },
      },
      animation: {
        'blink-caret': 'blink-caret 1s step-end infinite',
      },
    },
  },
  plugins: [],
} satisfies Config;
