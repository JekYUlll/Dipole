#!/usr/bin/env node

// Appends a single top-level frame `Agent Workspace v1` to design/dipole-ui.pen.
// Idempotent: if the frame is already present (by name), replaces it in place.
// Writes atomically via a temp file.

import { readFileSync, writeFileSync, renameSync } from 'node:fs'
import { resolve, dirname } from 'node:path'

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
const id = () => `awv1_${String(++idCounter).padStart(4, '0')}`

const text = (name, content, opts = {}) => ({
  type: 'text',
  id: id(),
  name,
  content,
  fontFamily: opts.font ?? '$font-body',
  fontSize: opts.size ?? 13,
  fontWeight: opts.weight ?? 'normal',
  letterSpacing: opts.tracking ?? 0,
  fill: opts.fill ?? '$ink',
  ...(opts.width !== undefined ? { width: opts.width } : {}),
  ...(opts.wrap ? { textWrap: true } : {}),
})

const icon = (name, feather, size, fill) => ({
  type: 'icon',
  id: id(),
  name,
  library: 'feather',
  icon: feather,
  width: size,
  height: size,
  fill,
})

const frame = (name, opts = {}) => ({
  type: 'frame',
  id: id(),
  name,
  ...(opts.width !== undefined ? { width: opts.width } : {}),
  ...(opts.height !== undefined ? { height: opts.height } : {}),
  ...(opts.fill !== undefined ? { fill: opts.fill } : {}),
  ...(opts.cornerRadius !== undefined ? { cornerRadius: opts.cornerRadius } : {}),
  ...(opts.stroke !== undefined ? { stroke: opts.stroke } : {}),
  ...(opts.strokeWidth !== undefined ? { strokeWidth: opts.strokeWidth } : {}),
  ...(opts.layout !== undefined ? { layout: opts.layout } : {}),
  ...(opts.gap !== undefined ? { gap: opts.gap } : {}),
  ...(opts.padding !== undefined ? { padding: opts.padding } : {}),
  ...(opts.justifyContent !== undefined ? { justifyContent: opts.justifyContent } : {}),
  ...(opts.alignItems !== undefined ? { alignItems: opts.alignItems } : {}),
  ...(opts.clip !== undefined ? { clip: opts.clip } : {}),
  children: opts.children ?? [],
})

const rect = (name, opts = {}) => ({
  type: 'rectangle',
  id: id(),
  name,
  ...(opts.width !== undefined ? { width: opts.width } : {}),
  ...(opts.height !== undefined ? { height: opts.height } : {}),
  ...(opts.fill !== undefined ? { fill: opts.fill } : {}),
  ...(opts.cornerRadius !== undefined ? { cornerRadius: opts.cornerRadius } : {}),
})

const pill = (label, tone) => {
  const bg = `$${tone}-soft`
  const fg = tone === 'agent' ? '$agent' : `$${tone}`
  return frame(`${label} Pill`, {
    height: 20,
    fill: bg,
    cornerRadius: 999,
    layout: 'horizontal',
    gap: 6,
    padding: [0, 8, 0, 8],
    alignItems: 'center',
    children: [
      rect(`${label} Dot`, { width: 6, height: 6, fill: fg, cornerRadius: 999 }),
      text(`${label} Label`, label, { font: '$font-data', size: 10, weight: '600', tracking: 0.06, fill: fg }),
    ],
  })
}

const kpiCard = (label, value, delta, tone = 'neutral') => frame(`KPI ${label}`, {
  width: 'fill_container',
  height: 76,
  fill: '$surface',
  cornerRadius: 0,
  layout: 'vertical',
  padding: 12,
  gap: 6,
  children: [
    text(`KPI ${label} Eyebrow`, label.toUpperCase(), {
      font: '$font-data', size: 10, weight: '600', tracking: 0.06, fill: '$ink-soft',
    }),
    frame(`KPI ${label} Value Row`, {
      layout: 'horizontal', gap: 8, alignItems: 'baseline',
      children: [
        text(`KPI ${label} Value`, value, { font: '$font-display', size: 22, weight: '700', fill: '$ink' }),
        text(`KPI ${label} Delta`, delta, { font: '$font-data', size: 10, fill: tone === 'danger' ? '$danger' : (tone === 'success' ? '$success' : '$ink-faint') }),
      ],
    }),
  ],
})

