<script setup>
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { 
  Database, Users, FileText, ChevronRight, Trash2, Plus, Copy, Check, ArrowLeft, LogOut, Shield, RefreshCw, Key, AlertTriangle, Upload
} from '@lucide/vue'

// --- Reactive State ---
const currentView = ref('projects') // 'projects', 'project-detail', 'table-detail', 'auth', 'docs'
const stats = ref({ projects: 0, tables: 0, users: 0 })
const logs = ref([])
const projectsList = ref([])
const activeProject = ref(null)
const tablesList = ref(ref([]))
const activeTable = ref(null)

// Table details state
const tableColumns = ref([])
const tablePolicies = ref([])
const tableRows = ref([])
const activeTab = ref('data') // 'data', 'schema', 'rls', 'danger'

// Auth view state
const projectUsers = ref([])

// Form states
const newProjectName = ref('')
const newTableName = ref('')
const newColName = ref('')
const newColType = ref('text')
const newColNullable = ref(true)
const newUserName = ref('')
const newUserPassword = ref('')
const newPolicyAction = ref('SELECT')
const newPolicyRole = ref('authenticated')
const newPolicyExpression = ref('auth.uid() = user_id')
const importJSONData = ref('')
const showImportPanel = ref(false)

// Notifications and tooltips
const copiedItem = ref(null)
const systemError = ref('')

// WebSocket status
const wsStatus = ref('disconnected')
let wsConnection = null

// --- Clipboard Copy Helper ---
function copyToClipboard(text, id) {
  navigator.clipboard.writeText(text).then(() => {
    copiedItem.value = id
    setTimeout(() => {
      if (copiedItem.value === id) copiedItem.value = null
    }, 1500)
  })
}

// --- API Fetch Functions ---
async function fetchStatsAndLogs() {
  try {
    const statsRes = await fetch('/dashboard/api/stats')
    if (statsRes.ok) stats.value = await statsRes.json()

    const logsRes = await fetch('/dashboard/api/logs')
    if (logsRes.ok) logs.value = await logsRes.json()
  } catch (err) {
    console.error('Gagal mengambil stats & logs:', err)
  }
}

async function fetchProjects() {
  try {
    const res = await fetch('/dashboard/api/projects')
    if (res.ok) projectsList.value = await res.json()
  } catch (err) {
    console.error('Gagal mengambil daftar proyek:', err)
  }
}

async function selectProject(proj) {
  try {
    const res = await fetch(`/dashboard/api/projects/${proj.id}`)
    if (res.ok) {
      const data = await res.json()
      activeProject.value = data.project
      tablesList.value = data.tables
      currentView.value = 'project-detail'
      activeTable.value = null
    }
  } catch (err) {
    console.error('Gagal memuat detail proyek:', err)
  }
}

async function createProject() {
  if (!newProjectName.value.trim()) return
  try {
    const res = await fetch('/dashboard/api/projects', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: newProjectName.value })
    })
    if (res.ok) {
      newProjectName.value = ''
      await fetchProjects()
      await fetchStatsAndLogs()
    }
  } catch (err) {
    console.error('Gagal membuat proyek:', err)
  }
}

async function createTable() {
  if (!newTableName.value.trim() || !activeProject.value) return
  try {
    const res = await fetch(`/dashboard/api/projects/${activeProject.value.id}/tables`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: newTableName.value })
    })
    if (res.ok) {
      newTableName.value = ''
      await selectProject(activeProject.value)
      await fetchStatsAndLogs()
    } else {
      const errMsg = await res.text()
      alert('Gagal membuat tabel: ' + errMsg)
    }
  } catch (err) {
    console.error('Gagal membuat tabel:', err)
  }
}

async function deleteTable() {
  if (!activeTable.value || !activeProject.value) return
  if (!confirm(`Apakah Anda yakin ingin menghapus tabel "${activeTable.value.name}" secara permanen? Seluruh data fisik, kolom, dan policy akan hilang.`)) return
  
  try {
    const res = await fetch(`/dashboard/api/projects/${activeProject.value.id}/tables/${activeTable.value.id}/delete`, {
      method: 'POST'
    })
    if (res.ok) {
      const currentProj = activeProject.value
      currentView.value = 'project-detail'
      activeTable.value = null
      await selectProject(currentProj)
      await fetchStatsAndLogs()
    }
  } catch (err) {
    console.error('Gagal menghapus tabel:', err)
  }
}

async function selectTable(table) {
  try {
    const res = await fetch(`/dashboard/api/projects/${activeProject.value.id}/tables/${table.id}`)
    if (res.ok) {
      const data = await res.json()
      activeTable.value = data.table
      tableColumns.value = data.columns
      tablePolicies.value = data.policies
      tableRows.value = data.rows || []
      currentView.value = 'table-detail'
      activeTab.value = 'data'
      showImportPanel.value = false
    }
  } catch (err) {
    console.error('Gagal memuat detail tabel:', err)
  }
}

async function addColumn() {
  if (!newColName.value.trim() || !activeTable.value) return
  try {
    const res = await fetch(`/dashboard/api/projects/${activeProject.value.id}/tables/${activeTable.value.id}/columns`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: newColName.value,
        type: newColType.value,
        is_nullable: newColNullable.value
      })
    })
    if (res.ok) {
      newColName.value = ''
      await selectTable(activeTable.value)
    } else {
      const msg = await res.text()
      alert('Gagal menambah kolom: ' + msg)
    }
  } catch (err) {
    console.error(err)
  }
}

async function deleteColumn(col) {
  if (!confirm(`Apakah Anda yakin ingin menghapus kolom "${col.name}"? Data di kolom ini akan dihapus permanen.`)) return
  try {
    const res = await fetch(`/dashboard/api/projects/${activeProject.value.id}/tables/${activeTable.value.id}/columns/${col.id}/delete`, {
      method: 'POST'
    })
    if (res.ok) {
      await selectTable(activeTable.value)
    }
  } catch (err) {
    console.error(err)
  }
}

async function deleteRow(rowId) {
  if (!confirm('Apakah Anda yakin ingin menghapus data dengan ID ini?')) return
  try {
    const res = await fetch(`/dashboard/api/projects/${activeProject.value.id}/tables/${activeTable.value.id}/rows/${rowId}/delete`, {
      method: 'POST'
    })
    if (res.ok) {
      // Baris lokal akan ter-update via WebSocket jika aktif, tapi panggil fallback
      await selectTable(activeTable.value)
    }
  } catch (err) {
    console.error(err)
  }
}

async function importJSON() {
  if (!importJSONData.value.trim()) return
  try {
    const res = await fetch(`/dashboard/api/projects/${activeProject.value.id}/tables/${activeTable.value.id}/import-json`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ json_data: importJSONData.value })
    })
    if (res.ok) {
      const data = await res.json()
      alert(`Berhasil mengimpor ${data.imported} baris data unik!`)
      importJSONData.value = ''
      showImportPanel.value = false
      await selectTable(activeTable.value)
    } else {
      const msg = await res.text()
      alert('Gagal import JSON: ' + msg)
    }
  } catch (err) {
    console.error(err)
  }
}

