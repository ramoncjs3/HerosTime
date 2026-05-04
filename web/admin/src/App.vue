<script setup lang="ts">
import { Clock, Connection, Delete, Finished, Key, Plus, Refresh, Search, SwitchButton } from '@element-plus/icons-vue'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

type ActionName = 'bootstrap' | 'refresh' | 'check'
type ViewName = 'overview' | 'servers' | 'records' | 'settings'

interface ScheduleStatus {
  bootstrap_on_start: boolean
  refresh_sessions: string
  check_shop: string
}

interface AppStatus {
  name: string
  timezone: string
  dry_run: boolean
  state_file: string
  storage_type: string
  admin_addr: string
  qq_enabled: boolean
  wxpusher_enabled: boolean
  schedules: ScheduleStatus
  configured_server_count: number
}

interface VariantStatus {
  name: string
  kind: string
  enabled: boolean
  server_count: number
  servers: string[]
}

interface ExpansionStatus {
  variant_name: string
  kind: string
  server: string
  username_configured: boolean
  password_configured: boolean
  wxpusher_configured: boolean
  qq_configured: boolean
  created_at: string
  updated_at: string
}

interface ServerStateRow {
  key: string
  source: string
  server: string
  date: string
  status: string
  items?: string[]
  updated_at: string
  created_at?: string
}

interface SessionInfo {
  variant_name: string
  kind: string
  server_code: string
  role_id?: number
}

interface RuntimeSnapshot {
  sessions: SessionInfo[]
  session_count: number
}

interface QueuedActionStatus {
  name: string
  queued_at: string
}

interface ActionStatus {
  name?: string
  running: boolean
  started_at?: string
  finished_at?: string
  error?: string
  queued?: QueuedActionStatus[]
}

interface StatusResponse {
  now: string
  app: AppStatus
  notifications: {
    watch_items: string[]
  }
  variants: VariantStatus[]
  expansion: ExpansionStatus[]
  state: ServerStateRow[]
  events?: ServerStateRow[]
  runtime: RuntimeSnapshot
  action: ActionStatus
}

const loginMode = ref(window.location.pathname === '/login')
const loginForm = ref({ username: 'admin', password: '' })
const loginLoading = ref(false)
const status = ref<StatusResponse | null>(null)
const statusLoading = ref(false)
const expansionLoading = ref(false)
const expansionDeleting = ref('')
const actionSubmitting = ref<ActionName | ''>('')
const currentView = ref<ViewName>('overview')
const query = ref('')
const sourceFilter = ref('all')
const stateFilter = ref('all')
const autoRefresh = ref(true)
const lastLoadedAt = ref('')
const lastError = ref('')
const expansionForm = ref({
  variant_name: '',
  server: '',
  username: '',
  password: '',
  wxpusher_topic_id: '',
  qq_group_id: '',
})

let timer: number | undefined

const navItems: Array<{ id: ViewName; label: string; icon: string }> = [
  { id: 'overview', label: '运行总览', icon: 'Monitor' },
  { id: 'servers', label: '区服配置', icon: 'List' },
  { id: 'records', label: '推送记录', icon: 'DataAnalysis' },
  { id: 'settings', label: '调度通道', icon: 'Setting' },
]

const actionLabels: Record<ActionName, string> = {
  bootstrap: '完整检查',
  refresh: '刷新登录',
  check: '检查商店',
}

const activeVariants = computed(() => status.value?.variants.filter((item) => item.enabled) ?? [])
const disabledVariants = computed(() => status.value?.variants.filter((item) => !item.enabled) ?? [])

const channelList = computed(() => {
  if (!status.value) return []
  const channels = []
  if (status.value.app.wxpusher_enabled) channels.push('WxPusher')
  if (status.value.app.qq_enabled) channels.push('QQ')
  return channels
})