const tableCell = (name, width, node) => frame(`${name} Cell`, {
  width, height: 'fill_container', layout: 'horizontal', alignItems: 'center', padding: [0, 12, 0, 12],
  children: [node],
})

const tableRow = (rowName, cols, opts = {}) => frame(rowName, {
  width: 'fill_container', height: 32,
  fill: opts.fill ?? '$surface',
  layout: 'horizontal', gap: 0, alignItems: 'stretch',
  children: [
    rect(`${rowName} Stripe`, { width: 3, height: 'fill_container', fill: opts.stripe ?? (opts.fill ?? '$surface') }),
    ...cols,
  ],
})

// -----------------------------------------------------------------------------
// Frame contents
// -----------------------------------------------------------------------------
const FRAME_W = 1440
const FRAME_H = 1024

// --- App Top Bar (48) ---
const topBar = frame('App Top Bar', {
  width: 'fill_container', height: 48, fill: '$rail',
  layout: 'horizontal', padding: [0, 20, 0, 20], gap: 24, alignItems: 'center',
  children: [
    frame('Brand Group', {
      layout: 'horizontal', gap: 32, alignItems: 'center',
      children: [
        frame('Brand', {
          layout: 'horizontal', gap: 8, alignItems: 'center',
          children: [
            rect('Brand Dot', { width: 10, height: 10, fill: '$accent', cornerRadius: 999 }),
            text('Brand Wordmark', 'DIPOLE', {
              font: '$font-display', size: 15, weight: '700', tracking: 0.12, fill: '$text-inverse',
            }),
          ],
        }),
        frame('Workspace Switcher', {
          layout: 'horizontal', gap: 20, alignItems: 'center',
          children: [
            text('Chat Workspace Tab', 'Chat', { font: '$font-body', size: 13, fill: '$ink-faint' }),
            text('Agent Workspace Tab', 'Agent', { font: '$font-body', size: 13, weight: '700', fill: '$text-inverse' }),
            text('Directory Workspace Tab', 'Directory', { font: '$font-body', size: 13, fill: '$ink-faint' }),
            text('Settings Workspace Tab', 'Settings', { font: '$font-body', size: 13, fill: '$ink-faint' }),
          ],
        }),
      ],
    }),
    frame('Top Bar Spacer', { width: 'fill_container' }),
    frame('Top Bar Right', {
      layout: 'horizontal', gap: 16, alignItems: 'center',
      children: [
        icon('Top Bar Search Icon', 'search', 16, '$ink-faint'),
        icon('Top Bar Bell Icon', 'bell', 16, '$ink-faint'),
        frame('Top Bar Avatar', {
          width: 28, height: 28, fill: '$accent', cornerRadius: 999,
          layout: 'vertical', justifyContent: 'center', alignItems: 'center',
          children: [text('Avatar Initials', 'EJ', { font: '$font-data', size: 10, weight: '700', fill: '$text-inverse' })],
        }),
      ],
    }),
  ],
})

// --- Icon Rail (48 wide) ---
const iconRail = frame('Icon Rail', {
  width: 48, height: 'fill_container', fill: '$rail',
  layout: 'vertical', padding: [16, 0, 16, 0], gap: 8, alignItems: 'center',
  children: [
    frame('Rail Tasks Item', {
      width: 40, height: 40, fill: '$accent', cornerRadius: 0,
      layout: 'vertical', justifyContent: 'center', alignItems: 'center',
      children: [icon('Rail Tasks Icon', 'inbox', 18, '$text-inverse')],
    }),
    frame('Rail Artifacts Item', {
      width: 40, height: 40, cornerRadius: 0,
      layout: 'vertical', justifyContent: 'center', alignItems: 'center',
      children: [icon('Rail Artifacts Icon', 'package', 18, '$ink-faint')],
    }),
    frame('Rail Definitions Item', {
      width: 40, height: 40, cornerRadius: 0,
      layout: 'vertical', justifyContent: 'center', alignItems: 'center',
      children: [icon('Rail Definitions Icon', 'grid', 18, '$ink-faint')],
    }),
    frame('Rail Subscriptions Item', {
      width: 40, height: 40, cornerRadius: 0,
      layout: 'vertical', justifyContent: 'center', alignItems: 'center',
      children: [icon('Rail Subscriptions Icon', 'radio', 18, '$ink-faint')],
    }),
    frame('Rail Memories Item', {
      width: 40, height: 40, cornerRadius: 0,
      layout: 'vertical', justifyContent: 'center', alignItems: 'center',
      children: [icon('Rail Memories Icon', 'cpu', 18, '$ink-faint')],
    }),
  ],
})

