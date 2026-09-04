#!/usr/bin/env node

// Appends 4 top-level design frames to design/dipole-ui.pen that implement the
// "Chat + Agent Drawer" IA agreed in docs/notes/frontend-bi-redesign.md §3.1:
//
//   1. App Chrome v2                       — top bar with 4 tabs + 🤖 toggle
//   2. Chat + Agent Drawer · Live          — default view when Drawer opens
//   3. Chat + Agent Drawer · Tasks         — DataTable + selected-row sub-panel
//   4. State Patterns v1                   — Loading / Empty / Failed / Stale
//
// Idempotent: if a frame with the same name already exists it's replaced in
// place. Writes atomically via .tmp rename. All cornerRadius ∈ {0, 999};
// StatusPill/dot are the only non-rectangular shapes.

import { readFileSync, writeFileSync, renameSync } from 'node:fs'
import { resolve } from 'node:path'

const target = resolve(process.argv[2] ?? 'design/dipole-ui.pen')
const doc = JSON.parse(readFileSync(target, 'utf8'))
if (!doc || !Array.isArray(doc.children)) {
  console.error('pen file has no children[]')
  process.exit(1)
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------
let idCounter = 0
const id = () => `cad_${String(++idCounter).padStart(4, '0')}`

const clean = (o) => {
  const out = {}
  for (const k of Object.keys(o)) if (o[k] !== undefined) out[k] = o[k]
  return out
}

const text = (name, content, o = {}) => clean({
  type: 'text', id: id(), name, content,
  fontFamily: o.font ?? '$font-body',
  fontSize: o.size ?? 13,
  fontWeight: o.weight ?? 'normal',
  letterSpacing: o.tracking ?? 0,
  fill: o.fill ?? '$ink',
  width: o.width,
  textWrap: o.wrap,
  opacity: o.opacity,
})

const icon = (name, feather, size, fill, o = {}) => clean({
  type: 'icon', id: id(), name,
  library: 'feather', icon: feather,
  width: size, height: size, fill,
  opacity: o.opacity,
})

const frame = (name, o = {}) => clean({
  type: 'frame', id: id(), name,
  x: o.x, y: o.y,
  width: o.width, height: o.height, fill: o.fill,
  cornerRadius: o.cornerRadius, stroke: o.stroke, strokeWidth: o.strokeWidth,
  layout: o.layout, gap: o.gap, padding: o.padding,
  justifyContent: o.justifyContent, alignItems: o.alignItems,
  clip: o.clip, opacity: o.opacity,
  children: o.children ?? [],
})

const rect = (name, o = {}) => clean({
  type: 'rectangle', id: id(), name,
  width: o.width, height: o.height, fill: o.fill,
  cornerRadius: o.cornerRadius, stroke: o.stroke, strokeWidth: o.strokeWidth,
  opacity: o.opacity,
})

const divider = (name, orient = 'h') => rect(name, {
  width: orient === 'h' ? 'fill_container' : 1,
  height: orient === 'h' ? 1 : 'fill_container',
  fill: '$line', cornerRadius: 0,
})

const pill = (label, tone) => {
  const bg = tone === 'neutral' ? '$surface-muted' : `$${tone}-soft`
  const fg = tone === 'neutral' ? '$ink-soft' : (tone === 'agent' ? '$agent' : `$${tone}`)
  return frame(`${label} StatusPill`, {
    height: 20, fill: bg, cornerRadius: 999,
    layout: 'horizontal', gap: 6, padding: [0, 8, 0, 8], alignItems: 'center',
    children: [
      rect(`${label} StatusPill Dot`, { width: 6, height: 6, fill: fg, cornerRadius: 999 }),
      text(`${label} StatusPill Label`, label, {
        font: '$font-data', size: 10, weight: '600', tracking: 0.06, fill: fg,
      }),
    ],
  })
}

const button = (name, label, o = {}) => {
  const isPrimary = o.tone === 'primary'
  const isGhost = o.tone === 'ghost'
  const fill = isPrimary ? '$accent' : (isGhost ? undefined : '$surface')
  const stroke = isGhost ? '$line' : (isPrimary ? undefined : '$line')
  const fg = isPrimary ? '$text-inverse' : '$ink'
  return frame(`${name} Button`, {
    height: o.height ?? 32,
    fill, stroke, strokeWidth: stroke ? 1 : undefined,
    cornerRadius: 0,
    layout: 'horizontal', gap: 6, padding: [0, 12, 0, 12],
    alignItems: 'center', justifyContent: 'center',
    children: [
      ...(o.iconLeft ? [icon(`${name} Button Icon`, o.iconLeft, 14, fg)] : []),
      text(`${name} Button Label`, label, {
        font: '$font-body', size: 12, weight: '600', fill: fg,
      }),
    ],
  })
}

const banner = (name, tone, message, actionLabel) => {
  const bg = `$${tone}-soft`
  const fg = `$${tone}`
  const iconName = tone === 'danger' ? 'alert-circle'
    : tone === 'warning' ? 'alert-triangle'
    : tone === 'success' ? 'check-circle' : 'info'
  return frame(`${name} Banner`, {
    width: 'fill_container', height: 36, fill: bg, cornerRadius: 0,
    layout: 'horizontal', gap: 10, padding: [0, 12, 0, 12], alignItems: 'center',
    children: [
      icon(`${name} Banner Icon`, iconName, 14, fg),
      text(`${name} Banner Message`, message, {
        font: '$font-body', size: 12, weight: '500', fill: fg,
      }),
      frame(`${name} Banner Spacer`, { width: 'fill_container' }),
      ...(actionLabel ? [
        text(`${name} Banner Action`, actionLabel, {
          font: '$font-body', size: 12, weight: '700', fill: fg,
        }),
      ] : []),
      icon(`${name} Banner Close`, 'x', 14, fg, { opacity: 0.6 }),
    ],
  })
}

const skeletonRow = (name, cols) => frame(name, {
  width: 'fill_container', height: 32, fill: '$surface',
  layout: 'horizontal', gap: 0, alignItems: 'center',
  children: cols.map((w, i) => frame(`${name} Cell ${i}`, {
    width: w, height: 32, layout: 'vertical', justifyContent: 'center',
    padding: [0, 12, 0, 12],
    children: [rect(`${name} Bar ${i}`, {
      width: Math.max(40, w - 40), height: 8, fill: '$surface-muted', cornerRadius: 0,
    })],
  })),
})

const cell = (name, w, node, o = {}) => frame(`${name} Cell`, {
  width: w, height: 'fill_container',
  layout: 'horizontal', alignItems: 'center',
  padding: [0, 12, 0, 12], gap: 8,
  children: Array.isArray(node) ? node : [node],
})

const tableHeader = (cols) => frame('Table Header Row', {
  width: 'fill_container', height: 32, fill: '$surface-muted',
  layout: 'horizontal', gap: 0, alignItems: 'stretch',
  children: cols.map(([w, label]) => cell(`Header ${label}`, w,
    text(`Header ${label} Label`, label, {
      font: '$font-data', size: 10, weight: '700', tracking: 0.08, fill: '$ink-soft',
    })
  )),
})

const tableRow = (name, cells, o = {}) => frame(name, {
  width: 'fill_container', height: 32,
  fill: o.selected ? '$surface-muted' : '$surface',
  stroke: '$line', strokeWidth: 1,
  layout: 'horizontal', gap: 0, alignItems: 'stretch',
  children: [
    rect(`${name} Stripe`, {
      width: 2, height: 'fill_container',
      fill: o.stripe ?? (o.selected ? '$accent' : 'transparent'),
    }),
    ...cells,
  ],
})

const kv = (label, value, o = {}) => frame(`${label} KV`, {
  layout: 'horizontal', gap: 12, alignItems: 'baseline',
  padding: [4, 0, 4, 0],
  children: [
    text(`${label} KV Key`, label.toUpperCase(), {
      font: '$font-data', size: 10, weight: '600', tracking: 0.08, fill: '$ink-faint',
      width: 100,
    }),
    text(`${label} KV Value`, value, {
      font: o.mono ? '$font-data' : '$font-body',
      size: o.mono ? 11 : 12, fill: '$ink',
    }),
  ],
})

// Chat message bubble with avatar + meta + bubble content
// Own messages (o.own=true) flip to the right and use accent tint.
const message = (name, o = {}) => {
  const isOwn = o.own === true
  const bubbleFill = isOwn ? '$accent-soft' : '$surface'
  const bubbleStroke = isOwn ? '$accent-soft' : '$line'
  const bubbleTextFill = isOwn ? '$accent-strong' : '$ink'
  return frame(name, {
    width: 'fill_container',
    layout: 'horizontal', gap: 12, alignItems: 'flex-start',
    justifyContent: isOwn ? 'flex-end' : 'flex-start',
    children: (isOwn ? [
      // Body first, avatar last for own messages (right-aligned)
      frame(`${name} Body`, {
        layout: 'vertical', gap: 4, alignItems: 'flex-end',
        children: [
          frame(`${name} Meta`, {
            layout: 'horizontal', gap: 8, alignItems: 'baseline',
            children: [
              text(`${name} Time`, o.time, {
                font: '$font-data', size: 10, fill: '$ink-faint',
              }),
              text(`${name} Sender`, o.sender, {
                font: '$font-body', size: 12, weight: '700', fill: '$ink',
              }),
            ],
          }),
          frame(`${name} Bubble`, {
            fill: bubbleFill, stroke: bubbleStroke, strokeWidth: 1, cornerRadius: 0,
            padding: [10, 14, 10, 14],
            layout: 'vertical', gap: 6,
            children: [
              text(`${name} Content`, o.content, {
                font: '$font-body', size: 13, fill: bubbleTextFill, wrap: true,
                width: o.width ?? 400,
              }),
              ...(o.attachment ? [frame(`${name} Attachment`, {
                fill: '$surface', stroke: '$line', strokeWidth: 1, cornerRadius: 0,
                padding: 8, layout: 'horizontal', gap: 8, alignItems: 'center',
                children: [
                  icon(`${name} Attach Icon`, o.attachment.icon ?? 'file-text', 14, '$ink-soft'),
                  text(`${name} Attach Name`, o.attachment.name, {
                    font: '$font-data', size: 11, weight: '600', fill: '$ink',
                  }),
                  text(`${name} Attach Meta`, o.attachment.meta, {
                    font: '$font-data', size: 10, fill: '$ink-faint',
                  }),
                ],
              })] : []),
            ],
          }),
        ],
      }),
      frame(`${name} Avatar`, {
        width: 30, height: 30, fill: o.avatarFill ?? '$accent', cornerRadius: 999,
        layout: 'vertical', justifyContent: 'center', alignItems: 'center',
        children: [text(`${name} Avatar Initial`, o.initial, {
          font: '$font-data', size: 11, weight: '700', fill: '$text-inverse',
        })],
      }),
    ] : [
      // Standard left-aligned: avatar then bubble
      frame(`${name} Avatar`, {
        width: 30, height: 30, fill: o.avatarFill ?? '$rail-soft', cornerRadius: 999,
        layout: 'vertical', justifyContent: 'center', alignItems: 'center',
        children: [text(`${name} Avatar Initial`, o.initial, {
          font: '$font-data', size: 11, weight: '700', fill: '$text-inverse',
        })],
      }),
      frame(`${name} Body`, {
        layout: 'vertical', gap: 4,
        children: [
          frame(`${name} Meta`, {
            layout: 'horizontal', gap: 8, alignItems: 'baseline',
            children: [
              text(`${name} Sender`, o.sender, {
                font: '$font-body', size: 12, weight: '700', fill: '$ink',
              }),
              text(`${name} Time`, o.time, {
                font: '$font-data', size: 10, fill: '$ink-faint',
              }),
            ],
          }),
          frame(`${name} Bubble`, {
            fill: bubbleFill, stroke: bubbleStroke, strokeWidth: 1, cornerRadius: 0,
            padding: [10, 14, 10, 14],
            layout: 'vertical', gap: 6,
            children: [
              text(`${name} Content`, o.content, {
                font: '$font-body', size: 13, fill: bubbleTextFill, wrap: true,
                width: o.width ?? 400,
              }),
              ...(o.attachment ? [frame(`${name} Attachment`, {
                fill: '$surface-muted', stroke: '$line', strokeWidth: 1, cornerRadius: 0,
                padding: 8, layout: 'horizontal', gap: 8, alignItems: 'center',
                children: [
                  icon(`${name} Attach Icon`, o.attachment.icon ?? 'file-text', 14, '$ink-soft'),
                  text(`${name} Attach Name`, o.attachment.name, {
                    font: '$font-data', size: 11, weight: '600', fill: '$ink',
                  }),
                  text(`${name} Attach Meta`, o.attachment.meta, {
                    font: '$font-data', size: 10, fill: '$ink-faint',
                  }),
                ],
              })] : []),
            ],
          }),
        ],
      }),
    ]),
  })
}

// Date separator: -------- TODAY --------
const dateSeparator = (name, label) => frame(`${name} Separator`, {
  width: 'fill_container', layout: 'horizontal', gap: 12, alignItems: 'center',
  padding: [8, 0, 8, 0],
  children: [
    rect(`${name} Line Left`, { width: 'fill_container', height: 1, fill: '$line', cornerRadius: 0 }),
    text(`${name} Label`, label.toUpperCase(), {
      font: '$font-data', size: 10, weight: '700', tracking: 0.12, fill: '$ink-faint',
    }),
    rect(`${name} Line Right`, { width: 'fill_container', height: 1, fill: '$line', cornerRadius: 0 }),
  ],
})

// Inline system hint (agent picked up task, etc.)
const systemHint = (name, o = {}) => frame(name, {
  layout: 'horizontal', gap: 10, alignItems: 'center',
  padding: [8, 12, 8, 12],
  fill: o.fill ?? '$agent-soft', cornerRadius: 0,
  stroke: o.stroke ?? '$agent', strokeWidth: 1,
  children: [
    icon(`${name} Icon`, o.icon ?? 'cpu', 14, o.tone ?? '$agent'),
    text(`${name} Content`, o.content, {
      font: '$font-data', size: 11, weight: '600', fill: o.tone ?? '$agent',
    }),
    frame(`${name} Spacer`, { width: 'fill_container' }),
    ...(o.action ? [text(`${name} Action`, o.action, {
      font: '$font-body', size: 11, weight: '700', fill: o.tone ?? '$agent',
    })] : []),
  ],
})

// Timeline event with left-side vertical connecting line
// Renders as a horizontal strip: [24px marker column with dot+line] [body]
// Sequential rows share a continuous line effect visually because their
// marker columns are the same width and the dots sit centered in it.
const timelineEvent = (name, o = {}) => frame(name, {
  width: 'fill_container', layout: 'horizontal', gap: 12, alignItems: 'stretch',
  children: [
    // Marker column: vertical line + dot on top of it
    frame(`${name} Marker`, {
      width: 16, layout: 'vertical', alignItems: 'center', gap: 0, padding: [6, 0, 0, 0],
      children: [
        rect(`${name} Dot`, {
          width: 8, height: 8, fill: o.tone ? `$${o.tone}` : '$ink-faint', cornerRadius: 999,
        }),
        rect(`${name} Line`, {
          width: 2, height: 'fill_container', fill: '$line', cornerRadius: 0,
          opacity: o.isLast ? 0 : 1,
        }),
      ],
    }),
    // Body
    frame(`${name} Body`, {
      width: 'fill_container', layout: 'vertical', gap: 2,
      padding: [0, 0, 12, 0],
      children: [
        frame(`${name} Head`, {
          layout: 'horizontal', gap: 8, alignItems: 'baseline',
          children: [
            text(`${name} Kind`, o.kind, {
              font: '$font-data', size: 11, weight: '700', fill: '$ink',
            }),
            text(`${name} At`, o.at, {
              font: '$font-data', size: 10, fill: '$ink-faint',
            }),
            ...(o.badge ? [pill(o.badge, o.tone ?? 'agent')] : []),
          ],
        }),
        text(`${name} Detail`, o.detail, {
          font: '$font-body', size: 12, fill: '$ink-soft', wrap: true, width: o.width ?? 380,
        }),
      ],
    }),
  ],
})

const sectionHeader = (name, label, count, action) => frame(`${name} Section Header`, {
  width: 'fill_container', height: 28,
  layout: 'horizontal', gap: 8, alignItems: 'center',
  padding: [0, 0, 0, 0],
  children: [
    rect(`${name} Section Accent`, {
      width: 3, height: 14, fill: '$accent', cornerRadius: 0,
    }),
    text(`${name} Section Label`, label.toUpperCase(), {
      font: '$font-data', size: 10, weight: '800', tracking: 0.12, fill: '$ink',
    }),
    ...(count !== undefined ? [frame(`${name} Section Count Frame`, {
      height: 16, fill: '$surface-muted', cornerRadius: 0,
      padding: [0, 6, 0, 6], layout: 'horizontal', alignItems: 'center',
      children: [text(`${name} Section Count`, String(count), {
        font: '$font-data', size: 10, weight: '700', fill: '$ink-soft',
      })],
    })] : []),
    frame(`${name} Section Spacer`, { width: 'fill_container' }),
    ...(action ? [text(`${name} Section Action`, action, {
      font: '$font-body', size: 11, weight: '700', fill: '$accent',
    })] : []),
  ],
})

// -----------------------------------------------------------------------------
// Reusable chrome pieces
// -----------------------------------------------------------------------------

// Top bar: DIPOLE brand + tab switcher + right actions with 🤖 badge
const topBar = (o = {}) => frame('App Top Bar', {
  width: 'fill_container', height: 48, fill: '$rail', cornerRadius: 0,
  layout: 'horizontal', padding: [0, 20, 0, 20], gap: 32, alignItems: 'center',
  children: [
    // Brand
    frame('Brand', {
      layout: 'horizontal', gap: 8, alignItems: 'center',
      children: [
        rect('Brand Dot', { width: 10, height: 10, fill: '$accent', cornerRadius: 999 }),
        text('Brand Wordmark', 'DIPOLE', {
          font: '$font-display', size: 15, weight: '700', tracking: 0.12, fill: '$text-inverse',
        }),
      ],
    }),
    // Workspace tabs (Chat / Directory / Settings)
    frame('Workspace Tabs', {
      layout: 'horizontal', gap: 24, alignItems: 'center',
      children: [
        text('Chat Tab', 'Chat', {
          font: '$font-body', size: 13,
          weight: o.activeTab === 'chat' ? '700' : 'normal',
          fill: o.activeTab === 'chat' ? '$text-inverse' : '$ink-faint',
        }),
        text('Directory Tab', 'Directory', { font: '$font-body', size: 13, fill: '$ink-faint' }),
        text('Settings Tab', 'Settings', { font: '$font-body', size: 13, fill: '$ink-faint' }),
      ],
    }),
    frame('Top Bar Spacer', { width: 'fill_container' }),
    // Right group: search + 🤖 + settings gear + avatar
    frame('Top Bar Right', {
      layout: 'horizontal', gap: 16, alignItems: 'center',
      children: [
        icon('Top Bar Search Icon', 'search', 16, '$ink-faint'),
        // Agent toggle button — the ONLY entry point to Agent
        frame('Agent Toggle', {
          layout: 'horizontal', gap: 6, alignItems: 'center',
          padding: [4, 8, 4, 8],
          fill: o.agentActive ? '$rail-soft' : undefined,
          cornerRadius: 0,
          children: [
            icon('Agent Toggle Icon', 'cpu', 16,
              o.agentActive ? '$text-inverse' : '$ink-faint'),
            // Red pending badge
            ...(o.pending ? [frame('Agent Toggle Badge', {
              height: 16, fill: '$danger', cornerRadius: 999,
              padding: [0, 5, 0, 5],
              layout: 'horizontal', alignItems: 'center', justifyContent: 'center',
              children: [text('Agent Badge Count', String(o.pending), {
                font: '$font-data', size: 10, weight: '700', fill: '$text-inverse',
              })],
            })] : []),
          ],
        }),
        icon('Top Bar Settings Icon', 'settings', 16, '$ink-faint'),
        frame('Top Bar Avatar', {
          width: 28, height: 28, fill: '$accent', cornerRadius: 999,
          layout: 'vertical', justifyContent: 'center', alignItems: 'center',
          children: [text('Avatar Initials', 'EJ', {
            font: '$font-data', size: 10, weight: '700', fill: '$text-inverse',
          })],
        }),
      ],
    }),
  ],
})

// Conversation sidebar (left of chat)
const conversationSidebar = () => frame('Conversation Sidebar', {
  width: 260, height: 'fill_container', fill: '$surface',
  stroke: '$line', strokeWidth: 1, cornerRadius: 0,
  layout: 'vertical', gap: 0,
  children: [
    // Sidebar header
    frame('Sidebar Header', {
      width: 'fill_container', height: 40,
      layout: 'horizontal', gap: 8, alignItems: 'center',
      padding: [0, 16, 0, 16],
      children: [
        text('Sidebar Title', 'CONVERSATIONS', {
          font: '$font-data', size: 10, weight: '700', tracking: 0.1, fill: '$ink-soft',
        }),
        frame('Sidebar Header Spacer', { width: 'fill_container' }),
        icon('Sidebar New Chat', 'edit', 14, '$ink-faint'),
      ],
    }),
    divider('Sidebar Divider'),
    // Search
    frame('Sidebar Search', {
      width: 'fill_container', height: 36,
      layout: 'horizontal', gap: 8, alignItems: 'center',
      padding: [0, 16, 0, 16],
      children: [
        icon('Sidebar Search Icon', 'search', 14, '$ink-faint'),
        text('Sidebar Search Placeholder', '搜索会话…', {
          font: '$font-body', size: 12, fill: '$ink-faint',
        }),
      ],
    }),
    divider('Sidebar Search Divider'),
    // Conversation rows
    ...[
      { name: 'Alice',           preview: '我看下这个 diff',       time: '10:34', unread: 0, active: true },
      { name: 'Bob',             preview: '好的',                  time: '10:12', unread: 0 },
      { name: 'project-alpha',   preview: 'agent 已产出报告',      time: '09:58', unread: 2 },
      { name: 'agent-uai',       preview: '需要审批的任务 (1)',    time: '09:44', unread: 1 },
      { name: 'devops-review',   preview: '@Evan 请看时间线',      time: 'Wed',   unread: 0 },
      { name: 'security-audit',  preview: '扫描完成',              time: 'Wed',   unread: 0 },
      { name: 'Charlie',         preview: '晚点聊',                time: 'Tue',   unread: 0 },
    ].map(c => frame(`Convo ${c.name}`, {
      width: 'fill_container', height: 56,
      fill: c.active ? '$surface-muted' : '$surface',
      layout: 'horizontal', gap: 12, alignItems: 'center',
      padding: [0, 16, 0, 16],
      children: [
        frame(`Convo ${c.name} Avatar`, {
          width: 32, height: 32,
          fill: c.name.startsWith('agent') || c.name.startsWith('project') ? '$agent' : '$rail-soft',
          cornerRadius: 999,
          layout: 'vertical', justifyContent: 'center', alignItems: 'center',
          children: [text(`Convo ${c.name} Avatar Initial`, c.name[0].toUpperCase(), {
            font: '$font-data', size: 11, weight: '700', fill: '$text-inverse',
          })],
        }),
        frame(`Convo ${c.name} Body`, {
          width: 'fill_container', layout: 'vertical', gap: 2,
          children: [
            frame(`Convo ${c.name} Row1`, {
              width: 'fill_container', layout: 'horizontal', alignItems: 'baseline',
              children: [
                text(`Convo ${c.name} Name`, c.name, {
                  font: '$font-body', size: 13, weight: '600', fill: '$ink',
                }),
                frame(`Convo ${c.name} Spacer`, { width: 'fill_container' }),
                text(`Convo ${c.name} Time`, c.time, {
                  font: '$font-data', size: 10, fill: '$ink-faint',
                }),
              ],
            }),
            frame(`Convo ${c.name} Row2`, {
              width: 'fill_container', layout: 'horizontal', alignItems: 'center', gap: 6,
              children: [
                text(`Convo ${c.name} Preview`, c.preview, {
                  font: '$font-body', size: 12, fill: '$ink-faint',
                }),
                frame(`Convo ${c.name} Spacer2`, { width: 'fill_container' }),
                ...(c.unread ? [frame(`Convo ${c.name} Badge`, {
                  height: 16, fill: '$accent', cornerRadius: 999,
                  padding: [0, 6, 0, 6],
                  layout: 'horizontal', alignItems: 'center', justifyContent: 'center',
                  children: [text(`Convo ${c.name} Badge Count`, String(c.unread), {
                    font: '$font-data', size: 10, weight: '700', fill: '$text-inverse',
                  })],
                })] : []),
              ],
            }),
          ],
        }),
      ],
    })),
  ],
})

// Chat main body: header, message stream, composer
const chatMainBody = (width) => frame('Chat Main Body', {
  width, height: 'fill_container', fill: '$canvas',
  layout: 'vertical', gap: 0,
  children: [
    // Chat header
    frame('Chat Header', {
      width: 'fill_container', height: 48, fill: '$surface',
      stroke: '$line', strokeWidth: 1, cornerRadius: 0,
      layout: 'horizontal', gap: 12, alignItems: 'center',
      padding: [0, 24, 0, 24],
      children: [
        frame('Chat Header Avatar', {
          width: 28, height: 28, fill: '$rail-soft', cornerRadius: 999,
          layout: 'vertical', justifyContent: 'center', alignItems: 'center',
          children: [text('Chat Header Avatar Initial', 'A', {
            font: '$font-data', size: 11, weight: '700', fill: '$text-inverse',
          })],
        }),
        frame('Chat Header Info', {
          layout: 'vertical', gap: 0,
          children: [
            text('Chat Header Name', 'Alice', {
              font: '$font-display', size: 14, weight: '700', fill: '$ink',
            }),
            frame('Chat Header Meta', {
              layout: 'horizontal', gap: 6, alignItems: 'center',
              children: [
                rect('Chat Header Online Dot', { width: 6, height: 6, fill: '$success', cornerRadius: 999 }),
                text('Chat Header Status', 'online · e2ee', {
                  font: '$font-data', size: 10, fill: '$ink-faint',
                }),
              ],
            }),
          ],
        }),
        frame('Chat Header Spacer', { width: 'fill_container' }),
        icon('Chat Header Search', 'search', 16, '$ink-faint'),
        icon('Chat Header More', 'more-horizontal', 16, '$ink-faint'),
      ],
    }),
    // Message stream — 6 messages + TODAY separator + 2 system hints
    frame('Chat Message Stream', {
      width: 'fill_container', height: 'fill_container',
      layout: 'vertical', gap: 16,
      padding: [24, 40, 24, 40],
      children: [
        dateSeparator('Chat Sep Today', '今天 · Sep 3'),
        message('Message 1', {
          initial: 'A', avatarFill: '$rail-soft', sender: 'Alice', time: '10:02',
          content: '这个 pull request 需要你 review 一下，我加了新的重试策略。',
          attachment: { icon: 'git-pull-request', name: 'dipole/mr/487',
            meta: '+128 −34 · Ready for review' },
          width: 460,
        }),
        message('Message 2', {
          own: true,
          initial: 'EJ', avatarFill: '$accent', sender: 'Evan', time: '10:03',
          content: '收到，我让 agent-uai 先跑一次 diff 分析。',
          width: 380,
        }),
        // Agent hint bubble
        systemHint('Message 3 System', {
          icon: 'cpu', tone: '$agent',
          content: 'agent-uai 已接收任务 task:854035b7… · RUNNING',
          action: '在右侧 Drawer 查看 →',
        }),
        message('Message 4', {
          initial: 'A', avatarFill: '$rail-soft', sender: 'Alice', time: '10:04',
          content: '👍 我等 agent 结果再一起看',
          width: 460,
        }),
        // Second agent hint: waiting for input
        systemHint('Message 5 System', {
          icon: 'alert-triangle', tone: '$warning', fill: '$warning-soft', stroke: '$warning',
          content: 'agent-uai 需要 owner 确认扫描范围（elicitation）',
          action: '处理 →',
        }),
        message('Message 6', {
          own: true,
          initial: 'EJ', avatarFill: '$accent', sender: 'Evan', time: '10:05',
          content: '好，我在 Drawer 里直接答，你等一下',
          width: 380,
        }),
      ],
    }),
    // Composer
    frame('Composer', {
      width: 'fill_container', height: 88, fill: '$surface',
      stroke: '$line', strokeWidth: 1, cornerRadius: 0,
      layout: 'vertical', gap: 8,
      padding: [12, 24, 12, 24],
      children: [
        frame('Composer Input', {
          width: 'fill_container', height: 40,
          fill: '$canvas', stroke: '$line', strokeWidth: 1, cornerRadius: 0,
          layout: 'horizontal', gap: 8, alignItems: 'center',
          padding: [0, 12, 0, 12],
          children: [
            text('Composer Placeholder', '输入消息，⌘Enter 发送，@ 触发 agent…', {
              font: '$font-body', size: 13, fill: '$ink-faint',
            }),
          ],
        }),
        frame('Composer Actions', {
          width: 'fill_container', layout: 'horizontal', gap: 8, alignItems: 'center',
          children: [
            icon('Composer Attach', 'paperclip', 14, '$ink-faint'),
            icon('Composer Emoji', 'smile', 14, '$ink-faint'),
            icon('Composer At', 'at-sign', 14, '$ink-faint'),
            frame('Composer Actions Spacer', { width: 'fill_container' }),
            text('Composer Hint', '⌘+Enter', {
              font: '$font-data', size: 10, fill: '$ink-faint',
            }),
            button('Composer Send', '发送', { tone: 'primary', iconLeft: 'send' }),
          ],
        }),
      ],
    }),
  ],
})

// Status bar (bottom, 28px)
const statusBar = (text2) => frame('App Status Bar', {
  width: 'fill_container', height: 28, fill: '$rail-soft', cornerRadius: 0,
  layout: 'horizontal', gap: 16, alignItems: 'center',
  padding: [0, 20, 0, 20],
  children: [
    text('Status Env', text2 ?? 'SHADOW · DIRECT_TARGET · deepseek/v4-flash', {
      font: '$font-data', size: 10, weight: '600', fill: '$ink-faint',
    }),
    frame('Status Bar Spacer', { width: 'fill_container' }),
    frame('Status Connected Group', {
      layout: 'horizontal', gap: 6, alignItems: 'center',
      children: [
        rect('Status Connected Dot', { width: 6, height: 6, fill: '$success', cornerRadius: 999 }),
        text('Status Connected', 'ws · connected', {
          font: '$font-data', size: 10, fill: '$ink-faint',
        }),
      ],
    }),
    text('Status Time', '22:11:04', {
      font: '$font-data', size: 10, fill: '$ink-faint',
    }),
  ],
})

// -----------------------------------------------------------------------------
// Agent Drawer content — one function per view
// -----------------------------------------------------------------------------

const drawerHeader = (subtitle, tabActive) => frame('Drawer Header', {
  width: 'fill_container', layout: 'vertical', gap: 0,
  children: [
    // Top row: title + close
    frame('Drawer Top', {
      width: 'fill_container', height: 48,
      layout: 'horizontal', gap: 8, alignItems: 'center',
      padding: [0, 16, 0, 16],
      children: [
        icon('Drawer Icon', 'cpu', 16, '$agent'),
        text('Drawer Title', 'AGENT', {
          font: '$font-data', size: 11, weight: '700', tracking: 0.1, fill: '$ink',
        }),
        ...(subtitle ? [
          text('Drawer Sep', '·', {
            font: '$font-body', size: 12, fill: '$ink-faint',
          }),
          text('Drawer Subtitle', subtitle, {
            font: '$font-body', size: 12, fill: '$ink-soft',
          }),
        ] : []),
        frame('Drawer Header Spacer', { width: 'fill_container' }),
        icon('Drawer Settings', 'settings', 14, '$ink-faint'),
        icon('Drawer Close', 'x', 16, '$ink-soft'),
      ],
    }),
    divider('Drawer Header Divider'),
    // Tab bar
    frame('Drawer Tab Bar', {
      width: 'fill_container', height: 40,
      layout: 'horizontal', gap: 4, alignItems: 'stretch',
      padding: [0, 12, 0, 12],
      children: ['live', 'tasks', 'artifacts', 'definitions', 'subscriptions', 'memories'].map(t => {
        const active = t === tabActive
        const label = { live: 'Live', tasks: 'Tasks', artifacts: 'Artifacts', definitions: 'Defs', subscriptions: 'Subs', memories: 'Memories' }[t]
        return frame(`Drawer Tab ${t}`, {
          layout: 'vertical', gap: 0, justifyContent: 'center', alignItems: 'center',
          padding: [0, 10, 0, 10],
          children: [
            frame(`Drawer Tab ${t} Inner`, {
              layout: 'horizontal', gap: 4, alignItems: 'center', height: 38,
              children: [
                text(`Drawer Tab ${t} Label`, label, {
                  font: '$font-body', size: 12,
                  weight: active ? '700' : '500',
                  fill: active ? '$ink' : '$ink-faint',
                }),
                ...(t === 'live' ? [frame(`Drawer Tab ${t} Badge`, {
                  height: 14, fill: '$danger', cornerRadius: 999,
                  padding: [0, 4, 0, 4],
                  layout: 'horizontal', alignItems: 'center', justifyContent: 'center',
                  children: [text(`Drawer Tab ${t} Badge Count`, '3', {
                    font: '$font-data', size: 9, weight: '700', fill: '$text-inverse',
                  })],
                })] : []),
              ],
            }),
            rect(`Drawer Tab ${t} Underline`, {
              width: 'fill_container', height: 2,
              fill: active ? '$accent' : 'transparent',
            }),
          ],
        })
      }),
    }),
    divider('Drawer Tab Divider'),
  ],
})

// Live view: three sections
const drawerLiveContent = () => frame('Drawer Live Content', {
  width: 'fill_container', height: 'fill_container',
  layout: 'vertical', gap: 20,
  padding: [16, 20, 16, 20],
  children: [
    // Inline stale banner
    banner('Live Stale', 'warning', 'Definition v3 已失效', '刷新'),

    // Section: Current task
    frame('Live Current Task Section', {
      width: 'fill_container', layout: 'vertical', gap: 10,
      children: [
        sectionHeader('Live Current', '当前会话任务', undefined, undefined),
        frame('Live Current Task Card', {
          width: 'fill_container', fill: '$surface',
          stroke: '$line', strokeWidth: 1, cornerRadius: 0,
          layout: 'vertical', gap: 12, padding: 16,
          children: [
            // Card top: id + pill + metrics
            frame('Live Task Head', {
              width: 'fill_container', layout: 'horizontal', gap: 8, alignItems: 'center',
              children: [
                text('Live Task Id', 'task:854035b7…', {
                  font: '$font-data', size: 12, weight: '700', fill: '$ink',
                }),
                pill('RUNNING', 'agent'),
                frame('Live Task Head Spacer', { width: 'fill_container' }),
                text('Live Task Elapsed', '34s · 3 events', {
                  font: '$font-data', size: 10, fill: '$ink-faint',
                }),
              ],
            }),
            // Timeline with connecting line
            frame('Live Task Timeline', {
              width: 'fill_container', layout: 'vertical', gap: 0,
              children: [
                timelineEvent('Live TL 0', {
                  kind: 'cap.assign',   at: '10:02:03', tone: 'success',
                  detail: 'assign_capability(agent-uai) · 已授予权限 chat.read',
                  width: 340,
                }),
                timelineEvent('Live TL 1', {
                  kind: 'cap.execute',  at: '10:02:11', tone: 'success',
                  detail: 'run_workflow(diff-analysis) · 拉取 dipole/mr/487',
                  width: 340,
                }),
                timelineEvent('Live TL 2', {
                  kind: 'cap.reply',    at: '10:02:34', tone: 'agent',
                  detail: 'post_message(#conv-alice) · 等待模型返回',
                  isLast: true, width: 340,
                }),
              ],
            }),
          ],
        }),
      ],
    }),

    // Section: Pending to me
    frame('Live Pending Section', {
      width: 'fill_container', layout: 'vertical', gap: 10,
      children: [
        sectionHeader('Live Pending', '待我处理', 2),
        ...[
          { id: 'task:71fa2e91…', kind: 'INPUT',    tone: 'warning', label: '需要扫描范围确认' },
          { id: 'task:9820cd4a…', kind: 'APPROVAL', tone: 'danger',  label: 'sec-scan 结果需要签名' },
        ].map(p => frame(`Live Pending ${p.id}`, {
          width: 'fill_container', height: 52,
          fill: '$surface', stroke: '$line', strokeWidth: 1, cornerRadius: 0,
          layout: 'horizontal', gap: 12, alignItems: 'center',
          padding: [0, 14, 0, 0],
          children: [
            rect(`Live Pending ${p.id} Stripe`, {
              width: 3, height: 'fill_container', fill: `$${p.tone}`, cornerRadius: 0,
            }),
            frame(`Live Pending ${p.id} Body`, {
              layout: 'vertical', gap: 3, width: 'fill_container',
              padding: [0, 0, 0, 12],
              children: [
                frame(`Live Pending ${p.id} Top`, {
                  layout: 'horizontal', gap: 8, alignItems: 'center',
                  children: [
                    pill(p.kind, p.tone),
                    text(`Live Pending ${p.id} Id`, p.id, {
                      font: '$font-data', size: 11, weight: '600', fill: '$ink',
                    }),
                  ],
                }),
                text(`Live Pending ${p.id} Label`, p.label, {
                  font: '$font-body', size: 12, fill: '$ink-soft',
                }),
              ],
            }),
            text(`Live Pending ${p.id} Action`, p.kind === 'INPUT' ? '处理 →' : '审批 →', {
              font: '$font-body', size: 12, weight: '700', fill: '$accent',
            }),
          ],
        })),
      ],
    }),

    // Section: Recent artifacts
    frame('Live Artifacts Section', {
      width: 'fill_container', layout: 'vertical', gap: 10,
      children: [
        sectionHeader('Live Recent', '最近产物', 3, '全部 →'),
        frame('Live Artifacts List', {
          width: 'fill_container', fill: '$surface',
          stroke: '$line', strokeWidth: 1, cornerRadius: 0,
          layout: 'vertical', gap: 0,
          children: [
            ['report-2026-09-03.pdf',   '124 KB', '10:02', 'file-text'],
            ['diff-summary.md',         '8.4 KB', '09:44', 'file-text'],
            ['security-scan.json',      '32 KB',  '09:12', 'shield'],
          ].map(([name, size, time, iconName], i, arr) => frame(`Live Artifact ${i}`, {
            width: 'fill_container', height: 36,
            layout: 'horizontal', gap: 10, alignItems: 'center',
            padding: [0, 14, 0, 14],
            stroke: i < arr.length - 1 ? '$line' : undefined,
            strokeWidth: i < arr.length - 1 ? 1 : undefined,
            children: [
              icon(`Live Artifact ${i} Icon`, iconName, 14, '$ink-soft'),
              text(`Live Artifact ${i} Name`, name, {
                font: '$font-data', size: 11, weight: '600', fill: '$ink',
              }),
              frame(`Live Artifact ${i} Spacer`, { width: 'fill_container' }),
              text(`Live Artifact ${i} Size`, size, {
                font: '$font-data', size: 10, fill: '$ink-faint',
              }),
              text(`Live Artifact ${i} Time`, time, {
                font: '$font-data', size: 10, fill: '$ink-faint',
              }),
            ],
          })),
        }),
      ],
    }),
  ],
})

// Tasks view: DataTable + selected row sub-panel
const drawerTasksContent = () => frame('Drawer Tasks Content', {
  width: 'fill_container', height: 'fill_container',
  layout: 'vertical', gap: 0,
  children: [
    // Sub toolbar
    frame('Tasks Sub Toolbar', {
      width: 'fill_container', height: 40, fill: '$surface',
      layout: 'horizontal', gap: 8, alignItems: 'center',
      padding: [0, 16, 0, 16],
      children: [
        text('Tasks Count', '任务 · 12', {
          font: '$font-body', size: 12, weight: '600', fill: '$ink',
        }),
        text('Tasks Filter', '按状态: 全部 ▾', {
          font: '$font-body', size: 12, fill: '$ink-faint',
        }),
        frame('Tasks Sub Toolbar Spacer', { width: 'fill_container' }),
        button('Tasks Create', '+ 创建任务', { tone: 'primary', iconLeft: 'plus' }),
      ],
    }),
    divider('Tasks Sub Toolbar Divider'),
    // Split: table (left) + sub-panel (right)
    frame('Tasks Split', {
      width: 'fill_container', height: 'fill_container',
      layout: 'horizontal', gap: 0, alignItems: 'stretch',
      children: [
        // Left: DataTable
        frame('Tasks Table Region', {
          width: 240, height: 'fill_container',
          layout: 'vertical', gap: 0,
          stroke: '$line', strokeWidth: 1, cornerRadius: 0,
          children: [
            frame('Tasks List Table', {
              width: 'fill_container',
              layout: 'vertical', gap: 0,
              children: [
                frame('Tasks List Header', {
                  width: 'fill_container', height: 32, fill: '$surface-muted',
                  layout: 'horizontal', gap: 0, padding: [0, 12, 0, 12],
                  alignItems: 'center',
                  children: [
                    text('Tasks List Header Status', 'STATUS', {
                      font: '$font-data', size: 9, weight: '700', tracking: 0.08, fill: '$ink-soft',
                      width: 72,
                    }),
                    text('Tasks List Header Id', 'TASK', {
                      font: '$font-data', size: 9, weight: '700', tracking: 0.08, fill: '$ink-soft',
                    }),
                  ],
                }),
                ...[
                  { id: 'task:854035b7…', st: 'RUNNING',      tone: 'agent',  active: true  },
                  { id: 'task:71fa2e91…', st: 'AWAIT_INPUT',  tone: 'warning' },
                  { id: 'task:9820cd4a…', st: 'AWAIT_APPROV', tone: 'danger'  },
                  { id: 'task:5b1c9f2d…', st: 'SUCCESS',      tone: 'success' },
                  { id: 'task:3a04b17c…', st: 'FAILED',       tone: 'danger'  },
                  { id: 'task:c81efeb1…', st: 'SUCCESS',      tone: 'success' },
                ].map(t => frame(`Tasks List Row ${t.id}`, {
                  width: 'fill_container', height: 34,
                  fill: t.active ? '$surface-muted' : '$surface',
                  layout: 'horizontal', gap: 0, alignItems: 'center',
                  padding: [0, 12, 0, 12],
                  children: [
                    frame(`Tasks List Row ${t.id} Status`, {
                      width: 72, layout: 'horizontal', alignItems: 'center', gap: 6,
                      children: [
                        rect(`Tasks List Row ${t.id} Dot`, {
                          width: 6, height: 6, fill: `$${t.tone}`, cornerRadius: 999,
                        }),
                        text(`Tasks List Row ${t.id} St`, t.st, {
                          font: '$font-data', size: 9, weight: '700', tracking: 0.05, fill: `$${t.tone}`,
                        }),
                      ],
                    }),
                    text(`Tasks List Row ${t.id} Id`, t.id, {
                      font: '$font-data', size: 10, weight: t.active ? '700' : '500', fill: '$ink',
                    }),
                  ],
                })),
              ],
            }),
          ],
        }),
        // Right: sub-panel
        frame('Tasks Sub Panel', {
          width: 'fill_container', height: 'fill_container', fill: '$surface',
          layout: 'vertical', gap: 0,
          children: [
            // Sub-panel header
            frame('Sub Panel Header', {
              width: 'fill_container', height: 40, fill: '$surface',
              layout: 'horizontal', gap: 8, alignItems: 'center',
              padding: [0, 16, 0, 16],
              children: [
                text('Sub Task Id', 'task:854035b7…', {
                  font: '$font-data', size: 12, weight: '700', fill: '$ink',
                }),
                pill('RUNNING', 'agent'),
                text('Sub Elapsed', '· 34s', {
                  font: '$font-data', size: 10, fill: '$ink-faint',
                }),
                frame('Sub Header Spacer', { width: 'fill_container' }),
                icon('Sub More', 'more-horizontal', 14, '$ink-faint'),
              ],
            }),
            // Sub tabs
            frame('Sub Tabs', {
              width: 'fill_container', height: 32,
              layout: 'horizontal', gap: 4, alignItems: 'stretch',
              padding: [0, 12, 0, 12],
              children: [
                { t: 'Timeline', active: true },
                { t: 'Input', dot: true },
                { t: 'Approval', dot: true },
                { t: 'Artifacts' },
                { t: 'Boundary' },
              ].map(t => frame(`Sub Tab ${t.t}`, {
                layout: 'vertical', gap: 0, justifyContent: 'center', padding: [0, 8, 0, 8],
                children: [
                  frame(`Sub Tab ${t.t} Row`, {
                    layout: 'horizontal', gap: 4, alignItems: 'center', height: 30,
                    children: [
                      text(`Sub Tab ${t.t} Label`, t.t, {
                        font: '$font-body', size: 11,
                        weight: t.active ? '700' : '500',
                        fill: t.active ? '$ink' : '$ink-faint',
                      }),
                      ...(t.dot ? [rect(`Sub Tab ${t.t} Dot`, {
                        width: 5, height: 5, fill: '$danger', cornerRadius: 999,
                      })] : []),
                    ],
                  }),
                  rect(`Sub Tab ${t.t} UL`, {
                    width: 'fill_container', height: 2,
                    fill: t.active ? '$accent' : 'transparent',
                  }),
                ],
              })),
            }),
            divider('Sub Tabs Divider'),
            // KPI strip: elapsed / events / retries / cost
            frame('Sub KPI Strip', {
              width: 'fill_container', height: 60,
              layout: 'horizontal', gap: 0, alignItems: 'stretch',
              stroke: '$line', strokeWidth: 1,
              children: [
                ['Elapsed', '34s',    '$ink'],
                ['Events',  '5',      '$agent'],
                ['Retries', '0',      '$ink'],
                ['Cost',    '2.4k tk', '$ink'],
              ].map(([label, value, valueFill], i, arr) => frame(`Sub KPI ${label}`, {
                width: 'fill_container', height: 'fill_container',
                fill: '$surface',
                stroke: i < arr.length - 1 ? '$line' : undefined,
                strokeWidth: i < arr.length - 1 ? 1 : undefined,
                layout: 'vertical', gap: 4, padding: [10, 14, 10, 14],
                children: [
                  text(`Sub KPI ${label} Label`, label.toUpperCase(), {
                    font: '$font-data', size: 9, weight: '700', tracking: 0.1, fill: '$ink-faint',
                  }),
                  text(`Sub KPI ${label} Value`, value, {
                    font: '$font-display', size: 18, weight: '700', fill: valueFill,
                  }),
                ],
              })),
            }),
            // Timeline events with connecting line
            frame('Sub Timeline', {
              width: 'fill_container', layout: 'vertical', gap: 0,
              padding: [12, 16, 12, 16],
              children: (() => {
                const events = [
                  { at: '10:02:03', kind: 'cap.assign',   detail: 'assign_capability(agent-uai) · 已授予 chat.read', tone: 'success' },
                  { at: '10:02:11', kind: 'cap.execute',  detail: 'run_workflow(diff-analysis) · 拉取 dipole/mr/487', tone: 'success' },
                  { at: '10:02:22', kind: 'artifact.new', detail: 'diff-summary.md · 8.4 KB · 已挂到当前任务', tone: 'success', badge: 'ARTIFACT' },
                  { at: '10:02:34', kind: 'cap.reply',    detail: 'post_message(#conv-alice) · 等待模型返回', tone: 'agent' },
                  { at: '10:02:41', kind: 'wait.input',   detail: '需要 owner 确认扫描范围（3 选项）', tone: 'warning', badge: 'INPUT' },
                ]
                return events.map((e, i) => timelineEvent(`Sub TL ${i}`, {
                  ...e, isLast: i === events.length - 1, width: 360,
                }))
              })(),
            }),
          ],
        }),
      ],
    }),
  ],
})

// -----------------------------------------------------------------------------
// Canvas layout — column A (mockups) at x=0, column B (patterns) at x=1600
// -----------------------------------------------------------------------------
const COL_A_X = 0
const COL_B_X = 1600
const ROW_GAP = 80

// -----------------------------------------------------------------------------
// Frame 1: App Chrome v2 (top bar only, no Chat body, no Drawer — reference)
// -----------------------------------------------------------------------------
const frame1 = frame('App Chrome v2', {
  x: COL_A_X, y: 0,
  width: 1440, height: 200, fill: '$canvas', cornerRadius: 0, clip: true,
  layout: 'vertical', gap: 0,
  children: [
    topBar({ activeTab: 'chat', pending: 3 }),
    frame('Chrome v2 Explainer Body', {
      width: 'fill_container', layout: 'vertical', gap: 8, padding: 24,
      children: [
        text('Chrome v2 Title', 'App Chrome v2', {
          font: '$font-display', size: 20, weight: '700', fill: '$ink',
        }),
        text('Chrome v2 Kv1', '顶级 tab: Chat · Directory · Settings（3 项，Agent 不在此列）', {
          font: '$font-body', size: 13, fill: '$ink-soft',
        }),
        text('Chrome v2 Kv2', '右侧 🤖 Cpu icon: 常驻，红色徽标 = pending input+approval 数', {
          font: '$font-body', size: 13, fill: '$ink-soft',
        }),
        text('Chrome v2 Kv3', '点击 🤖 → 右侧 Drawer 展开，默认 view=live', {
          font: '$font-body', size: 13, fill: '$ink-soft',
        }),
      ],
    }),
  ],
})

// -----------------------------------------------------------------------------
// Frame 2: Chat + Agent Drawer · Live view
// -----------------------------------------------------------------------------
const DRAWER_W = 460
const frame2 = frame('Chat + Agent Drawer · Live', {
  x: COL_A_X, y: 200 + ROW_GAP,
  width: 1440, height: 900, fill: '$canvas', cornerRadius: 0, clip: true,
  layout: 'vertical', gap: 0,
  children: [
    topBar({ activeTab: 'chat', pending: 3, agentActive: true }),
    frame('Chat Live Body', {
      width: 'fill_container', height: 'fill_container',
      layout: 'horizontal', gap: 0, alignItems: 'stretch',
      children: [
        conversationSidebar(),
        chatMainBody(1440 - 260 - DRAWER_W),
        // Agent Drawer
        frame('Agent Drawer Live', {
          width: DRAWER_W, height: 'fill_container', fill: '$surface',
          stroke: '$line', strokeWidth: 1, cornerRadius: 0,
          layout: 'vertical', gap: 0,
          children: [
            drawerHeader('Alice 上下文', 'live'),
            drawerLiveContent(),
          ],
        }),
      ],
    }),
    statusBar('SHADOW · DIRECT_TARGET · deepseek/v4-flash · 12 subs · 3 pending'),
  ],
})

// -----------------------------------------------------------------------------
// Frame 3: Chat + Agent Drawer · Tasks
// -----------------------------------------------------------------------------
const frame3 = frame('Chat + Agent Drawer · Tasks', {
  x: COL_A_X, y: 200 + ROW_GAP + 900 + ROW_GAP,
  width: 1440, height: 900, fill: '$canvas', cornerRadius: 0, clip: true,
  layout: 'vertical', gap: 0,
  children: [
    topBar({ activeTab: 'chat', pending: 3, agentActive: true }),
    frame('Chat Tasks Body', {
      width: 'fill_container', height: 'fill_container',
      layout: 'horizontal', gap: 0, alignItems: 'stretch',
      children: [
        conversationSidebar(),
        chatMainBody(1440 - 260 - 600),
        // Wider drawer for Tasks (600px) to fit sub-panel
        frame('Agent Drawer Tasks', {
          width: 600, height: 'fill_container', fill: '$surface',
          stroke: '$line', strokeWidth: 1, cornerRadius: 0,
          layout: 'vertical', gap: 0,
          children: [
            drawerHeader('任务运行 · 12 rows', 'tasks'),
            drawerTasksContent(),
          ],
        }),
      ],
    }),
    statusBar('SHADOW · DIRECT_TARGET · deepseek/v4-flash · 12 subs · 3 pending'),
  ],
})

// -----------------------------------------------------------------------------
// Frame 4: State Patterns v1 — the direct answer to "State Matrix 空洞占页"
// -----------------------------------------------------------------------------
// Renders 6 panels side-by-side: BEFORE (state-card) vs 4 replacement patterns
// (Loading / Empty / Failed / Stale) + Success toast + ConfirmDialog.

const PANEL_W = 440
const PANEL_H = 320
const PANEL_GAP = 24

const statePanel = (name, title, subtitle, body, opts = {}) => frame(`Panel ${name}`, {
  width: PANEL_W, height: PANEL_H, fill: '$surface',
  stroke: opts.tone ? `$${opts.tone}` : '$line', strokeWidth: opts.tone ? 2 : 1, cornerRadius: 0,
  layout: 'vertical', gap: 0,
  children: [
    frame(`Panel ${name} Head`, {
      width: 'fill_container', height: 44,
      layout: 'vertical', gap: 2, padding: [8, 12, 8, 12],
      children: [
        text(`Panel ${name} Title`, title, {
          font: '$font-display', size: 13, weight: '700',
          fill: opts.tone ? `$${opts.tone}` : '$ink',
        }),
        text(`Panel ${name} Subtitle`, subtitle, {
          font: '$font-data', size: 10, fill: '$ink-faint',
        }),
      ],
    }),
    divider(`Panel ${name} Divider`),
    body,
  ],
})

// Panel A: BEFORE — the .state-card independent page (marked with red border to signal deprecation)
const panelBefore = statePanel('Before', 'BEFORE · State Matrix 独占页', '.state-card { margin:72px auto } — 8 处复制，占屏无信息',
  frame('Panel Before Body', {
    width: 'fill_container', height: 'fill_container',
    layout: 'vertical', gap: 0, alignItems: 'center', justifyContent: 'center',
    children: [
      frame('Before Card', {
        width: 300, fill: '$danger-soft',
        stroke: '$danger', strokeWidth: 1, cornerRadius: 0,
        layout: 'vertical', gap: 8, padding: 20, alignItems: 'center',
        children: [
          text('Before State Code', 'UNAVAILABLE', {
            font: '$font-data', size: 10, weight: '700', tracking: 0.1, fill: '$danger',
          }),
          text('Before State Title', '订阅控制面暂时不可用', {
            font: '$font-display', size: 18, weight: '700', fill: '$ink',
          }),
          text('Before State Body', '已清空缓存列表并关闭撤销动作', {
            font: '$font-body', size: 12, fill: '$ink-soft',
          }),
          text('Before Action', '重新确认 →', {
            font: '$font-body', size: 12, weight: '700', fill: '$danger',
          }),
        ],
      }),
    ],
  }),
  { tone: 'danger' }
)

// Panel B: LOADING — skeleton row + toolbar spinner
const panelLoading = statePanel('Loading', 'AFTER · Loading', 'Toolbar inline spinner + Skeleton rows；骨架永远先渲染',
  frame('Panel Loading Body', {
    width: 'fill_container', height: 'fill_container',
    layout: 'vertical', gap: 0,
    children: [
      // Sub-toolbar with spinner
      frame('Loading Sub Toolbar', {
        width: 'fill_container', height: 32, fill: '$surface',
        layout: 'horizontal', gap: 8, alignItems: 'center', padding: [0, 12, 0, 12],
        children: [
          text('Loading Sub Count', '订阅 · —', {
            font: '$font-body', size: 12, weight: '600', fill: '$ink-soft',
          }),
          icon('Loading Spinner', 'loader', 12, '$accent'),
          text('Loading Sub Msg', '正在读取…', {
            font: '$font-data', size: 10, fill: '$ink-faint',
          }),
          frame('Loading Sub Spacer', { width: 'fill_container' }),
          button('Loading Create', '+ 创建', { tone: 'ghost', height: 24 }),
        ],
      }),
      divider('Loading Sub Div'),
      tableHeader([[100, 'STATUS'], [180, 'AGENT'], [80, 'VERSION']]),
      skeletonRow('Loading Skel 1', [100, 180, 80]),
      skeletonRow('Loading Skel 2', [100, 180, 80]),
      skeletonRow('Loading Skel 3', [100, 180, 80]),
      skeletonRow('Loading Skel 4', [100, 180, 80]),
    ],
  })
)

// Panel C: EMPTY — empty row + primary CTA
const panelEmpty = statePanel('Empty', 'AFTER · Empty', '表格内 empty row + 内嵌 CTA；骨架仍在',
  frame('Panel Empty Body', {
    width: 'fill_container', height: 'fill_container',
    layout: 'vertical', gap: 0,
    children: [
      frame('Empty Sub Toolbar', {
        width: 'fill_container', height: 32, fill: '$surface',
        layout: 'horizontal', gap: 8, alignItems: 'center', padding: [0, 12, 0, 12],
        children: [
          text('Empty Sub Count', '订阅 · 0', {
            font: '$font-body', size: 12, weight: '600', fill: '$ink',
          }),
          frame('Empty Sub Spacer', { width: 'fill_container' }),
          button('Empty Create', '+ 创建订阅', { tone: 'primary', height: 24, iconLeft: 'plus' }),
        ],
      }),
      divider('Empty Sub Div'),
      tableHeader([[100, 'STATUS'], [180, 'AGENT'], [80, 'VERSION']]),
      frame('Empty Row', {
        width: 'fill_container', height: 120, fill: '$surface',
        layout: 'vertical', gap: 6, alignItems: 'center', justifyContent: 'center',
        children: [
          icon('Empty Icon', 'inbox', 20, '$ink-faint'),
          text('Empty Row Label', '还没有事件订阅', {
            font: '$font-body', size: 13, weight: '600', fill: '$ink-soft',
          }),
          text('Empty Row Hint', '创建订阅前需要 active Definition version', {
            font: '$font-body', size: 11, fill: '$ink-faint',
          }),
          button('Empty Inline CTA', '+ 创建第一个订阅', { tone: 'primary', height: 28, iconLeft: 'plus' }),
        ],
      }),
    ],
  })
)

// Panel D: FAILED — inline banner + retry + stale data still visible
const panelFailed = statePanel('Failed', 'AFTER · Failed', 'Banner tone=danger + Retry；表格保留上次数据（灰化）',
  frame('Panel Failed Body', {
    width: 'fill_container', height: 'fill_container',
    layout: 'vertical', gap: 0,
    children: [
      banner('Failed', 'danger', '订阅列表读取失败 · 授权或网络异常', 'Retry'),
      tableHeader([[100, 'STATUS'], [180, 'AGENT'], [80, 'VERSION']]),
      // Stale rows with reduced opacity
      ...[
        { st: 'ACTIVE',  ag: 'Project Guardian', ver: 'v3', tone: 'agent' },
        { st: 'ACTIVE',  ag: 'agent-uai',        ver: 'v3', tone: 'agent' },
        { st: 'REVOKED', ag: 'sec-scan',         ver: 'v1', tone: 'danger'},
      ].map(r => frame(`Failed Row ${r.ag}`, {
        width: 'fill_container', height: 32, fill: '$surface',
        stroke: '$line', strokeWidth: 1, opacity: 0.5,
        layout: 'horizontal', gap: 0, alignItems: 'stretch',
        children: [
          cell(`Failed ${r.ag} Status`, 100, pill(r.st, r.tone === 'agent' ? 'agent' : 'danger')),
          cell(`Failed ${r.ag} Agent`, 180,
            text(`Failed ${r.ag} Name`, r.ag, { font: '$font-body', size: 12, fill: '$ink' })),
          cell(`Failed ${r.ag} Ver`, 80,
            text(`Failed ${r.ag} V`, r.ver, { font: '$font-data', size: 11, fill: '$ink-faint' })),
        ],
      })),
      frame('Failed Watermark', {
        width: 'fill_container', height: 24,
        layout: 'horizontal', gap: 6, alignItems: 'center', justifyContent: 'flex-end',
        padding: [0, 12, 0, 12],
        children: [
          icon('Failed WM Icon', 'clock', 10, '$ink-faint'),
          text('Failed WM Text', 'stale · updated 34s ago', {
            font: '$font-data', size: 9, fill: '$ink-faint',
          }),
        ],
      }),
    ],
  })
)

// Panel E: STALE — warning banner + refresh; data still fresh but backend conflict
const panelStale = statePanel('Stale', 'AFTER · Stale (definition drift)', 'Banner tone=warning + Refresh；不清空数据',
  frame('Panel Stale Body', {
    width: 'fill_container', height: 'fill_container',
    layout: 'vertical', gap: 0,
    children: [
      banner('Stale', 'warning', 'Definition v3 已失效，撤销请求未被接受', '重新读取'),
      tableHeader([[100, 'STATUS'], [180, 'AGENT'], [80, 'VERSION']]),
      ...[
        { st: 'ACTIVE',  ag: 'Project Guardian', ver: 'v3', tone: 'agent' },
        { st: 'ACTIVE',  ag: 'agent-uai',        ver: 'v3', tone: 'agent' },
      ].map(r => frame(`Stale Row ${r.ag}`, {
        width: 'fill_container', height: 32, fill: '$surface',
        stroke: '$line', strokeWidth: 1,
        layout: 'horizontal', gap: 0, alignItems: 'stretch',
        children: [
          cell(`Stale ${r.ag} Status`, 100, pill(r.st, 'agent')),
          cell(`Stale ${r.ag} Agent`, 180,
            text(`Stale ${r.ag} Name`, r.ag, { font: '$font-body', size: 12, fill: '$ink' })),
          cell(`Stale ${r.ag} Ver`, 80,
            text(`Stale ${r.ag} V`, r.ver, { font: '$font-data', size: 11, fill: '$ink' })),
        ],
      })),
    ],
  })
)

// Panel F: SUCCESS FLASH — toast bottom-right
const panelSuccess = statePanel('Success', 'AFTER · Success flash', 'Toast 右下角 · 3s 自动消失',
  frame('Panel Success Body', {
    width: 'fill_container', height: 'fill_container', fill: '$canvas',
    layout: 'vertical', gap: 0, justifyContent: 'flex-end', alignItems: 'flex-end',
    padding: 16,
    children: [
      frame('Success Toast', {
        width: 280, fill: '$success-soft',
        stroke: '$success', strokeWidth: 1, cornerRadius: 0,
        layout: 'horizontal', gap: 10, padding: 12, alignItems: 'center',
        children: [
          icon('Success Toast Icon', 'check-circle', 16, '$success'),
          frame('Success Toast Body', {
            width: 'fill_container', layout: 'vertical', gap: 2,
            children: [
              text('Success Toast Title', '订阅已创建', {
                font: '$font-body', size: 12, weight: '700', fill: '$success',
              }),
              text('Success Toast Body Text', 'Project Guardian → #project-alpha', {
                font: '$font-data', size: 10, fill: '$ink-soft',
              }),
            ],
          }),
          icon('Success Toast Close', 'x', 12, '$success', { opacity: 0.5 }),
        ],
      }),
    ],
  })
)

const frame4 = frame('State Patterns v1', {
  x: COL_B_X, y: 0,
  width: PANEL_W * 3 + PANEL_GAP * 4, height: PANEL_H * 2 + 44 + PANEL_GAP * 3,
  fill: '$canvas', cornerRadius: 0, clip: true,
  layout: 'vertical', gap: PANEL_GAP,
  padding: PANEL_GAP,
  children: [
    frame('State Patterns Title', {
      width: 'fill_container', layout: 'vertical', gap: 4,
      children: [
        text('State Patterns Title Text', 'State Patterns v1', {
          font: '$font-display', size: 20, weight: '700', fill: '$ink',
        }),
        text('State Patterns Subtitle', 'BEFORE 一格是当前 .state-card 独占页；后面 5 格是新规则（骨架永远先渲染，状态永远不占 hero）', {
          font: '$font-body', size: 12, fill: '$ink-soft',
        }),
      ],
    }),
    frame('State Patterns Row 1', {
      layout: 'horizontal', gap: PANEL_GAP,
      children: [panelBefore, panelLoading, panelEmpty],
    }),
    frame('State Patterns Row 2', {
      layout: 'horizontal', gap: PANEL_GAP,
      children: [panelFailed, panelStale, panelSuccess],
    }),
  ],
})

// -----------------------------------------------------------------------------
// Frame 5: Chat + Agent Drawer · Subscriptions
// —— 直接把你点名的场景（Agent Subscription/State Matrix + Create State Matrix）
//    嵌回 Chat 右侧 Drawer，且用行内 Banner + Skeleton + Empty Row 取代原独占页
// -----------------------------------------------------------------------------

const drawerSubscriptionsContent = () => frame('Drawer Subs Content', {
  width: 'fill_container', height: 'fill_container',
  layout: 'vertical', gap: 0,
  children: [
    // Sub toolbar
    frame('Subs Sub Toolbar', {
      width: 'fill_container', height: 40, fill: '$surface',
      layout: 'horizontal', gap: 12, alignItems: 'center',
      padding: [0, 16, 0, 16],
      children: [
        text('Subs Count', '订阅 · 4', {
          font: '$font-body', size: 12, weight: '600', fill: '$ink',
        }),
        text('Subs Filter', '状态: 全部 ▾', {
          font: '$font-body', size: 12, fill: '$ink-faint',
        }),
        frame('Subs Sub Toolbar Spacer', { width: 'fill_container' }),
        button('Subs Create', '+ 创建订阅', { tone: 'primary', iconLeft: 'plus' }),
      ],
    }),
    divider('Subs Sub Toolbar Divider'),
    // Inline warning banner —— 直接 replaces the "definition_stale" state-card 独占页
    banner('Subs Stale', 'warning',
      'Definition v3 已失效，撤销请求未被接受', '重新读取'),
    // Table header
    frame('Subs Table Header', {
      width: 'fill_container', height: 32, fill: '$surface-muted',
      layout: 'horizontal', gap: 0, alignItems: 'stretch',
      children: [
        cell('Subs H Status', 110, text('Subs H Status L', 'STATUS', {
          font: '$font-data', size: 9, weight: '800', tracking: 0.1, fill: '$ink-soft',
        })),
        cell('Subs H Agent', 160, text('Subs H Agent L', 'AGENT', {
          font: '$font-data', size: 9, weight: '800', tracking: 0.1, fill: '$ink-soft',
        })),
        cell('Subs H Scope', 200, text('Subs H Scope L', 'SCOPE', {
          font: '$font-data', size: 9, weight: '800', tracking: 0.1, fill: '$ink-soft',
        })),
        cell('Subs H Filter', 130, text('Subs H Filter L', 'FILTER', {
          font: '$font-data', size: 9, weight: '800', tracking: 0.1, fill: '$ink-soft',
        })),
      ],
    }),
    // Rows
    ...[
      { st: 'ACTIVE',  ag: 'Project Guardian', scope: '#conv-alice',      filter: 'contains any (3)', tone: 'agent'   },
      { st: 'ACTIVE',  ag: 'agent-uai',        scope: '#project-alpha',    filter: 'all messages',    tone: 'agent'   },
      { st: 'ACTIVE',  ag: 'sec-scan',         scope: '#devops-review',   filter: 'contains any (2)', tone: 'agent'   },
      { st: 'REVOKED', ag: 'legacy-crawler',   scope: '#conv-bob',        filter: 'all messages',    tone: 'danger'  },
    ].map(r => frame(`Subs Row ${r.ag}`, {
      width: 'fill_container', height: 40, fill: '$surface',
      stroke: '$line', strokeWidth: 1,
      layout: 'horizontal', gap: 0, alignItems: 'stretch',
      opacity: r.st === 'REVOKED' ? 0.55 : 1,
      children: [
        cell(`Subs ${r.ag} Status`, 110, pill(r.st, r.tone === 'agent' ? 'agent' : 'danger')),
        cell(`Subs ${r.ag} Agent`, 160, text(`Subs ${r.ag} N`, r.ag, {
          font: '$font-body', size: 12, weight: '600', fill: '$ink',
        })),
        cell(`Subs ${r.ag} Scope`, 200, text(`Subs ${r.ag} S`, r.scope, {
          font: '$font-data', size: 11, fill: '$ink-soft',
        })),
        cell(`Subs ${r.ag} Filter`, 130, text(`Subs ${r.ag} F`, r.filter, {
          font: '$font-data', size: 11, fill: '$ink-soft',
        })),
      ],
    })),
    // Bottom explainer band
    frame('Subs Bottom Note', {
      width: 'fill_container', fill: '$surface-muted',
      layout: 'horizontal', gap: 8, alignItems: 'center',
      padding: [10, 16, 10, 16],
      children: [
        icon('Subs Bottom Note Icon', 'info', 12, '$ink-faint'),
        text('Subs Bottom Note Text',
          '过去 4 种状态独占整页；现在 stale 走上方 Banner，revoked 走灰化行内，empty/失败走 Skeleton + Retry。',
          { font: '$font-body', size: 11, fill: '$ink-soft', wrap: true, width: 500 }),
      ],
    }),
  ],
})

const frame5 = frame('Chat + Agent Drawer · Subscriptions', {
  x: COL_A_X, y: 200 + ROW_GAP + (900 + ROW_GAP) * 2,
  width: 1440, height: 900, fill: '$canvas', cornerRadius: 0, clip: true,
  layout: 'vertical', gap: 0,
  children: [
    topBar({ activeTab: 'chat', pending: 3, agentActive: true }),
    frame('Chat Subs Body', {
      width: 'fill_container', height: 'fill_container',
      layout: 'horizontal', gap: 0, alignItems: 'stretch',
      children: [
        conversationSidebar(),
        chatMainBody(1440 - 260 - 620),
        frame('Agent Drawer Subs', {
          width: 620, height: 'fill_container', fill: '$surface',
          stroke: '$line', strokeWidth: 1, cornerRadius: 0,
          layout: 'vertical', gap: 0,
          children: [
            drawerHeader('事件订阅 · 4 rows', 'subscriptions'),
            drawerSubscriptionsContent(),
          ],
        }),
      ],
    }),
    statusBar('SHADOW · DIRECT_TARGET · deepseek/v4-flash · 4 subs · 3 pending'),
  ],
})

// -----------------------------------------------------------------------------
// Splice: replace-by-name, atomic write
// -----------------------------------------------------------------------------

const frames = [
  ['App Chrome v2', frame1],
  ['Chat + Agent Drawer · Live', frame2],
  ['Chat + Agent Drawer · Tasks', frame3],
  ['Chat + Agent Drawer · Subscriptions', frame5],
  ['State Patterns v1', frame4],
]

const report = []
for (const [name, newFrame] of frames) {
  const existing = doc.children.findIndex(c => c && c.name === name)
  if (existing >= 0) {
    doc.children.splice(existing, 1, newFrame)
    report.push(`  replaced   ${name}  (index ${existing})`)
  } else {
    doc.children.push(newFrame)
    report.push(`  appended   ${name}  (index ${doc.children.length - 1})`)
  }
}

const tmp = `${target}.tmp-${process.pid}`
writeFileSync(tmp, JSON.stringify(doc, null, 2), 'utf8')
renameSync(tmp, target)

const sizeKb = (readFileSync(target, 'utf8').length / 1024).toFixed(1)
console.log(`Wrote ${target} (${sizeKb} KB)`)
console.log(report.join('\n'))
console.log(`Node count generated by this script: ${idCounter}`)