// --- Auth Console API ---
async function openAuthConsole() {
  try {
    const res = await fetch(`/dashboard/api/projects/${activeProject.value.id}/users`)
    if (res.ok) {
      projectUsers.value = await res.json()
      currentView.value = 'auth'
    }
  } catch (err) {
    console.error(err)
  }
}

async function createUser() {
  if (!newUserName.value.trim() || !newUserPassword.value.trim()) return
  try {
    const res = await fetch(`/dashboard/api/projects/${activeProject.value.id}/users`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: newUserName.value, password: newUserPassword.value })
    })
    if (res.ok) {
      newUserName.value = ''
      newUserPassword.value = ''
      await openAuthConsole()
      await fetchStatsAndLogs()
    } else {
      const msg = await res.text()
      alert('Gagal menambah user: ' + msg)
    }
  } catch (err) {
    console.error(err)
  }
}

// --- RLS Policies API ---
async function createPolicy() {
  if (!activeTable.value) return
  try {
    const res = await fetch(`/dashboard/api/projects/${activeProject.value.id}/tables/${activeTable.value.id}/policies`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        action: newPolicyAction.value,
        role: newPolicyRole.value,
        expression: newPolicyExpression.value
      })
    })
    if (res.ok) {
      await selectTable(activeTable.value)
    } else {
      const msg = await res.text()
      alert('Gagal menyimpan policy: ' + msg)
    }
  } catch (err) {
    console.error(err)
  }
}

async function deletePolicy(policyId) {
  try {
    const res = await fetch(`/dashboard/api/projects/${activeProject.value.id}/tables/${activeTable.value.id}/policies/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ policy_id: policyId })
    })
    if (res.ok) {
      await selectTable(activeTable.value)
    }
  } catch (err) {
    console.error(err)
  }
}

// --- WebSockets Realtime Manager ---
function connectWebSocket() {
  if (wsConnection) wsConnection.close()

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/api/v1/realtime`

  wsConnection = new WebSocket(wsUrl)
  wsStatus.value = 'connecting'

  wsConnection.onopen = () => {
    wsStatus.value = 'connected'
    console.log('WebSocket terhubung ke Coderbase Hub.')
  }

  wsConnection.onclose = () => {
    wsStatus.value = 'disconnected'
    console.log('WebSocket terputus. Mencoba menghubungkan kembali dalam 5 detik...')
    setTimeout(connectWebSocket, 5000)
  }

  wsConnection.onerror = (err) => {
    wsStatus.value = 'error'
    console.error('WebSocket Error:', err)
  }

  wsConnection.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      // Struktur event realtime: { project_id, table_name, action, record }
      if (activeProject.value && msg.project_id === activeProject.value.id) {
        if (activeTable.value && msg.table_name === activeTable.value.name) {
          handleRealtimeEvent(msg.action, msg.record)
        }
        // Perbarui log trafik di dashboard utama secara langsung jika ada aksi
        fetchStatsAndLogs()
      }
    } catch (err) {
      console.error('Gagal memproses pesan WebSocket:', err)
    }
  }
}

function handleRealtimeEvent(action, record) {
  console.log(`Menerima event Realtime: ${action}`, record)
  
  if (action === 'INSERT') {
    // Hindari duplikasi jika sudah di-insert secara lokal
    if (!tableRows.value.some(r => r.id === record.id)) {
      // Tambahkan baris baru ke paling atas array
      tableRows.value.unshift(record)
    }
  } else if (action === 'UPDATE') {
    const idx = tableRows.value.findIndex(r => r.id === record.id)
    if (idx !== -1) {
      tableRows.value[idx] = record
    }
  } else if (action === 'DELETE') {
    tableRows.value = tableRows.value.filter(r => r.id !== record.id)
  }
}

// --- Lifecycle Hooks ---
onMounted(() => {
  fetchStatsAndLogs()
  fetchProjects()
  connectWebSocket()
})

onUnmounted(() => {
  if (wsConnection) wsConnection.close()
})

// --- Watchers to sync views on change ---
watch(currentView, (newVal) => {
  if (newVal === 'projects') {
    activeProject.value = null
    activeTable.value = null
    fetchProjects()
    fetchStatsAndLogs()
  }
})
</script>