const todayKey = computed(() => {
  const now = new Date()
  const y = now.getFullYear()
  const m = String(now.getMonth() + 1).padStart(2, '0')
  const d = String(now.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
})

const todayRows = computed(() => status.value?.state.filter((row) => row.date === todayKey.value) ?? [])

const terminalTodayCount = computed(() =>
  todayRows.value.filter((row) => ['notified', 'watch_notified', 'event_over', 'dry_run'].includes(row.status)).length,
)

const statusCounts = computed(() => {
  const counts = new Map<string, number>()
  for (const row of status.value?.state ?? []) {
    counts.set(row.status, (counts.get(row.status) ?? 0) + 1)
  }
  return Array.from(counts.entries())
    .sort((a, b) => b[1] - a[1])
    .map(([name, count]) => ({ name, count }))
})

const recordRows = computed(() => {
  const events = status.value?.events ?? []
  return events.length > 0 ? events : (status.value?.state ?? [])
})

const sourceOptions = computed(() => {
  const set = new Set(recordRows.value.map((row) => row.source))
  return Array.from(set).sort()
})

const stateOptions = computed(() => {
  const set = new Set(recordRows.value.map((row) => row.status))
  return Array.from(set).sort()
})

const filteredStateRows = computed(() => {
  const needle = query.value.trim().toLowerCase()
  return recordRows.value.filter((row) => {
    if (sourceFilter.value !== 'all' && row.source !== sourceFilter.value) return false
    if (stateFilter.value !== 'all' && row.status !== stateFilter.value) return false
    if (!needle) return true
    const haystack = [row.key, row.source, row.server, row.status, row.date, ...(row.items ?? [])].join(' ').toLowerCase()
    return haystack.includes(needle)
  })
})

const sessionGroups = computed(() => {
  const groups = new Map<string, SessionInfo[]>()
  for (const session of status.value?.runtime.sessions ?? []) {
    const key = session.variant_name || session.kind
    groups.set(key, [...(groups.get(key) ?? []), session])
  }
  return Array.from(groups.entries()).map(([name, sessions]) => ({ name, sessions }))
})

const actionSummary = computed(() => {
  const action = status.value?.action
  if (!action) return '等待状态'
  const queuedCount = action.queued?.length ?? 0
  const queuedText = queuedCount > 0 ? `，${queuedCount} 个排队` : ''
  if (action.running) return `${action.name ?? '任务'}运行中${queuedText}`
  if (action.error) return action.error
  if (action.name) return `${action.name}已完成`
  return '空闲'
})

function cookieValue(name: string) {
  const prefix = `${encodeURIComponent(name)}=`
  const item = document.cookie
    .split(';')
    .map((part) => part.trim())
    .find((part) => part.startsWith(prefix))
  if (!item) return ''
  return decodeURIComponent(item.slice(prefix.length))
}

function csrfHeaders(): Record<string, string> {
  const token = cookieValue('oldbeggar_admin_csrf')
  return token ? { 'X-CSRF-Token': token } : {}
}

function queuedActionNames(action = status.value?.action) {
  return action?.queued?.map((item) => item.name) ?? []
}

function isActionQueued(action: ActionName) {
  return queuedActionNames().includes(actionLabels[action])
}

function actionButtonLabel(action: ActionName) {
  const label = actionLabels[action]
  return isActionQueued(action) ? `${label}已排队` : label
}

function actionButtonDisabled(action: ActionName) {
  return actionSubmitting.value !== '' && actionSubmitting.value !== action
}

async function login() {
  loginLoading.value = true
  lastError.value = ''
  try {
    const response = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify(loginForm.value),
    })
    const result = await response.json().catch(() => ({}))
    if (!response.ok || !result.ok) {
      throw new Error(result.error || '登录失败')
    }
    loginMode.value = false
    window.history.replaceState({}, '', '/')
    await loadStatus()
  } catch (error) {
    lastError.value = error instanceof Error ? error.message : '登录失败'
  } finally {
    loginLoading.value = false
  }
}

async function logout() {
  await fetch('/api/logout', { method: 'POST', headers: csrfHeaders(), credentials: 'same-origin' }).catch(() => undefined)
  status.value = null
  loginMode.value = true
  window.history.replaceState({}, '', '/login')
}

async function loadStatus(silent = false) {
  if (!silent) statusLoading.value = true
  try {
    const response = await fetch('/api/status', { credentials: 'same-origin' })
    if (response.status === 401) {
      loginMode.value = true
      window.history.replaceState({}, '', '/login')
      return
    }
    if (!response.ok) throw new Error('状态读取失败')
    status.value = await response.json()
    if (!expansionForm.value.variant_name) {
      expansionForm.value.variant_name =
        status.value?.variants.find((item) => item.enabled)?.name ?? status.value?.variants[0]?.name ?? ''
    }
    lastLoadedAt.value = formatDateTime(status.value?.now)
    lastError.value = ''
  } catch (error) {
    lastError.value = error instanceof Error ? error.message : '状态读取失败'
    if (!silent) ElMessage.error(lastError.value)
  } finally {
    statusLoading.value = false
  }
}