// --- Workspace Toolbar (48) ---
const workspaceToolbar = frame('Workspace Toolbar', {
  width: 'fill_container', height: 48, fill: '$surface',
  stroke: '$line', strokeWidth: 1,
  layout: 'horizontal', padding: [0, 16, 0, 16], gap: 24, alignItems: 'center',
  children: [
    frame('Toolbar Title Group', {
      layout: 'horizontal', gap: 8, alignItems: 'baseline',
      children: [
        text('Toolbar Workspace Title', 'Agent', { font: '$font-display', size: 15, weight: '700', fill: '$ink' }),
        text('Toolbar Workspace Separator', '/', { font: '$font-body', size: 13, fill: '$ink-faint' }),
        text('Toolbar Tab Title', 'Tasks', { font: '$font-body', size: 13, weight: '600', fill: '$ink' }),
      ],
    }),
    frame('Toolbar Tab Rail', {
      layout: 'horizontal', gap: 20, alignItems: 'center', padding: [0, 12, 0, 12],
      children: ['Tasks', 'Artifacts', 'Definitions', 'Subscriptions', 'Memories'].map((t, i) => frame(`${t} Tab`, {
        layout: 'vertical', gap: 4, height: 48, justifyContent: 'center',
        children: [
          frame(`${t} Tab Row`, {
            layout: 'horizontal', gap: 6, alignItems: 'center',
            children: [
              icon(`${t} Tab Icon`, ['inbox', 'package', 'grid', 'radio', 'cpu'][i], 14, i === 0 ? '$ink' : '$ink-faint'),
              text(`${t} Tab Label`, t, { font: '$font-body', size: 12, weight: i === 0 ? '700' : 'normal', fill: i === 0 ? '$ink' : '$ink-soft' }),
            ],
          }),
          rect(`${t} Tab Underline`, { width: 'fill_container', height: 2, fill: i === 0 ? '$accent' : '$surface' }),
        ],
      })),
    }),
    frame('Toolbar Spacer', { width: 'fill_container' }),
    frame('Toolbar Actions', {
      layout: 'horizontal', gap: 8, alignItems: 'center',
      children: [
        // Search
        frame('Toolbar Search', {
          width: 240, height: 32, fill: '$surface',
          stroke: '$line', strokeWidth: 1, cornerRadius: 0,
          layout: 'horizontal', padding: [0, 10, 0, 10], gap: 8, alignItems: 'center',
          children: [
            icon('Toolbar Search Icon', 'search', 14, '$ink-faint'),
            text('Toolbar Search Placeholder', '按 task / owner / state 检索', { font: '$font-body', size: 12, fill: '$ink-faint' }),
          ],
        }),
        // Filter icon-only button
        frame('Filter Button', {
          width: 32, height: 32, fill: '$surface',
          stroke: '$line', strokeWidth: 1, cornerRadius: 0,
          layout: 'vertical', justifyContent: 'center', alignItems: 'center',
          children: [icon('Filter Button Icon', 'filter', 14, '$ink')],
        }),
        // Refresh icon-only button
        frame('Refresh Button', {
          width: 32, height: 32, fill: '$surface',
          stroke: '$line', strokeWidth: 1, cornerRadius: 0,
          layout: 'vertical', justifyContent: 'center', alignItems: 'center',
          children: [icon('Refresh Button Icon', 'refresh-cw', 14, '$ink')],
        }),
        // Primary create button
        frame('New Task Button', {
          height: 32, fill: '$accent', cornerRadius: 0,
          layout: 'horizontal', padding: [0, 14, 0, 12], gap: 6, alignItems: 'center',
          children: [
            icon('New Task Plus Icon', 'plus', 14, '$text-inverse'),
            text('New Task Label', '新建任务', { font: '$font-body', size: 13, weight: '600', fill: '$text-inverse' }),
          ],
        }),
      ],
    }),
  ],
})

// --- Metric Strip (76h + 12px padding = 100) ---
const metricStrip = frame('Metric Strip', {
  width: 'fill_container', fill: '$canvas',
  layout: 'horizontal', gap: 12, padding: 12,
  children: [
    kpiCard('Total tasks 24h', '128', '+12', 'success'),
    kpiCard('Pending approval', '3', 'oldest 12m', 'neutral'),
    kpiCard('Waiting input', '5', 'SLA 30m', 'neutral'),
    kpiCard('Failed today', '2', '↓1 vs 昨日', 'danger'),
  ],
})

