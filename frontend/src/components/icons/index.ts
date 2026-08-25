// Inline SVG icon components — no external dependency
// All icons use currentColor, size controlled via width/height props (default 20px)

import { defineComponent, h } from 'vue'

const icon = (paths: string, viewBox = '0 0 24 24') =>
  defineComponent({
    props: { size: { type: Number, default: 20 } },
    setup(props) {
      return () =>
        h('svg', {
          xmlns: 'http://www.w3.org/2000/svg',
          width: props.size,
          height: props.size,
          viewBox,
          fill: 'none',
          stroke: 'currentColor',
          'stroke-width': '1.75',
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          innerHTML: paths,
          style: 'display:inline-block;vertical-align:middle;flex-shrink:0',
        })
    },
  })

// Navigation
export const IconChat = icon(
  '<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>'
)

export const IconContacts = icon(
  '<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>'
)

export const IconGroups = icon(
  '<path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/>'
)

export const IconLogout = icon(
  '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/>'
)

// Chat header
export const IconInfo = icon(
  '<circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>'
)

export const IconBack = icon(
  '<polyline points="15 18 9 12 15 6"/>'
)

// Input toolbar
export const IconPaperclip = icon(
  '<path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/>'
)

export const IconSend = icon(
  '<line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/>'
)

// File types
export const IconFile = icon(
  '<path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/>'
)

export const IconDownload = icon(
  '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>'
)

// Actions
export const IconPlus = icon(
  '<line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>'
)

export const IconSearch = icon(
  '<circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>'
)

export const IconClose = icon(
  '<line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>'
)

export const IconCheck = icon(
  '<polyline points="20 6 9 17 4 12"/>'
)

export const IconAlertCircle = icon(
  '<circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>'
)

// Toast
export const IconCheckCircle = icon(
  '<path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/>'
)

export const IconXCircle = icon(
  '<circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>'
)

// Group / member
export const IconUserPlus = icon(
  '<path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="20" y1="8" x2="20" y2="14"/><line x1="23" y1="11" x2="17" y2="11"/>'
)

export const IconUsers = icon(
  '<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>'
)

export const IconLoadMore = icon(
  '<polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 .49-4.5"/>'
)