async function addExpansion() {
  const payload = {
    variant_name: expansionForm.value.variant_name,
    server: expansionForm.value.server.trim(),
    username: expansionForm.value.username.trim(),
    password: expansionForm.value.password.trim(),
    wxpusher_topic_id: expansionForm.value.wxpusher_topic_id ? Number(expansionForm.value.wxpusher_topic_id) : 0,
    qq_group_id: status.value?.app.qq_enabled ? expansionForm.value.qq_group_id.trim() : '',
  }
  if (!payload.variant_name || !payload.server) {
    ElMessage.error('请选择服种并填写新区区服')
    return
  }
  if (!Number.isFinite(payload.wxpusher_topic_id) || payload.wxpusher_topic_id < 0) {
    ElMessage.error('WxPusher topicId 必须是数字')
    return
  }

  expansionLoading.value = true
  try {
    const response = await fetch('/api/expansion', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...csrfHeaders() },
      credentials: 'same-origin',
      body: JSON.stringify(payload),
    })
    const result = (await response.json().catch(() => ({}))) as {
      ok?: boolean
      error?: string
      warning?: string
      action?: ActionStatus
    }
    if (!response.ok || !result.ok) {
      throw new Error(result.error || '新区保存失败')
    }
    const refreshQueued = queuedActionNames(result.action).includes(actionLabels.refresh)
    ElMessage.success(result.warning || (refreshQueued ? '新区已保存，刷新登录已加入队列' : '新区已保存，正在刷新登录'))
    expansionForm.value.server = ''
    expansionForm.value.username = ''
    expansionForm.value.password = ''
    expansionForm.value.wxpusher_topic_id = ''
    expansionForm.value.qq_group_id = ''
    await loadStatus(true)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '新区保存失败')
  } finally {
    expansionLoading.value = false
  }
}

async function deleteExpansion(row: ExpansionStatus) {
  try {
    await ElMessageBox.confirm(`确认移除 ${row.variant_name} / ${row.server}？`, '移除新区', {
      confirmButtonText: '移除',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }

  expansionDeleting.value = `${row.variant_name}:${row.server}`
  try {
    const response = await fetch('/api/expansion/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...csrfHeaders() },
      credentials: 'same-origin',
      body: JSON.stringify({ variant_name: row.variant_name, server: row.server }),
    })
    const result = (await response.json().catch(() => ({}))) as {
      ok?: boolean
      error?: string
      warning?: string
      action?: ActionStatus
    }
    if (!response.ok || !result.ok) {
      throw new Error(result.error || '新区移除失败')
    }
    const refreshQueued = queuedActionNames(result.action).includes(actionLabels.refresh)
    ElMessage.success(result.warning || (refreshQueued ? '新区已移除，刷新登录已加入队列' : '新区已移除，正在刷新登录'))
    await loadStatus(true)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '新区移除失败')
  } finally {
    expansionDeleting.value = ''
  }
}