// --- Data Table Panel ---
const tableHeader = frame('DataTable Header Row', {
  width: 'fill_container', height: 36, fill: '$surface-muted',
  layout: 'horizontal', alignItems: 'stretch', gap: 0,
  children: [
    rect('Header Stripe', { width: 3, height: 'fill_container', fill: '$surface-muted' }),
    tableCell('Header Task', 320, text('Header Task Text', 'TASK', { font: '$font-data', size: 10, weight: '600', tracking: 0.06, fill: '$ink-soft' })),
    tableCell('Header Kind', 200, text('Header Kind Text', 'KIND', { font: '$font-data', size: 10, weight: '600', tracking: 0.06, fill: '$ink-soft' })),
    tableCell('Header State', 180, text('Header State Text', 'STATE', { font: '$font-data', size: 10, weight: '600', tracking: 0.06, fill: '$ink-soft' })),
    tableCell('Header Rev', 90, text('Header Rev Text', 'REV', { font: '$font-data', size: 10, weight: '600', tracking: 0.06, fill: '$ink-soft' })),
    tableCell('Header Updated', 170, text('Header Updated Text', 'UPDATED', { font: '$font-data', size: 10, weight: '600', tracking: 0.06, fill: '$ink-soft' })),
    tableCell('Header Owner', 180, text('Header Owner Text', 'OWNER', { font: '$font-data', size: 10, weight: '600', tracking: 0.06, fill: '$ink-soft' })),
  ],
})

const mockRows = [
  { task: 'task:c31da…e020', kind: 'subscription_autoreply', state: ['WAITING_APPROVAL', 'warning'], rev: '1024', updated: '2026-09-03 20:41', owner: 'approval.bot', selected: true },
  { task: 'task:e72bf…a113', kind: 'oauth_bootstrap',        state: ['FAILED',           'danger' ], rev: '1023', updated: '2026-09-03 20:38', owner: 'gateway',      selected: false },
  { task: 'task:d9981…f887', kind: 'artifact_digest',        state: ['SUCCEEDED',        'success'], rev: '1022', updated: '2026-09-03 20:35', owner: 'ops.runner',   selected: false },
  { task: 'task:bc213…d9aa', kind: 'subscription_autoreply', state: ['WAITING_INPUT',    'agent'  ], rev: '1021', updated: '2026-09-03 20:29', owner: 'lina.chen',    selected: false },
  { task: 'task:abc12…c3f4', kind: 'oauth_verify',           state: ['RUNNING',          'agent'  ], rev: '1020', updated: '2026-09-03 20:15', owner: 'system.agent', selected: false },
  { task: 'task:98af1…82a4', kind: 'artifact_digest',        state: ['SUCCEEDED',        'success'], rev: '1019', updated: '2026-09-03 19:58', owner: 'ops.runner',   selected: false },
  { task: 'task:7bb02…5c19', kind: 'subscription_autoreply', state: ['SUCCEEDED',        'success'], rev: '1018', updated: '2026-09-03 19:41', owner: 'lina.chen',    selected: false },
  { task: 'task:641ce…88b0', kind: 'oauth_callback',         state: ['FAILED',           'danger' ], rev: '1017', updated: '2026-09-03 19:22', owner: 'gateway',      selected: false },
]

const bodyRows = mockRows.map((r, i) => tableRow(`Body Row ${i + 1}`, [
  tableCell(`Row ${i + 1} Task`, 320, text(`Row ${i + 1} Task Text`, r.task, { font: '$font-data', size: 11, fill: '$ink' })),
  tableCell(`Row ${i + 1} Kind`, 200, text(`Row ${i + 1} Kind Text`, r.kind, { font: '$font-body', size: 12, fill: '$ink-soft' })),
  tableCell(`Row ${i + 1} State`, 180, pill(r.state[0], r.state[1])),
  tableCell(`Row ${i + 1} Rev`, 90, text(`Row ${i + 1} Rev Text`, `rev.${r.rev}`, { font: '$font-data', size: 11, fill: '$ink-soft' })),
  tableCell(`Row ${i + 1} Updated`, 170, text(`Row ${i + 1} Updated Text`, r.updated, { font: '$font-data', size: 11, fill: '$ink-soft' })),
  tableCell(`Row ${i + 1} Owner`, 180, text(`Row ${i + 1} Owner Text`, r.owner, { font: '$font-body', size: 12, fill: '$ink-soft' })),
], {
  fill: r.selected ? '$accent-soft' : (i % 2 === 0 ? '$surface' : '$surface-muted'),
  stripe: r.selected ? '$accent' : (i % 2 === 0 ? '$surface' : '$surface-muted'),
}))