<template>
  <div class="h-screen flex flex-col bg-[#121212] text-zinc-300 font-sans antialiased overflow-hidden">
    <!-- Navbar -->
    <nav class="h-14 bg-[#161616] border-b border-white/5 flex items-center justify-between px-6 z-40 shrink-0 select-none">
      <div class="flex items-center gap-3">
        <a href="#" @click.prevent="currentView = 'projects'" class="flex items-center gap-2.5 font-semibold text-lg text-white">
          <svg class="h-5 w-5 text-emerald-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <ellipse cx="12" cy="5" rx="9" ry="3"></ellipse>
            <path d="M3 5V19A9 3 0 0 0 21 19V5"></path>
            <path d="M3 12A9 3 0 0 0 21 12"></path>
          </svg>
          <span class="tracking-tight text-xl font-bold text-white">Coderbase</span>
          <span class="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">STUDIO</span>
        </a>
      </div>
      <div class="flex items-center gap-4 text-xs">
        <span class="flex items-center gap-2 bg-zinc-900 border border-white/5 px-2.5 py-1 rounded-full text-zinc-400">
          <span class="h-1.5 w-1.5 rounded-full" :class="wsStatus === 'connected' ? 'bg-emerald-500 animate-pulse' : 'bg-rose-500'"></span>
          <span>Realtime: {{ wsStatus }}</span>
        </span>
        <form action="/dashboard/logout" method="POST" class="inline">
          <button type="submit" class="text-zinc-500 hover:text-rose-400 transition font-semibold px-2.5 py-1 rounded hover:bg-rose-500/10 flex items-center gap-1.5">
            <LogOut class="h-3.5 w-3.5" />
            Logout
          </button>
        </form>
      </div>
    </nav>

    <!-- Main Container -->
    <div class="flex-1 flex overflow-hidden">
      <!-- Sidebar -->
      <aside class="w-60 bg-[#161616] border-r border-white/5 flex flex-col shrink-0">
        <div class="flex-1 py-6 px-4 space-y-7 overflow-y-auto">
          <div v-if="activeProject" class="space-y-1.5">
            <div class="px-3 mb-4 border-b border-white/5 pb-3">
              <span class="text-[10px] font-bold text-zinc-500 uppercase tracking-wider block">Project Console</span>
              <span class="text-sm font-bold text-white truncate block mt-1" :title="activeProject.name">{{ activeProject.name }}</span>
            </div>
            
            <a href="#" @click.prevent="selectProject(activeProject)" class="flex items-center gap-3 px-3 py-2 text-sm font-semibold rounded-lg transition" :class="currentView === 'project-detail' || currentView === 'table-detail' ? 'text-white bg-zinc-800/40' : 'text-zinc-400 hover:text-white hover:bg-zinc-800/20'">
              <Database class="h-4 w-4 text-emerald-500" />
              Database Tables
            </a>
            <a href="#" @click.prevent="currentView = 'docs'" class="flex items-center gap-3 px-3 py-2 text-sm font-semibold rounded-lg transition" :class="currentView === 'docs' ? 'text-white bg-zinc-800/40' : 'text-zinc-400 hover:text-white hover:bg-zinc-800/20'">
              <FileText class="h-4 w-4 text-zinc-400" />
              API Docs (Swagger)
            </a>
            <a href="#" @click.prevent="openAuthConsole" class="flex items-center gap-3 px-3 py-2 text-sm font-semibold rounded-lg transition" :class="currentView === 'auth' ? 'text-white bg-zinc-800/40' : 'text-zinc-400 hover:text-white hover:bg-zinc-800/20'">
              <Users class="h-4 w-4 text-zinc-400" />
              Authentication (Users)
            </a>

            <div class="border-t border-white/5 my-6 pt-4">
              <button @click="currentView = 'projects'" class="flex items-center gap-2 px-3 py-2 text-xs font-semibold rounded-lg text-zinc-500 hover:text-zinc-300 transition w-full text-left">
                <ArrowLeft class="h-3.5 w-3.5" />
                Back to Projects
              </button>
            </div>
          </div>

          <div v-else class="space-y-1.5">
            <span class="text-[10px] font-bold text-zinc-500 uppercase tracking-wider px-3">Navigation</span>
            <a href="#" @click.prevent="currentView = 'projects'" class="flex items-center gap-3 px-3 py-2 text-sm font-semibold rounded-lg text-white bg-zinc-800/40 transition">
              <Database class="h-4 w-4 text-emerald-500" />
              Dashboard Overview
            </a>
          </div>
        </div>
        <div class="p-4 border-t border-white/5 text-[10px] text-zinc-600 font-medium select-none">
          Coderbase Studio Engine v2.0 (Vue)
        </div>
      </aside>

      <!-- Main Content View -->
      <main class="flex-1 overflow-y-auto p-8">
        <div class="max-w-6xl mx-auto">
          
          <!-- --- VIEW: PROJECTS (OVERVIEW LIST) --- -->
          <div v-if="currentView === 'projects'" class="space-y-8">
            <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
              <div>
                <h1 class="text-3xl font-extrabold text-white tracking-tight">Projects Console</h1>
                <p class="mt-1 text-sm text-zinc-500">Kelola credentials proyek dan struktur database di bawah ini.</p>
              </div>
              <div class="shrink-0">
                <form @submit.prevent="createProject" class="flex gap-2.5">
                  <input type="text" v-model="newProjectName" required placeholder="Nama Project Baru" class="rounded-lg bg-zinc-900 border border-white/10 px-4 py-2 text-sm text-white focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500/20 transition">
                  <button type="submit" class="inline-flex items-center gap-1.5 rounded-lg bg-emerald-500 hover:bg-emerald-400 px-4 py-2 text-sm font-semibold text-zinc-950 shadow-md transition duration-200 cursor-pointer">
                    <Plus class="h-4 w-4" />
                    Create Project
                  </button>
                </form>
              </div>
            </div>

            <!-- Stats Grid -->
            <div class="grid grid-cols-1 gap-5 sm:grid-cols-3 mb-8 select-none">
              <div class="bg-zinc-900/40 border border-white/5 p-6 rounded-xl flex flex-col justify-between hover:border-zinc-700 transition">
                <span class="text-[10px] font-bold text-zinc-500 uppercase tracking-wider">Total Projects</span>
                <span class="text-4xl font-black text-white mt-2">{{ stats.projects }}</span>
              </div>
              <div class="bg-zinc-900/40 border border-white/5 p-6 rounded-xl flex flex-col justify-between hover:border-zinc-700 transition">
                <span class="text-[10px] font-bold text-zinc-500 uppercase tracking-wider">Total Tables</span>
                <span class="text-4xl font-black text-emerald-400 mt-2">{{ stats.tables }}</span>
              </div>
              <div class="bg-zinc-900/40 border border-white/5 p-6 rounded-xl flex flex-col justify-between hover:border-zinc-700 transition">
                <span class="text-[10px] font-bold text-zinc-500 uppercase tracking-wider">Total Users</span>
                <span class="text-4xl font-black text-white mt-2">{{ stats.users }}</span>
              </div>
            </div>

            <!-- Content Grid: Proyek & Logs -->
            <div class="grid grid-cols-1 lg:grid-cols-3 gap-8">
              <!-- Projects List -->
              <div class="lg:col-span-2 rounded-xl bg-zinc-900/40 border border-white/5 overflow-hidden flex flex-col">
                <div class="p-5 border-b border-white/5 bg-zinc-900/40 flex justify-between items-center select-none">
                  <h3 class="text-xs font-bold text-white uppercase tracking-wider">Active Projects</h3>
                  <span class="text-xs text-zinc-500">Total: {{ projectsList.length }}</span>
                </div>
                <div class="overflow-x-auto">
                  <table class="w-full text-left text-xs border-collapse">
                    <thead>
                      <tr class="bg-zinc-900/80 text-zinc-400 uppercase tracking-wider text-[9px] border-b border-white/5 font-bold">
                        <th class="p-4">Project Name</th>
                        <th class="p-4">Project ID</th>
                        <th class="p-4">API Key</th>
                        <th class="p-4 text-right">Actions</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-white/5">
                      <tr v-for="(p, i) in projectsList" :key="p.id" class="hover:bg-zinc-800/20 transition">
                        <td class="p-4 font-bold text-white text-sm">{{ p.name }}</td>
                        <td class="p-4 font-mono text-zinc-500">
                          <div class="flex items-center gap-2">
                            <span class="max-w-[120px] truncate" :title="p.id">{{ p.id }}</span>
                            <button @click="copyToClipboard(p.id, `p-id-${i}`)" class="text-zinc-600 hover:text-zinc-400 transition" title="Copy ID">
                              <Copy class="h-3.5 w-3.5" />
                            </button>
                            <span v-if="copiedItem === `p-id-${i}`" class="text-[9px] text-emerald-400 font-semibold font-sans">Copied!</span>
                          </div>
                        </td>
                        <td class="p-4 font-mono text-emerald-400/80">
                          <div class="flex items-center gap-2">
                            <span class="max-w-[200px] truncate" :title="p.api_key">{{ p.api_key }}</span>
                            <button @click="copyToClipboard(p.api_key, `p-key-${i}`)" class="text-zinc-600 hover:text-zinc-400 transition" title="Copy Key">
                              <Copy class="h-3.5 w-3.5" />
                            </button>
                            <span v-if="copiedItem === `p-key-${i}`" class="text-[9px] text-emerald-400 font-semibold font-sans">Copied!</span>
                          </div>
                        </td>
                        <td class="p-4 text-right">
                          <button @click="selectProject(p)" class="inline-flex items-center rounded-md bg-zinc-800 hover:bg-zinc-700 px-3.5 py-1.5 text-xs font-bold text-zinc-300 border border-white/5 transition cursor-pointer">
                            Console →
                          </button>
                        </td>
                      </tr>
                      <tr v-if="projectsList.length === 0">
                        <td colspan="4" class="p-12 text-center text-zinc-500">
                          Belum ada project terdaftar. Masukkan nama project di sebelah kanan atas!
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>

              <!-- Systems Logs -->
              <div class="lg:col-span-1 bg-zinc-900/40 border border-white/5 rounded-xl flex flex-col overflow-hidden self-start">
                <div class="p-5 border-b border-white/5 bg-zinc-900/40 flex justify-between items-center select-none">
                  <h3 class="text-xs font-bold text-white uppercase tracking-wider">Live System Logs</h3>
                  <span class="flex items-center gap-1.5 text-xs text-emerald-400 font-semibold">
                    <span class="h-2 w-2 rounded-full bg-emerald-500 animate-ping"></span>
                    <span>Live Traffic</span>
                  </span>
                </div>
                <div class="max-h-[380px] overflow-y-auto divide-y divide-white/5">
                  <div v-for="(log, idx) in logs" :key="idx" class="p-4 hover:bg-zinc-800/10 transition text-[11px] font-mono space-y-1">
                    <div class="flex items-center justify-between">
                      <span class="px-1.5 py-0.5 rounded text-[8px] font-bold text-zinc-950 uppercase" :class="{
                        'bg-blue-500': log.method === 'GET',
                        'bg-emerald-500': log.method === 'POST',
                        'bg-amber-500': log.method === 'PATCH',
                        'bg-rose-500': log.method === 'DELETE'
                      }">
                        {{ log.method }}
                      </span>
                      <span :class="log.status < 300 ? 'text-emerald-400' : 'text-rose-400'" class="font-bold">
                        {{ log.status }}
                      </span>
                    </div>
                    <div class="text-zinc-300 truncate" :title="log.path">{{ log.path }}</div>
                    <div class="text-zinc-550 flex items-center justify-between text-[9px] pt-1 font-sans">
                      <span>{{ log.latency_ms }}ms</span>
                      <span>{{ log.created_at }}</span>
                    </div>
                    <div v-if="log.status >= 400 && log.error_message?.Valid" class="text-rose-400/90 text-[10px] font-sans pt-1 whitespace-pre-wrap leading-relaxed border-t border-rose-500/10 mt-1">
                      ❌ Error: {{ log.error_message.String }}
                    </div>
                  </div>
                  <div v-if="logs.length === 0" class="p-8 text-center text-zinc-600">
                    Belum ada request lalu lintas data masuk.
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- --- VIEW: PROJECT DETAIL (TABLES LIST) --- -->
          <div v-if="currentView === 'project-detail'" class="space-y-6">
            <!-- Breadcrumbs -->
            <nav class="flex text-xs font-semibold space-x-2 text-zinc-500 select-none">
              <a href="#" @click.prevent="currentView = 'projects'" class="hover:text-zinc-350 transition">Dashboard</a>
              <span>/</span>
              <span class="text-zinc-300 font-bold">{{ activeProject.name }}</span>
            </nav>

            <!-- Header -->
            <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between pb-6 border-b border-white/5 gap-4">
              <div>
                <h1 class="text-3xl font-extrabold text-white tracking-tight">Database Tables</h1>
                <div class="mt-2 flex flex-wrap gap-x-6 gap-y-2 text-xs text-zinc-400">
                  <span class="flex items-center gap-2">
                    <span>Project ID:</span>
                    <code class="text-zinc-300 font-mono bg-zinc-950 border border-white/5 px-2 py-0.5 rounded">{{ activeProject.id }}</code>
                    <button @click="copyToClipboard(activeProject.id, 'act-proj-id')" class="text-zinc-600 hover:text-zinc-400 transition">
                      <Copy class="h-3.5 w-3.5" />
                    </button>
                    <span v-if="copiedItem === 'act-proj-id'" class="text-[9px] text-emerald-400 font-semibold">Copied!</span>
                  </span>
                  <span class="flex items-center gap-2">
                    <span>API Key:</span>
                    <code class="text-emerald-400 font-mono bg-zinc-950 border border-white/5 px-2 py-0.5 rounded">{{ activeProject.api_key }}</code>
                    <button @click="copyToClipboard(activeProject.api_key, 'act-proj-key')" class="text-zinc-650 hover:text-emerald-400 transition">
                      <Copy class="h-3.5 w-3.5" />
                    </button>
                    <span v-if="copiedItem === 'act-proj-key'" class="text-[9px] text-emerald-400 font-semibold">Copied!</span>
                  </span>
                </div>
              </div>
            </div>

            <!-- Create Table Form -->
            <div class="flex items-center justify-between pt-4">
              <h3 class="text-sm font-bold text-zinc-500 uppercase tracking-wider">Tabel Terdaftar</h3>
              <form @submit.prevent="createTable" class="flex gap-2">
                <input type="text" v-model="newTableName" required placeholder="Nama Tabel Baru (e.g. posts)" class="rounded-lg bg-zinc-900 border border-white/10 px-3 py-1.5 text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-emerald-500">
                <button type="submit" class="inline-flex items-center gap-1.5 rounded-lg bg-emerald-500 hover:bg-emerald-400 px-3.5 py-1.5 text-xs font-semibold text-zinc-950 transition shadow cursor-pointer">
                  <Plus class="h-3.5 w-3.5" />
                  Buat Tabel
                </button>
              </form>
            </div>

            <!-- Tables Grid -->
            <div class="rounded-xl bg-zinc-900/40 border border-white/5 overflow-hidden">
              <table class="w-full text-left text-xs border-collapse">
                <thead>
                  <tr class="bg-zinc-900/80 text-zinc-400 uppercase tracking-wider text-[9px] border-b border-white/5 font-bold">
                    <th class="p-4">Table Name</th>
                    <th class="p-4">Physical DB Name</th>
                    <th class="p-4">Created At</th>
                    <th class="p-4 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-white/5">
                  <tr v-for="(t, idx) in tablesList" :key="t.id" class="hover:bg-zinc-800/20 transition">
                    <td class="p-4 font-bold text-zinc-100 text-sm">
                      <span class="flex items-center gap-2 text-emerald-400/90 select-none">
                        <Database class="h-4 w-4 text-emerald-500" />
                        <span class="text-zinc-100">{{ t.name }}</span>
                      </span>
                    </td>
                    <td class="p-4 font-mono text-zinc-500">
                      <div class="flex items-center gap-1.5">
                        <span>p_{{ activeProject.id.replace(/-/g, '_') }}_{{ t.name }}</span>
                        <button @click="copyToClipboard(`p_${activeProject.id.replace(/-/g, '_')}_${t.name}`, `tbl-phys-${idx}`)" class="text-zinc-600 hover:text-zinc-400 transition" title="Copy Physical Name">
                          <Copy class="h-3 w-3" />
                        </button>
                        <span v-if="copiedItem === `tbl-phys-${idx}`" class="text-[8px] text-zinc-500 font-sans">Copied!</span>
                      </div>
                    </td>
                    <td class="p-4 font-mono text-zinc-500">{{ t.created_at }}</td>
                    <td class="p-4 text-right">
                      <button @click="selectTable(t)" class="inline-flex items-center gap-1.5 rounded-md bg-zinc-800 hover:bg-zinc-700 px-3.5 py-1.5 text-xs font-bold text-zinc-300 border border-white/5 transition cursor-pointer">
                        View Data
                        <ChevronRight class="h-3.5 w-3.5" />
                      </button>
                    </td>
                  </tr>
                  <tr v-if="tablesList.length === 0">
                    <td colspan="4" class="p-12 text-center text-zinc-500">
                      Belum ada tabel yang terdefinisi. Buat tabel pertama Anda di atas!
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- --- VIEW: TABLE DETAIL (TABS NAVIGATION) --- -->
          <div v-if="currentView === 'table-detail'" class="space-y-6">
            <!-- Breadcrumbs -->
            <nav class="flex text-xs font-semibold space-x-2 text-zinc-500 select-none">
              <a href="#" @click.prevent="currentView = 'projects'" class="hover:text-zinc-350 transition">Dashboard</a>
              <span>/</span>
              <a href="#" @click.prevent="selectProject(activeProject)" class="hover:text-zinc-350 transition">{{ activeProject.name }}</a>
              <span>/</span>
              <span class="text-zinc-300 font-bold">{{ activeTable.name }}</span>
            </nav>

            <!-- Header -->
            <div class="pb-6 flex flex-col md:flex-row md:items-center md:justify-between gap-4 border-b border-white/5">
              <div class="flex items-center gap-3">
                <span class="p-2 rounded-lg bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                  <Database class="h-6 w-6" />
                </span>
                <div>
                  <h1 class="text-3xl font-extrabold text-white tracking-tight">{{ activeTable.name }}</h1>
                  <p class="text-xs text-zinc-500 mt-1.5">Physical Name: <code class="bg-zinc-950 px-2 py-0.5 rounded font-mono text-zinc-400">p_{{ activeProject.id.replace(/-/g, '_') }}_{{ activeTable.name }}</code></p>
                </div>
              </div>
              
              <div class="flex items-center gap-4 text-xs select-none">
                <div class="inline-flex items-center gap-2 bg-zinc-900 border border-white/5 px-3 py-1.5 rounded-lg">
                  <span class="font-bold text-zinc-500">RLS Status:</span>
                  <span v-if="tablePolicies.length > 0" class="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">ACTIVE</span>
                  <span v-else class="px-2 py-0.5 rounded text-[10px] font-bold bg-amber-500/10 text-amber-400 border border-amber-500/20">INACTIVE</span>
                </div>
              </div>
            </div>

            <!-- Tab Navigation Menu -->
            <div class="border-b border-zinc-800/80 select-none">
              <nav class="flex space-x-6 text-sm font-semibold">
                <button @click="activeTab = 'data'" :class="activeTab === 'data' ? 'border-emerald-500 text-emerald-400' : 'border-transparent text-zinc-400 hover:text-zinc-200'" class="pb-4 px-1 border-b-2 transition inline-flex items-center gap-2 cursor-pointer">
                  <Database class="h-4 w-4" />
                  Data Viewer
                </button>
                <button @click="activeTab = 'schema'" :class="activeTab === 'schema' ? 'border-emerald-500 text-emerald-400' : 'border-transparent text-zinc-400 hover:text-zinc-200'" class="pb-4 px-1 border-b-2 transition inline-flex items-center gap-2 cursor-pointer">
                  <Shield class="h-4 w-4" />
                  Schema Columns
                </button>
                <button @click="activeTab = 'rls'" :class="activeTab === 'rls' ? 'border-emerald-500 text-emerald-400' : 'border-transparent text-zinc-400 hover:text-zinc-200'" class="pb-4 px-1 border-b-2 transition inline-flex items-center gap-2 cursor-pointer">
                  <Shield class="h-4 w-4" />
                  Security Policies (RLS)
                </button>
                <button @click="activeTab = 'danger'" :class="activeTab === 'danger' ? 'border-rose-500 text-rose-400' : 'border-transparent text-zinc-400 hover:text-rose-400'" class="pb-4 px-1 border-b-2 transition inline-flex items-center gap-2 cursor-pointer">
                  <AlertTriangle class="h-4 w-4 text-rose-500" />
                  Danger Zone
                </button>
              </nav>
            </div>

            <!-- Tab Contents -->
            <div class="space-y-6">
              
              <!-- TAB: DATA VIEWER -->
              <div v-if="activeTab === 'data'" class="space-y-4">
                <div class="rounded-xl bg-zinc-900/40 border border-white/5 overflow-hidden flex flex-col">
                  <div class="p-5 border-b border-white/5 flex justify-between items-center bg-zinc-900/40">
                    <h3 class="text-sm font-bold text-white uppercase tracking-wider">Data Viewer Grid</h3>
                    <div class="flex items-center gap-3">
                      <button @click="showImportPanel = !showImportPanel" class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 border border-white/5 text-xs font-semibold text-zinc-300 transition cursor-pointer">
                        <Upload class="h-3.5 w-3.5 text-emerald-400" />
                        Import JSON
                      </button>
                      <span class="text-xs text-zinc-500 select-none">Live updates enabled</span>
                    </div>
                  </div>

                  <!-- Import JSON Panel -->
                  <div v-if="showImportPanel" class="p-5 border-b border-white/5 bg-zinc-900/60 space-y-4">
                    <div class="flex justify-between items-center select-none">
                      <h4 class="text-xs font-bold text-white uppercase tracking-wider">Import Data dari JSON (Array atau Object)</h4>
                      <button @click="showImportPanel = false" class="text-zinc-500 hover:text-zinc-300 cursor-pointer">✕</button>
                    </div>
                    <div class="space-y-3">
                      <textarea v-model="importJSONData" rows="6" placeholder='Paste JSON di sini...
