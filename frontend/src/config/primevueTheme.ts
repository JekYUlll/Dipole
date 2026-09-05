// PrimeVue 4 theme configuration.
//
// We bridge PrimeVue's Aura preset to Dipole's `--dp-*` design tokens so
// that:
//   1. Component chrome (Table borders, Drawer, Dialog, Button, Input) uses
//      Dipole colors instead of PrimeVue's default indigo/gray palette.
//   2. All rectangular components render with zero corner radius (BI/直角
//      规则 — see docs/notes/frontend-bi-redesign.md §4.1.1). Only Pill and
//      Chip retain 999px so their capsule semantics survive.
//   3. Density is tightened: buttons 32/28, inputs 32, table rows 34.
//
// The preset only sets what we care about — everything else falls back to
// Aura defaults. Consumers wire this into `PrimeVue` via `main.ts`.

import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'

export const dipolePreset = definePreset(Aura, {
  primitive: {
    borderRadius: {
      none: '0',
      xs: '0',
      sm: '0',
      md: '0',
      lg: '0',
      xl: '0',
    },
  },
  semantic: {
    // Density knobs: BI-tight but still tappable.
    formField: {
      paddingX: '10px',
      paddingY: '6px',
      borderRadius: '0',
      focusRing: {
        width: '2px',
        style: 'solid',
        offset: '0',
        shadow: 'none',
      },
    },
    content: {
      borderRadius: '0',
    },
    // Bridge PrimeVue's semantic color slots to Dipole tokens via CSS vars.
    // We keep the surface layers so Aura's default light/dark switching
    // still works, but the actual values are resolved at runtime from
    // --dp-* variables in styles/design-tokens.css.
    colorScheme: {
      light: {
        primary: {
          color: 'var(--dp-accent)',
          contrastColor: 'var(--dp-text-inverse)',
          hoverColor: 'var(--dp-accent-strong)',
          activeColor: 'var(--dp-accent-strong)',
        },
        surface: {
          0: 'var(--dp-surface)',
          50: 'var(--dp-surface)',
          100: 'var(--dp-surface-muted)',
          200: 'var(--dp-line)',
          300: 'var(--dp-line)',
          400: 'var(--dp-ink-faint)',
          500: 'var(--dp-ink-soft)',
          600: 'var(--dp-ink-soft)',
          700: 'var(--dp-ink)',
          800: 'var(--dp-ink)',
          900: 'var(--dp-rail)',
          950: 'var(--dp-rail)',
        },
        formField: {
          background: 'var(--dp-surface)',
          disabledBackground: 'var(--dp-surface-muted)',
          borderColor: 'var(--dp-line)',
          hoverBorderColor: 'var(--dp-ink-faint)',
          focusBorderColor: 'var(--dp-accent)',
          invalidBorderColor: 'var(--dp-danger)',
          color: 'var(--dp-ink)',
          placeholderColor: 'var(--dp-ink-faint)',
          floatLabelColor: 'var(--dp-ink-soft)',
        },
        text: {
          color: 'var(--dp-ink)',
          hoverColor: 'var(--dp-ink)',
          mutedColor: 'var(--dp-ink-soft)',
          hoverMutedColor: 'var(--dp-ink)',
        },
        content: {
          background: 'var(--dp-surface)',
          hoverBackground: 'var(--dp-surface-muted)',
          borderColor: 'var(--dp-line)',
          color: 'var(--dp-ink)',
          hoverColor: 'var(--dp-ink)',
        },
        overlay: {
          select: {
            background: 'var(--dp-surface)',
            borderColor: 'var(--dp-line)',
            color: 'var(--dp-ink)',
          },
          popover: {
            background: 'var(--dp-surface)',
            borderColor: 'var(--dp-line)',
            color: 'var(--dp-ink)',
          },
          modal: {
            background: 'var(--dp-surface)',
            borderColor: 'var(--dp-line)',
            color: 'var(--dp-ink)',
          },
        },
      },
    },
  },
})

// Options passed to `app.use(PrimeVue, primevueOptions)`.
export const primevueOptions = {
  theme: {
    preset: dipolePreset,
    options: {
      // Do not add a class prefix to injected CSS variables so we can
      // reference `--p-*` directly in scoped styles.
      prefix: 'p',
      // Dipole chrome is light-only. Aura's default `system` selector applies
      // dark form-field tokens (light text) when the OS is in dark mode, which
      // then paints native inputs/textareas as cream-on-cream after ChatView
      // overrides only the background. Pin light tokens unconditionally.
      darkModeSelector: false,
      cssLayer: {
        name: 'primevue',
        order: 'theme, base, primevue',
      },
    },
  },
  // We use Feather icons everywhere; disable PrimeVue's ripple to keep
  // the BI feel and reduce runtime work.
  ripple: false,
  // Chinese locale strings for a few common labels.
  locale: {
    accept: '确定',
    reject: '取消',
    choose: '选择',
    upload: '上传',
    cancel: '取消',
    completed: '完成',
    pending: '等待',
    fileSizeTypes: ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'],
    dayNames: ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六'],
    dayNamesShort: ['日', '一', '二', '三', '四', '五', '六'],
    dayNamesMin: ['日', '一', '二', '三', '四', '五', '六'],
    monthNames: ['一月', '二月', '三月', '四月', '五月', '六月', '七月', '八月', '九月', '十月', '十一月', '十二月'],
    monthNamesShort: ['1月', '2月', '3月', '4月', '5月', '6月', '7月', '8月', '9月', '10月', '11月', '12月'],
    today: '今天',
    weekHeader: '周',
    firstDayOfWeek: 1,
    dateFormat: 'yy-mm-dd',
    weak: '弱',
    medium: '中',
    strong: '强',
    passwordPrompt: '输入密码',
    emptyMessage: '没有可用选项',
    emptyFilterMessage: '没有匹配结果',
  },
}