const tablePanel = frame('DataTable Panel', {
  width: 'fill_container', fill: '$surface',
  stroke: '$line', strokeWidth: 1,
  layout: 'vertical', gap: 0,
  children: [tableHeader, ...bodyRows],
})

// --- Detail Drawer (right column) ---
const drawerHeader = frame('Drawer Header', {
  width: 'fill_container', height: 60, fill: '$surface',
  stroke: '$line', strokeWidth: 1,
  layout: 'vertical', padding: [10, 16, 10, 16], gap: 4,
  children: [
    frame('Drawer Header Row', {
      layout: 'horizontal', gap: 8, alignItems: 'center',
      children: [
        icon('Drawer Back Icon', 'chevron-left', 14, '$ink-soft'),
        text('Drawer Task ID', 'task:c31da…e020', { font: '$font-data', size: 12, weight: '600', fill: '$ink' }),
        frame('Drawer Header Spacer', { width: 'fill_container' }),
        pill('WAITING_APPROVAL', 'warning'),
        icon('Drawer Close Icon', 'x', 14, '$ink-soft'),
      ],
    }),
    frame('Drawer Sub Row', {
      layout: 'horizontal', gap: 12, alignItems: 'center',
      children: [
        text('Drawer Sub Kind', 'subscription_autoreply', { font: '$font-body', size: 12, fill: '$ink-soft' }),
        text('Drawer Sub Owner', 'approval.bot', { font: '$font-body', size: 12, fill: '$ink-faint' }),
        text('Drawer Sub Rev', 'rev.1024', { font: '$font-data', size: 11, fill: '$ink-faint' }),
      ],
    }),
  ],
})

const drawerTabs = frame('Drawer Tabs', {
  width: 'fill_container', height: 36, fill: '$surface',
  stroke: '$line', strokeWidth: 1,
  layout: 'horizontal', padding: [0, 16, 0, 16], gap: 16, alignItems: 'stretch',
  children: ['Timeline', 'Approval', 'Input', 'Artifacts', 'Raw'].map((t, i) => frame(`${t} DrawerTab`, {
    layout: 'vertical', gap: 4, height: 36, justifyContent: 'center',
    children: [
      text(`${t} DrawerTab Label`, t, { font: '$font-body', size: 12, weight: i === 1 ? '700' : 'normal', fill: i === 1 ? '$ink' : '$ink-soft' }),
      rect(`${t} DrawerTab Underline`, { width: 'fill_container', height: 2, fill: i === 1 ? '$accent' : '$surface' }),
    ],
  })),
})

// Approval tab content — banner + evidence + actions
const approvalBanner = frame('Approval Banner', {
  width: 'fill_container', fill: '$warning-soft',
  stroke: '$warning', strokeWidth: 1,
  layout: 'horizontal', padding: 12, gap: 10, alignItems: 'flex-start',
  children: [
    icon('Approval Banner Icon', 'alert-circle', 16, '$warning'),
    frame('Approval Banner Text', {
      layout: 'vertical', gap: 4, width: 'fill_container',
      children: [
        text('Approval Banner Title', '等待你的审批', { font: '$font-body', size: 13, weight: '700', fill: '$warning' }),
        text('Approval Banner Body', 'Runtime 计划在会话 lina.chen ↔ pay-support 发送以下自动回复。审批仅授权本次执行，不改变 Definition。', {
          font: '$font-body', size: 12, fill: '$ink-soft', wrap: true, width: 'fill_container',
        }),
      ],
    }),
  ],
})