async function runAction(action: ActionName) {
  const label = actionLabels[action]
  try {
    await ElMessageBox.confirm(`确认执行「${label}」？`, '手动任务', {
      confirmButtonText: '执行',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }

  actionSubmitting.value = action
  try {
    const response = await fetch(`/api/actions/${action}`, {
      method: 'POST',
      headers: csrfHeaders(),
      credentials: 'same-origin',
    })
    const result = (await response.json().catch(() => ({}))) as {
      ok?: boolean
      error?: string
      action?: ActionStatus
    }
    if (!response.ok || !result.ok) {
      throw new Error(result.error || '任务启动失败')
    }
    const queued = queuedActionNames(result.action).includes(label)
    ElMessage.success(queued ? `${label}已加入队列` : `${label}已启动`)
    await loadStatus(true)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '任务启动失败')
  } finally {
    actionSubmitting.value = ''
  }
}

function statusTagType(value: string) {
  switch (value) {
    case 'notified':
    case 'watch_notified':
      return 'success'
    case 'dry_run':
      return 'warning'
    case 'event_over':
      return 'info'
    default:
      return 'danger'
  }
}

function sourceLabel(source: string) {
  if (source === 'oldbeggar') return '老乞丐'
  if (source === 'notice_shop') return '告示牌奇货'
  return source
}

function formatDateTime(value?: string) {
  if (!value) return '--'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

function shortServers(servers: string[]) {
  return servers.slice(0, 48)
}

function toggleAutoRefresh(value: boolean) {
  autoRefresh.value = value
  ElMessage.info(value ? '已开启自动刷新' : '已暂停自动刷新')
}

onMounted(() => {
  if (!loginMode.value) {
    loadStatus()
  }
  timer = window.setInterval(() => {
    if (!loginMode.value && autoRefresh.value) {
      loadStatus(true)
    }
  }, 5000)
})

onUnmounted(() => {
  if (timer) window.clearInterval(timer)
})
</script>

<template>
  <main v-if="loginMode" class="login-page">
    <section class="login-copy">
      <div class="brand-mark">乞</div>
      <h1>老乞丐管理台</h1>
      <p>查看运行状态、推送记录和区服会话，手动触发刷新与检查。</p>
      <div class="login-notes">
        <span>账号登录</span>
        <span>Cookie 会话</span>
        <span>API 统一鉴权</span>
      </div>
    </section>

    <section class="login-panel">
      <h2>登录</h2>
      <el-alert v-if="lastError" :title="lastError" type="error" show-icon :closable="false" />
      <el-form label-position="top" @submit.prevent="login">
        <el-form-item label="账号">
          <el-input v-model="loginForm.username" size="large" autocomplete="username" :prefix-icon="Key" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="loginForm.password"
            size="large"
            type="password"
            autocomplete="current-password"
            show-password
            @keyup.enter="login"
          />
        </el-form-item>
        <el-button class="login-button" size="large" type="primary" :loading="loginLoading" @click="login">登录</el-button>
      </el-form>
    </section>
  </main>

  <div v-else class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <div class="brand-mark">乞</div>
        <div>
          <strong>老乞丐管理台</strong>
          <span>{{ status?.app.name ?? 'oldbeggar' }}</span>
        </div>
      </div>

      <nav class="nav-list">
        <button
          v-for="item in navItems"
          :key="item.id"
          type="button"
          class="nav-button"
          :class="{ active: currentView === item.id }"
          @click="currentView = item.id"
        >
          <el-icon><component :is="item.icon" /></el-icon>
          <span>{{ item.label }}</span>
        </button>
      </nav>

      <div class="sidebar-status">
        <div class="sidebar-row">
          <span>WxPusher</span>
          <el-tag size="small" :type="status?.app.wxpusher_enabled ? 'success' : 'info'">
            {{ status?.app.wxpusher_enabled ? '开启' : '关闭' }}
          </el-tag>
        </div>
        <div class="sidebar-row">
          <span>QQ</span>
          <el-tag size="small" :type="status?.app.qq_enabled ? 'success' : 'info'">
            {{ status?.app.qq_enabled ? '开启' : '关闭' }}
          </el-tag>
        </div>
      </div>

      <el-button class="logout-button" :icon="SwitchButton" @click="logout">退出</el-button>
    </aside>

    <section class="workspace">
      <header class="topbar">
        <div>
          <h1>{{ navItems.find((item) => item.id === currentView)?.label }}</h1>
          <p>
            更新时间：{{ lastLoadedAt || '--' }}
            <span v-if="lastError"> · {{ lastError }}</span>
          </p>
        </div>
        <div class="top-actions">
          <el-switch
            v-model="autoRefresh"
            inline-prompt
            active-text="自动"
            inactive-text="暂停"
            @change="toggleAutoRefresh"
          />
          <el-button :icon="Refresh" :loading="statusLoading" @click="loadStatus()">刷新</el-button>
          <el-button
            type="primary"
            :icon="Finished"
            :loading="actionSubmitting === 'bootstrap'"
            :disabled="actionButtonDisabled('bootstrap')"
            @click="runAction('bootstrap')"
          >
            {{ actionButtonLabel('bootstrap') }}
          </el-button>
        </div>
      </header>

      <el-skeleton v-if="!status" :rows="9" animated />

      <template v-else>
        <section v-show="currentView === 'overview'" class="view-stack">
          <div class="metric-grid">
            <div class="metric-tile">
              <span>运行模式</span>
              <strong>{{ status.app.dry_run ? '演练' : '正式' }}</strong>
              <small>通道：{{ channelList.length ? channelList.join(' / ') : '未启用' }}</small>
            </div>
            <div class="metric-tile">
              <span>当前会话</span>
              <strong>{{ status.runtime.session_count }}</strong>
              <small>配置区服：{{ status.app.configured_server_count }}</small>
            </div>
            <div class="metric-tile">
              <span>今日处理</span>
              <strong>{{ terminalTodayCount }}</strong>
              <small>今日记录：{{ todayRows.length }}</small>
            </div>
            <div class="metric-tile">
              <span>手动任务</span>
              <strong>{{ status.action.running ? (status.action.queued?.length ? '运行/排队' : '运行中') : '空闲' }}</strong>
              <small>{{ actionSummary }}</small>
            </div>
          </div>

          <div class="content-grid">
            <section class="panel">
              <div class="panel-head">
                <div>
                  <h2>手动操作</h2>
                  <p>已有任务运行时会自动排队，按顺序执行。</p>
                </div>
              </div>
              <div class="action-row">
                <el-button
                  :icon="Refresh"
                  :loading="actionSubmitting === 'refresh'"
                  :disabled="actionButtonDisabled('refresh')"
                  @click="runAction('refresh')"
                >
                  {{ actionButtonLabel('refresh') }}
                </el-button>
                <el-button
                  :icon="Search"
                  :loading="actionSubmitting === 'check'"
                  :disabled="actionButtonDisabled('check')"
                  @click="runAction('check')"
                >
                  {{ actionButtonLabel('check') }}
                </el-button>
                <el-button
                  type="primary"
                  :icon="Finished"
                  :loading="actionSubmitting === 'bootstrap'"
                  :disabled="actionButtonDisabled('bootstrap')"
                  @click="runAction('bootstrap')"
                >
                  {{ actionButtonLabel('bootstrap') }}
                </el-button>
              </div>
              <el-alert
                v-if="status.action.error"
                :title="status.action.error"
                type="error"
                show-icon
                :closable="false"
                class="mt-16"
              />
              <div class="detail-list mt-16">
                <div>
                  <span>开始时间</span>
                  <strong>{{ formatDateTime(status.action.started_at) }}</strong>
                </div>
                <div>
                  <span>完成时间</span>
                  <strong>{{ formatDateTime(status.action.finished_at) }}</strong>
                </div>
                <div v-if="status.action.queued?.length">
                  <span>排队任务</span>
                  <div class="queue-list">
                    <span v-for="item in status.action.queued" :key="`${item.name}:${item.queued_at}`" class="queue-pill">
                      {{ item.name }} · {{ formatDateTime(item.queued_at) }}
                    </span>
                  </div>
                </div>
              </div>
            </section>

            <section class="panel">
              <div class="panel-head">
                <div>
                  <h2>状态分布</h2>
                  <p>来自持久化状态文件。</p>
                </div>
              </div>
              <div class="status-bars">
                <div v-for="item in statusCounts" :key="item.name" class="status-bar">
                  <div>
                    <el-tag size="small" :type="statusTagType(item.name)">{{ item.name }}</el-tag>
                    <span>{{ item.count }}</span>
                  </div>
                  <meter :value="item.count" :max="status.state.length || 1" />
                </div>
              </div>
            </section>
          </div>

          <section class="panel">
            <div class="panel-head">
              <div>
                <h2>当前会话</h2>
                <p>启动自检或刷新登录后更新。</p>
              </div>
              <el-tag>{{ status.runtime.session_count }} 个</el-tag>
            </div>
            <div class="session-grid">
              <div v-for="group in sessionGroups" :key="group.name" class="session-group">
                <div class="session-title">
                  <strong>{{ group.name }}</strong>
                  <span>{{ group.sessions.length }} 个</span>
                </div>
                <div class="chip-row">
                  <span v-for="session in group.sessions" :key="session.server_code" class="server-chip">
                    {{ session.server_code }}
                  </span>
                </div>
              </div>
            </div>
          </section>
        </section>

        <section v-show="currentView === 'servers'" class="view-stack">
          <div class="split-head">
            <div>
              <h2>启用服种</h2>
              <p>{{ activeVariants.length }} 类启用，{{ disabledVariants.length }} 类停用。</p>
            </div>
            <el-tag type="success">{{ status.app.configured_server_count }} 个区服</el-tag>
          </div>

          <section class="panel">
            <div class="panel-head">
              <div>
                <h2>新区扩容</h2>
                <p>保存后会写入动态区服文件，并自动触发一次刷新登录。</p>
              </div>
              <el-tag>{{ status.expansion.length }} 个动态区服</el-tag>
            </div>
            <div class="expansion-form">
              <el-select v-model="expansionForm.variant_name" class="expansion-select" placeholder="选择服种">
                <el-option
                  v-for="variant in status.variants"
                  :key="variant.name"
                  :label="`${variant.name} · ${variant.kind}`"
                  :value="variant.name"
                />
              </el-select>
              <el-input v-model="expansionForm.server" class="expansion-server" clearable placeholder="新区区服，如 h12 / g99" />
              <el-input v-model="expansionForm.username" class="expansion-account" clearable placeholder="账号，留空使用服种默认账号" />
              <el-input
                v-model="expansionForm.password"
                class="expansion-account"
                type="password"
                show-password
                clearable
                placeholder="密码，留空使用服种默认密码"
              />
              <el-input
                v-model="expansionForm.wxpusher_topic_id"
                class="expansion-account"
                clearable
                placeholder="WxPusher topicId"
              />
              <el-input
                v-if="status.app.qq_enabled"
                v-model="expansionForm.qq_group_id"
                class="expansion-account"
                clearable
                placeholder="QQ 群号，可选"
              />
              <el-button type="primary" :icon="Plus" :loading="expansionLoading" @click="addExpansion">保存并刷新</el-button>
            </div>
            <div v-if="status.expansion.length" class="expansion-list">
              <div v-for="item in status.expansion" :key="`${item.variant_name}:${item.server}`" class="expansion-item">
                <div>
                  <strong>{{ item.server }}</strong>
                  <span>{{ item.variant_name }} · {{ item.kind }}</span>
                </div>
                <div class="expansion-meta">
                  <el-tag size="small" :type="item.username_configured || item.password_configured ? 'warning' : 'info'">
                    {{ item.username_configured || item.password_configured ? '独立账号' : '默认账号' }}
                  </el-tag>
                  <el-tag size="small" :type="item.wxpusher_configured ? 'success' : 'info'">
                    {{ item.wxpusher_configured ? 'WxPusher' : '无 WxPusher' }}
                  </el-tag>
                  <el-tag v-if="status.app.qq_enabled" size="small" :type="item.qq_configured ? 'success' : 'info'">
                    {{ item.qq_configured ? 'QQ' : '无 QQ' }}
                  </el-tag>
                  <span>{{ formatDateTime(item.created_at) }}</span>
                  <el-button
                    link
                    type="danger"
                    :icon="Delete"
                    :loading="expansionDeleting === `${item.variant_name}:${item.server}`"
                    @click="deleteExpansion(item)"
                  >
                    移除
                  </el-button>
                </div>
              </div>
            </div>
          </section>

          <section v-for="variant in status.variants" :key="variant.name" class="panel">
            <div class="panel-head">
              <div>
                <h2>{{ variant.name }}</h2>
                <p>{{ variant.kind }} · {{ variant.server_count }} 个区服</p>
              </div>
              <el-tag :type="variant.enabled ? 'success' : 'info'">{{ variant.enabled ? '启用' : '停用' }}</el-tag>
            </div>
            <div class="chip-row">
              <span v-for="server in shortServers(variant.servers)" :key="server" class="server-chip">{{ server }}</span>
              <span v-if="variant.servers.length > 48" class="server-chip muted">+{{ variant.servers.length - 48 }}</span>
            </div>
          </section>
        </section>

        <section v-show="currentView === 'records'" class="view-stack">
          <section class="panel">
            <div class="panel-head compact">
              <div>
                <h2>推送记录</h2>
                <p>{{ filteredStateRows.length }} / {{ recordRows.length }} 条</p>
              </div>
              <div class="filter-row">
                <el-input v-model="query" class="search-input" :prefix-icon="Search" clearable placeholder="搜索区服、商品、状态" />
                <el-select v-model="sourceFilter" class="filter-select">
                  <el-option label="全部来源" value="all" />
                  <el-option v-for="source in sourceOptions" :key="source" :label="sourceLabel(source)" :value="source" />
                </el-select>
                <el-select v-model="stateFilter" class="filter-select">
                  <el-option label="全部状态" value="all" />
                  <el-option v-for="item in stateOptions" :key="item" :label="item" :value="item" />
                </el-select>
              </div>
            </div>

            <el-table :data="filteredStateRows" height="620" stripe>
              <el-table-column prop="server" label="区服" width="110" fixed />
              <el-table-column label="来源" width="130">
                <template #default="{ row }">{{ sourceLabel(row.source) }}</template>
              </el-table-column>
              <el-table-column label="状态" width="150">
                <template #default="{ row }">
                  <el-tag :type="statusTagType(row.status)">{{ row.status }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="date" label="日期" width="130" />
              <el-table-column label="商品" min-width="240">
                <template #default="{ row }">
                  <span class="items-text">{{ row.items?.length ? row.items.join('、') : '--' }}</span>
                </template>
              </el-table-column>
              <el-table-column label="更新时间" width="190">
                <template #default="{ row }">{{ formatDateTime(row.updated_at) }}</template>
              </el-table-column>
            </el-table>
          </section>
        </section>

        <section v-show="currentView === 'settings'" class="view-stack">
          <div class="content-grid">
            <section class="panel">
              <div class="panel-head">
                <div>
                  <h2>调度</h2>
                  <p>当前仍由 Go 程序内的 cron 执行。</p>
                </div>
                <el-icon><Clock /></el-icon>
              </div>
              <div class="detail-list">
                <div>
                  <span>启动自检</span>
                  <strong>{{ status.app.schedules.bootstrap_on_start ? '开启' : '关闭' }}</strong>
                </div>
                <div>
                  <span>刷新登录</span>
                  <strong>{{ status.app.schedules.refresh_sessions }}</strong>
                </div>
                <div>
                  <span>检查商店</span>
                  <strong>{{ status.app.schedules.check_shop }}</strong>
                </div>
              </div>
            </section>

            <section class="panel">
              <div class="panel-head">
                <div>
                  <h2>通道</h2>
                  <p>敏感 token 不会返回到前端。</p>
                </div>
                <el-icon><Connection /></el-icon>
              </div>
              <div class="detail-list">
                <div>
                  <span>WxPusher</span>
                  <strong>{{ status.app.wxpusher_enabled ? '开启' : '关闭' }}</strong>
                </div>
                <div>
                  <span>QQ</span>
                  <strong>{{ status.app.qq_enabled ? '开启' : '关闭' }}</strong>
                </div>
                <div>
                  <span>后台地址</span>
                  <strong>{{ status.app.admin_addr }}</strong>
                </div>
              </div>
            </section>
          </div>

          <section class="panel">
            <div class="panel-head">
              <div>
                <h2>重点商品</h2>
                <p>命中这些商品时会按重点提醒推送。</p>
              </div>
              <el-tag type="success">{{ status.notifications.watch_items.length }} 个</el-tag>
            </div>
            <div class="chip-row">
              <span v-for="item in status.notifications.watch_items" :key="item" class="watch-chip">{{ item }}</span>
            </div>
          </section>

          <section class="panel">
            <div class="panel-head">
              <div>
                <h2>文件</h2>
                <p>只显示路径，不暴露账号、密码和 token。</p>
              </div>
            </div>
            <div class="detail-list">
              <div>
                <span>存储后端</span>
                <strong>{{ status.app.storage_type }}</strong>
              </div>
              <div>
                <span>状态文件</span>
                <strong>{{ status.app.state_file }}</strong>
              </div>
              <div>
                <span>时区</span>
                <strong>{{ status.app.timezone }}</strong>
              </div>
            </div>
          </section>
        </section>
      </template>
    </section>
  </div>
</template>