Contoh:
[
  { "actor_name": "miu shiromine", "code": "dsod-009" }
]' class="w-full font-mono text-xs rounded-lg bg-zinc-950 border border-white/10 p-3 text-zinc-300 placeholder-zinc-700 focus:outline-none focus:border-emerald-500"></textarea>
                      <div class="flex justify-end gap-3 select-none">
                        <button type="button" @click="showImportPanel = false" class="px-3.5 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-xs font-semibold text-zinc-350 transition cursor-pointer">Cancel</button>
                        <button type="button" @click="importJSON" class="px-3.5 py-1.5 rounded-lg bg-emerald-500 hover:bg-emerald-400 text-xs font-semibold text-zinc-950 transition shadow cursor-pointer">Mulai Import</button>
                      </div>
                    </div>
                  </div>

                  <!-- Data Grid Table -->
                  <div class="overflow-x-auto">
                    <table class="w-full text-left text-xs border-collapse">
                      <thead>
                        <tr class="bg-zinc-900/80 text-zinc-400 uppercase tracking-wider text-[9px] border-b border-white/5 font-bold whitespace-nowrap">
                          <th class="p-4 font-mono">ID</th>
                          <th v-for="col in tableColumns" :key="col.id" class="p-4">{{ col.name }}</th>
                          <th class="p-4">Created At</th>
                          <th class="p-4 text-right">Actions</th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-white/5">
                        <tr v-for="(row, rIdx) in tableRows" :key="row.id" class="hover:bg-zinc-800/20 transition">
                          <td class="p-4 font-mono text-zinc-500">
                            <div class="flex items-center gap-1.5">
                              <span class="max-w-[70px] truncate" :title="row.id">{{ row.id }}</span>
                              <button @click="copyToClipboard(row.id, `row-id-${rIdx}`)" class="text-zinc-650 hover:text-zinc-400 transition" title="Copy Record ID">
                                <Copy class="h-3 w-3" />
                              </button>
                              <span v-if="copiedItem === `row-id-${rIdx}`" class="text-[8px] text-zinc-500 font-sans">Copied!</span>
                            </div>
                          </td>
                          <td v-for="col in tableColumns" :key="col.id" class="p-4 text-zinc-300 whitespace-nowrap">
                            <div v-if="row[col.name] !== undefined && row[col.name] !== null" class="max-w-[280px] truncate font-sans text-xs" :title="String(row[col.name])">
                              {{ row[col.name] }}
                            </div>
                            <span v-else class="text-zinc-700 italic select-none">null</span>
                          </td>
                          <td class="p-4 text-zinc-500 font-mono whitespace-nowrap">{{ row.created_at }}</td>
                          <td class="p-4 text-right">
                            <button @click="deleteRow(row.id)" class="text-rose-500 hover:text-rose-400 hover:bg-rose-500/10 p-1.5 rounded-md transition inline-flex items-center justify-center cursor-pointer" title="Hapus Baris">
                              <Trash2 class="h-3.5 w-3.5" />
                            </button>
                          </td>
                        </tr>
                        <tr v-if="tableRows.length === 0">
                          <td :colspan="tableColumns.length + 3" class="p-12 text-center text-zinc-600 font-light leading-relaxed">
                            Tabel ini masih kosong. Silakan isi data melalui REST API atau import dari JSON.
                          </td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>

              <!-- TAB: SCHEMA COLUMNS -->
              <div v-if="activeTab === 'schema'" class="grid grid-cols-1 lg:grid-cols-3 gap-8">
                <!-- Column Schema list -->
                <div class="lg:col-span-2 rounded-xl bg-zinc-900/40 border border-white/5 overflow-hidden">
                  <div class="p-5 border-b border-white/5 bg-zinc-900/40">
                    <h3 class="text-sm font-bold text-white uppercase tracking-wider">Daftar Kolom Skema</h3>
                  </div>
                  <div class="overflow-x-auto">
                    <table class="w-full text-left text-xs border-collapse">
                      <thead>
                        <tr class="bg-zinc-900/80 text-zinc-400 uppercase tracking-wider text-[9px] border-b border-white/5 font-bold">
                          <th class="p-4">Column Name</th>
                          <th class="p-4">Type</th>
                          <th class="p-4">Is Nullable</th>
                          <th class="p-4">Constraint</th>
                          <th class="p-4 text-right">Actions</th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-white/5">
                        <!-- Default system columns -->
                        <tr class="hover:bg-zinc-800/10">
                          <td class="p-4 font-mono font-bold text-zinc-400">id</td>
                          <td class="p-4 font-mono text-zinc-550">UUID</td>
                          <td class="p-4 text-zinc-600">NO</td>
                          <td class="p-4"><span class="px-2 py-0.5 text-[9px] rounded font-bold bg-emerald-500/10 text-emerald-450 border border-emerald-500/20">PRIMARY KEY</span></td>
                          <td class="p-4 text-right text-zinc-700">—</td>
                        </tr>
                        <tr class="hover:bg-zinc-800/10">
                          <td class="p-4 font-mono font-bold text-zinc-400">project_id</td>
                          <td class="p-4 font-mono text-zinc-550">UUID</td>
                          <td class="p-4 text-zinc-600">NO</td>
                          <td class="p-4"><span class="px-2 py-0.5 text-[9px] rounded font-bold bg-zinc-800/60 text-zinc-500 border border-white/5">FOREIGN KEY</span></td>
                          <td class="p-4 text-right text-zinc-700">—</td>
                        </tr>
                        <!-- Custom user columns -->
                        <tr v-for="col in tableColumns" :key="col.id" class="hover:bg-zinc-800/10">
                          <td class="p-4 font-mono font-bold text-emerald-400">{{ col.name }}</td>
                          <td class="p-4 font-mono text-zinc-400 uppercase">{{ col.type }}</td>
                          <td class="p-4 font-mono text-zinc-500">{{ col.is_nullable ? 'YES' : 'NO' }}</td>
                          <td class="p-4 text-zinc-700">—</td>
                          <td class="p-4 text-right">
                            <button @click="deleteColumn(col)" class="text-rose-500 hover:text-rose-400 hover:bg-rose-500/10 p-1.5 rounded-md transition inline-flex items-center justify-center cursor-pointer" title="Delete Column">
                              <Trash2 class="h-3.5 w-3.5" />
                            </button>
                          </td>
                        </tr>
                        <!-- Default updated_at / created_at columns -->
                        <tr class="hover:bg-zinc-800/10">
                          <td class="p-4 font-mono font-bold text-zinc-400">created_at</td>
                          <td class="p-4 font-mono text-zinc-550">TIMESTAMP</td>
                          <td class="p-4 text-zinc-600">YES</td>
                          <td class="p-4 text-zinc-700">—</td>
                          <td class="p-4 text-right text-zinc-700">—</td>
                        </tr>
                        <tr class="hover:bg-zinc-800/10">
                          <td class="p-4 font-mono font-bold text-zinc-400">updated_at</td>
                          <td class="p-4 font-mono text-zinc-550">TIMESTAMP</td>
                          <td class="p-4 text-zinc-600">YES</td>
                          <td class="p-4 text-zinc-700">—</td>
                          <td class="p-4 text-right text-zinc-700">—</td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                </div>

                <!-- Add Column Form -->
                <div class="lg:col-span-1 rounded-xl bg-zinc-900/40 border border-white/5 p-6 self-start space-y-4">
                  <h4 class="text-xs font-bold text-white uppercase tracking-wider">Tambah Kolom Baru</h4>
                  <form @submit.prevent="addColumn" class="space-y-4">
                    <div>
                      <label class="block text-[10px] font-bold text-zinc-500 uppercase mb-1.5">Column Name</label>
                      <input type="text" v-model="newColName" required placeholder="Contoh: price" class="w-full rounded-lg bg-zinc-900 border border-white/10 px-3.5 py-2 text-xs text-white focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500/20 transition">
                    </div>
                    <div>
                      <label class="block text-[10px] font-bold text-zinc-500 uppercase mb-1.5">Data Type</label>
                      <select v-model="newColType" class="w-full rounded-lg bg-zinc-900 border border-white/10 px-3.5 py-2 text-xs text-white focus:outline-none focus:border-emerald-500 transition">
                        <option value="text">TEXT (String)</option>
                        <option value="integer">INTEGER (Angka)</option>
                        <option value="boolean">BOOLEAN (true/false)</option>
                        <option value="timestamp">TIMESTAMP (Tanggal/Waktu)</option>
                        <option value="jsonb">JSONB (Kompleks JSON)</option>
                      </select>
                    </div>
                    <div class="flex items-center gap-2">
                      <input type="checkbox" v-model="newColNullable" id="nullable_check" class="rounded border-zinc-800 bg-zinc-950 text-emerald-500 focus:ring-0">
                      <label for="nullable_check" class="text-xs text-zinc-450 select-none">Is Nullable (Bisa bernilai kosong)</label>
                    </div>
                    <button type="submit" class="w-full inline-flex justify-center items-center gap-1.5 rounded-lg bg-emerald-500 hover:bg-emerald-400 px-3 py-2.5 text-xs font-bold text-zinc-950 transition shadow cursor-pointer">
                      <Plus class="h-3.5 w-3.5" />
                      Simpan Kolom
                    </button>
                  </form>
                </div>
              </div>

              <!-- TAB: SECURITY POLICIES (RLS) -->
              <div v-if="activeTab === 'rls'" class="space-y-6">
                <!-- RLS Guide -->
                <div class="bg-emerald-500/5 border border-emerald-500/10 rounded-xl p-5 text-xs text-zinc-400 space-y-3 leading-relaxed flex gap-3">
                  <span class="text-emerald-400 shrink-0">
                    <Shield class="h-5 w-5" />
                  </span>
                  <div>
                    <div class="text-emerald-400 font-bold mb-1">Panduan Setup RLS untuk Pemula:</div>
                    <p class="font-light">
                      Secara default, jika tabel **tidak memiliki policy** apa pun, maka **RLS tidak aktif (Public Access)**. Semua orang yang memegang API Key proyek Anda bisa membaca dan menulis data ke tabel ini. Begitu Anda menambahkan minimal 1 policy, RLS akan otomatis menyala dan mengunci akses selain yang diatur oleh policy.
                    </p>
                  </div>
                </div>

                <div class="grid grid-cols-1 lg:grid-cols-3 gap-8">
                  <!-- Policies List -->
                  <div class="lg:col-span-2 rounded-xl bg-zinc-900/40 border border-white/5 overflow-hidden">
                    <div class="p-5 border-b border-white/5 bg-zinc-900/40">
                      <h3 class="text-sm font-bold text-white uppercase tracking-wider">Active Security Policies</h3>
                    </div>
                    <div class="overflow-x-auto">
                      <table class="w-full text-left text-xs border-collapse">
                        <thead>
                          <tr class="bg-zinc-900/60 text-zinc-400 uppercase tracking-wider text-[9px] border-b border-white/5 font-bold">
                            <th class="p-4">Action</th>
                            <th class="p-4">Role</th>
                            <th class="p-4">Expression / SQL Rule</th>
                            <th class="p-4 text-right">Delete</th>
                          </tr>
                        </thead>
                        <tbody class="divide-y divide-white/5">
                          <tr v-for="pol in tablePolicies" :key="pol.id" class="hover:bg-zinc-800/25 transition">
                            <td class="p-4"><span class="px-2.5 py-0.5 rounded text-[10px] font-bold bg-zinc-800 text-zinc-350 border border-white/5">{{ pol.action }}</span></td>
                            <td class="p-4 font-semibold text-emerald-400">{{ pol.role }}</td>
                            <td class="p-4 font-mono text-zinc-400 text-[11px]">{{ pol.expression }}</td>
                            <td class="p-4 text-right">
                              <button @click="deletePolicy(pol.id)" class="text-rose-500 hover:text-rose-455 hover:bg-rose-500/10 p-1.5 rounded-md transition duration-150 inline-flex items-center justify-center cursor-pointer" title="Delete Policy">
                                <Trash2 class="h-3.5 w-3.5" />
                              </button>
                            </td>
                          </tr>
                          <tr v-if="tablePolicies.length === 0">
                            <td colspan="4" class="p-12 text-center text-zinc-600 font-light">
                              RLS belum aktif pada tabel ini (Akses terbuka bebas).
                            </td>
                          </tr>
                        </tbody>
                      </table>
                    </div>
                  </div>

                  <!-- Add Policy Form -->
                  <div class="lg:col-span-1 rounded-xl bg-zinc-900/40 border border-white/5 p-6 self-start space-y-4">
                    <h4 class="text-xs font-bold text-white uppercase tracking-wider">Buat Policy Baru</h4>
                    <form @submit.prevent="createPolicy" class="space-y-4">
                      <div>
                        <label class="block text-[10px] font-bold text-zinc-500 uppercase mb-1.5">Action</label>
                        <select v-model="newPolicyAction" class="w-full rounded-lg bg-zinc-900 border border-white/10 px-3.5 py-2 text-xs text-white focus:outline-none focus:border-emerald-500 transition">
                          <option value="SELECT">SELECT (Membaca)</option>
                          <option value="INSERT">INSERT (Menulis)</option>
                          <option value="UPDATE">UPDATE (Mengubah)</option>
                          <option value="DELETE">DELETE (Menghapus)</option>
                        </select>
                      </div>
                      <div>
                        <label class="block text-[10px] font-bold text-zinc-500 uppercase mb-1.5">Target Role</label>
                        <select v-model="newPolicyRole" class="w-full rounded-lg bg-zinc-900 border border-white/10 px-3.5 py-2 text-xs text-white focus:outline-none focus:border-emerald-500 transition">
                          <option value="authenticated">AUTHENTICATED (Wajib Login)</option>
                          <option value="anon">ANON (Publik)</option>
                        </select>
                      </div>
                      <div>
                        <label class="block text-[10px] font-bold text-zinc-500 uppercase mb-1.5">Filter Rule</label>
                        <select v-model="newPolicyExpression" class="w-full rounded-lg bg-zinc-900 border border-white/10 px-3.5 py-2 text-xs text-white focus:outline-none focus:border-emerald-500 transition">
                          <option value="auth.uid() = user_id">auth.uid() = user_id (Hanya Pemilik Data)</option>
                          <option value="true">true (Diizinkan Bebas)</option>
                        </select>
                      </div>
                      <button type="submit" class="w-full inline-flex justify-center items-center gap-1.5 rounded-lg bg-emerald-500 hover:bg-emerald-400 px-3 py-2.5 text-xs font-bold text-zinc-950 transition shadow cursor-pointer">
                        <Plus class="h-3.5 w-3.5" />
                        Simpan Policy
                      </button>
                    </form>
                  </div>
                </div>
              </div>

              <!-- TAB: DANGER ZONE -->
              <div v-if="activeTab === 'danger'" class="rounded-xl border border-rose-500/20 bg-rose-500/5 p-6 space-y-4">
                <div>
                  <h3 class="text-sm font-bold text-rose-400 uppercase tracking-wider flex items-center gap-2">
                    <AlertTriangle class="h-4 w-4" />
                    Hapus Tabel Ini
                  </h3>
                  <p class="text-xs text-zinc-400 mt-1">Menghapus tabel ini akan menghapus seluruh data fisik di database, semua skema kolom kustom, serta kebijakan keamanan (RLS policy) secara permanen. Tindakan ini tidak dapat dibatalkan!</p>
                </div>
                <div>
                  <button @click="deleteTable" class="inline-flex items-center gap-1.5 rounded-lg bg-rose-600 hover:bg-rose-500 text-white px-4 py-2.5 text-xs font-bold transition shadow cursor-pointer">
                    Hapus Tabel Permanen
                    <Trash2 class="h-4 w-4" />
                  </button>
                </div>
              </div>

            </div>
          </div>

          <!-- --- VIEW: AUTH CONSOLE (USERS) --- -->
          <div v-if="currentView === 'auth'" class="space-y-6">
            <!-- Breadcrumbs -->
            <nav class="flex text-xs font-semibold space-x-2 text-zinc-500 select-none">
              <a href="#" @click.prevent="currentView = 'projects'" class="hover:text-zinc-350 transition">Dashboard</a>
              <span>/</span>
              <a href="#" @click.prevent="selectProject(activeProject)" class="hover:text-zinc-350 transition">{{ activeProject.name }}</a>
              <span>/</span>
              <span class="text-zinc-300 font-bold">Authentication</span>
            </nav>

            <div class="mb-8 border-b border-white/5 pb-6">
              <h1 class="text-3xl font-extrabold text-white tracking-tight">Authentication Console</h1>
              <p class="mt-1 text-sm text-zinc-500">Kelola pengguna terdaftar dan otorisasi JWT untuk project {{ activeProject.name }}.</p>
            </div>

            <div class="grid grid-cols-1 lg:grid-cols-3 gap-8">
              <!-- Create User Form -->
              <div class="lg:col-span-1 rounded-xl bg-zinc-900/40 border border-white/5 p-6 self-start space-y-4">
                <h3 class="text-sm font-bold text-white uppercase tracking-wider">Daftarkan User Baru</h3>
                <form @submit.prevent="createUser" class="space-y-3">
                  <div>
                    <input type="email" v-model="newUserName" required placeholder="Alamat Email" class="w-full rounded-lg bg-zinc-900 border border-white/10 px-3.5 py-2 text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-emerald-500 transition">
                  </div>
                  <div>
                    <input type="password" v-model="newUserPassword" required placeholder="Password" class="w-full rounded-lg bg-zinc-900 border border-white/10 px-3.5 py-2 text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-emerald-500 transition">
                  </div>
                  <button type="submit" class="w-full inline-flex justify-center items-center gap-1.5 rounded-lg bg-emerald-500 hover:bg-emerald-400 px-3 py-2.5 text-xs font-bold text-zinc-950 transition shadow cursor-pointer">
                    <Users class="h-3.5 w-3.5" />
                    Tambahkan User
                  </button>
                </form>
              </div>

              <!-- Users List Table -->
              <div class="lg:col-span-2 rounded-xl bg-zinc-900/40 border border-white/5 overflow-hidden flex flex-col">
                <div class="p-5 border-b border-white/5 bg-zinc-900/40 flex justify-between items-center select-none">
                  <h3 class="text-xs font-bold text-white uppercase tracking-wider">User Registrations</h3>
                  <span class="text-xs text-zinc-500 font-mono">Total: {{ projectUsers.length }}</span>
                </div>
                <div class="overflow-x-auto">
                  <table class="w-full text-left text-xs border-collapse">
                    <thead>
                      <tr class="bg-zinc-900/80 text-zinc-400 uppercase tracking-wider text-[9px] border-b border-white/5 font-bold">
                        <th class="p-4">User ID</th>
                        <th class="p-4">Email Address</th>
                        <th class="p-4">Registered At</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-white/5">
                      <tr v-for="(u, uIdx) in projectUsers" :key="u.id" class="hover:bg-zinc-800/20 transition">
                        <td class="p-4 font-mono text-zinc-550">
                          <div class="flex items-center gap-1.5">
                            <span class="max-w-[100px] truncate" :title="u.id">{{ u.id }}</span>
                            <button @click="copyToClipboard(u.id, `u-id-${uIdx}`)" class="text-zinc-650 hover:text-zinc-400 transition" title="Copy User ID">
                              <Copy class="h-3 w-3" />
                            </button>
                            <span v-if="copiedItem === `u-id-${uIdx}`" class="text-[8px] text-zinc-550 font-sans">Copied!</span>
                          </div>
                        </td>
                        <td class="p-4 font-bold text-zinc-200 text-sm">{{ u.email }}</td>
                        <td class="p-4 text-zinc-500 font-mono">{{ u.created_at }}</td>
                      </tr>
                      <tr v-if="projectUsers.length === 0">
                        <td colspan="3" class="p-12 text-center text-zinc-500">
                          Belum ada user yang terdaftar dalam proyek ini.
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          </div>

          <!-- --- VIEW: SWAGGER API DOCS --- -->
          <div v-if="currentView === 'docs'" class="space-y-6">
            <!-- Breadcrumbs -->
            <nav class="flex text-xs font-semibold space-x-2 text-zinc-500 select-none">
              <a href="#" @click.prevent="currentView = 'projects'" class="hover:text-zinc-350 transition">Dashboard</a>
              <span>/</span>
              <a href="#" @click.prevent="selectProject(activeProject)" class="hover:text-zinc-350 transition">{{ activeProject.name }}</a>
              <span>/</span>
              <span class="text-zinc-300 font-bold">API Docs (Swagger)</span>
            </nav>

            <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
              <div>
                <h1 class="text-3xl font-extrabold text-white tracking-tight">API Swagger Docs</h1>
                <p class="text-xs text-zinc-500 mt-2">OpenAPI Swagger spec yang digenerate otomatis menyesuaikan skema tabel project Anda.</p>
              </div>
              <a :href="`/api/projects/${activeProject.id}/swagger.json`" target="_blank" class="inline-flex items-center gap-2 rounded-lg bg-zinc-800 hover:bg-zinc-700 px-3.5 py-2 text-xs font-semibold text-zinc-300 border border-white/5 transition">
                Open Raw swagger.json
              </a>
            </div>

            <div class="rounded-xl border border-white/5 overflow-hidden bg-[#1e1e1e] p-4 min-h-[650px] relative">
              <iframe :src="`/dashboard/projects/${activeProject.id}/swagger-iframe`" class="w-full min-h-[650px] border-none" scrolling="yes"></iframe>
            </div>
          </div>

        </div>
      </main>
    </div>
  </div>
</template>