const approvalEvidence = frame('Approval Evidence', {
  width: 'fill_container', fill: '$surface-muted',
  layout: 'vertical', padding: 12, gap: 8,
  children: [
    text('Evidence Eyebrow', 'PROPOSED ACTION', { font: '$font-data', size: 10, weight: '600', tracking: 0.08, fill: '$ink-soft' }),
    text('Evidence Body', '你好，我们已经收到你的消息。付费问题请提供订单号 (18 位) 或商家 UID，我们的客服会在 30 分钟内回你。', {
      font: '$font-body', size: 13, fill: '$ink', wrap: true, width: 'fill_container',
    }),
    frame('Evidence Meta Row', {
      layout: 'horizontal', gap: 12,
      children: [
        text('Evidence Meta Cap', 'capability=conversation.send', { font: '$font-data', size: 10, fill: '$ink-faint' }),
        text('Evidence Meta Sha', 'sha256:9f4c…3821', { font: '$font-data', size: 10, fill: '$ink-faint' }),
      ],
    }),
  ],
})

const approvalReason = frame('Approval Reason', {
  width: 'fill_container', fill: '$surface',
  layout: 'vertical', padding: 12, gap: 6,
  children: [
    text('Reason Label', '批准理由（可选）', { font: '$font-body', size: 12, weight: '600', fill: '$ink' }),
    frame('Reason Textarea', {
      width: 'fill_container', height: 72, fill: '$surface',
      stroke: '$line', strokeWidth: 1, cornerRadius: 0,
      layout: 'vertical', padding: 10,
      children: [text('Reason Placeholder', '订单号已核验 · 属于常规客服流程', { font: '$font-body', size: 12, fill: '$ink-faint' })],
    }),
  ],
})

const approvalActions = frame('Approval Actions', {
  width: 'fill_container', fill: '$surface',
  stroke: '$line', strokeWidth: 1,
  layout: 'horizontal', padding: [12, 16, 12, 16], gap: 8, alignItems: 'center', justifyContent: 'flex-end',
  children: [
    frame('Reject Button', {
      height: 32, cornerRadius: 0,
      stroke: '$line', strokeWidth: 1, fill: '$surface',
      layout: 'horizontal', padding: [0, 14, 0, 14], alignItems: 'center', justifyContent: 'center',
      children: [text('Reject Button Label', '拒绝', { font: '$font-body', size: 13, weight: '600', fill: '$ink' })],
    }),
    frame('Cancel Button', {
      height: 32, cornerRadius: 0,
      layout: 'horizontal', padding: [0, 14, 0, 14], alignItems: 'center', justifyContent: 'center',
      children: [text('Cancel Button Label', '暂缓', { font: '$font-body', size: 13, weight: '600', fill: '$ink-soft' })],
    }),
    frame('Approve Button', {
      height: 32, fill: '$accent', cornerRadius: 0,
      layout: 'horizontal', padding: [0, 14, 0, 14], gap: 6, alignItems: 'center', justifyContent: 'center',
      children: [
        icon('Approve Check Icon', 'check', 14, '$text-inverse'),
        text('Approve Label', '批准并执行', { font: '$font-body', size: 13, weight: '700', fill: '$text-inverse' }),
      ],
    }),
  ],
})

// Timeline preview inside drawer (compact)
const timelinePreview = frame('Timeline Preview', {
  width: 'fill_container', fill: '$surface',
  layout: 'vertical', padding: 12, gap: 6,
  children: [
    text('Timeline Preview Eyebrow', 'TIMELINE  ·  5 events', { font: '$font-data', size: 10, weight: '600', tracking: 0.06, fill: '$ink-soft' }),
    ...[
      ['20:41', 'waiting_approval', 'agent', 'approval requested'],
      ['20:40', 'proposal.created', 'agent', 'capability=conversation.send'],
      ['20:39', 'context.compiled', 'neutral', 'compiler v2'],
      ['20:38', 'trigger.matched', 'neutral', 'subscription:sub-091'],
      ['20:38', 'run.started', 'success', 'rev.1024'],
    ].map(([t, kind, tone, meta], i) => frame(`Timeline Preview Row ${i + 1}`, {
      layout: 'horizontal', gap: 10, alignItems: 'center', width: 'fill_container',
      children: [
        text(`Timeline Row ${i + 1} Time`, t, { font: '$font-data', size: 11, fill: '$ink-faint', width: 44 }),
        rect(`Timeline Row ${i + 1} Dot`, { width: 6, height: 6, fill: `$${tone === 'neutral' ? 'ink-faint' : tone}`, cornerRadius: 999 }),
        text(`Timeline Row ${i + 1} Kind`, kind, { font: '$font-body', size: 12, weight: '600', fill: '$ink' }),
        text(`Timeline Row ${i + 1} Meta`, meta, { font: '$font-data', size: 11, fill: '$ink-faint' }),
      ],
    })),
  ],
})

const drawerBody = frame('Drawer Body', {
  width: 'fill_container', fill: '$surface',
  layout: 'vertical', gap: 0,
  children: [approvalBanner, approvalEvidence, approvalReason, approvalActions, timelinePreview],
})

const detailDrawer = frame('Detail Drawer', {
  width: 420, height: 'fill_container', fill: '$surface',
  stroke: '$line', strokeWidth: 1,
  layout: 'vertical', gap: 0,
  children: [drawerHeader, drawerTabs, drawerBody],
})

// --- Content Split ---
const contentSplit = frame('Content Split', {
  width: 'fill_container', fill: '$canvas',
  layout: 'horizontal', gap: 12, padding: [0, 12, 12, 12], alignItems: 'stretch',
  children: [
    frame('Data Column', {
      width: 'fill_container',
      layout: 'vertical', gap: 8,
      children: [
        frame('Data Column Caption', {
          layout: 'horizontal', gap: 8, alignItems: 'baseline',
          children: [
            text('Data Column Caption Title', 'TASK RECORDS  ·  128', { font: '$font-data', size: 10, weight: '600', tracking: 0.06, fill: '$ink-soft' }),
            text('Data Column Caption Sort', 'UPDATED DESC ↓', { font: '$font-data', size: 10, tracking: 0.04, fill: '$ink-faint' }),
          ],
        }),
        tablePanel,
      ],
    }),
    detailDrawer,
  ],
})

// --- Status Bar (28h) ---
const statusBar = frame('Status Bar', {
  width: 'fill_container', height: 28, fill: '$rail',
  layout: 'horizontal', padding: [0, 16, 0, 16], gap: 20, alignItems: 'center',
  children: [
    frame('Status Left', {
      layout: 'horizontal', gap: 12, alignItems: 'center',
      children: [
        rect('Status Connected Dot', { width: 6, height: 6, fill: '$success', cornerRadius: 999 }),
        text('Status Connected Label', 'CONNECTED · Live · rev.1024', { font: '$font-data', size: 10, weight: '600', tracking: 0.06, fill: '$text-inverse' }),
      ],
    }),
    frame('Status Center', {
      layout: 'horizontal', gap: 16, alignItems: 'center',
      children: [
        text('Status Kafka', 'Kafka lag 42ms', { font: '$font-data', size: 10, fill: '$ink-faint' }),
        text('Status Sync', 'Sync OK', { font: '$font-data', size: 10, fill: '$ink-faint' }),
        text('Status Gateway', 'Gateway p95 128ms', { font: '$font-data', size: 10, fill: '$ink-faint' }),
      ],
    }),
    frame('Status Spacer', { width: 'fill_container' }),
    text('Status Env', 'env=experience  ·  build 20250903-21', { font: '$font-data', size: 10, fill: '$ink-faint' }),
  ],
})

// --- Main Column (right of icon rail) ---
const mainColumn = frame('Main Column', {
  width: 'fill_container', height: 'fill_container', fill: '$canvas',
  layout: 'vertical', gap: 0,
  children: [workspaceToolbar, metricStrip, contentSplit, statusBar],
})

// --- Workspace Body (icon rail + main) ---
const workspaceBody = frame('Workspace Body', {
  width: 'fill_container', height: 'fill_container',
  layout: 'horizontal', gap: 0,
  children: [iconRail, mainColumn],
})

// --- Top-level frame ---
const workspaceFrame = frame('Agent Workspace v1', {
  width: FRAME_W, height: FRAME_H, fill: '$canvas',
  layout: 'vertical', gap: 0, padding: 0, clip: true,
  children: [topBar, workspaceBody],
})

// -----------------------------------------------------------------------------
// Splice into document
// -----------------------------------------------------------------------------
const existingIdx = doc.children.findIndex(c => c.name === 'Agent Workspace v1')
if (existingIdx >= 0) {
  doc.children.splice(existingIdx, 1, workspaceFrame)
  console.log(`replaced existing frame at index ${existingIdx}`)
} else {
  doc.children.push(workspaceFrame)
  console.log(`appended new frame at index ${doc.children.length - 1}`)
}

const out = target
const tmp = `${out}.tmp-${process.pid}`
writeFileSync(tmp, JSON.stringify(doc, null, 2), 'utf8')
renameSync(tmp, out)
console.log(`wrote ${out}, total children=${doc.children.length}`)
