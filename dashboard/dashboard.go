package dashboard

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"gobaas/db"
	"gobaas/policy"
	"gobaas/schema"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const loginHTML = `
<!DOCTYPE html>
<html lang="en" class="h-full bg-[#121212] text-zinc-150">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Login - Coderbase Studio</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <link href="https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@300;400;500;600;700&display=swap" rel="stylesheet">
    <style>
        body { font-family: 'Plus Jakarta Sans', sans-serif; }
        .glass-panel {
            background: rgba(30, 30, 30, 0.6);
            backdrop-filter: blur(8px);
            border: 1px solid rgba(255, 255, 255, 0.05);
        }
    </style>
</head>
<body class="h-full flex items-center justify-center p-6">
    <div class="w-full max-w-md glass-panel p-8 rounded-2xl space-y-6 shadow-2xl relative overflow-hidden">
        <!-- Glow effect -->
        <div class="absolute -top-10 -right-10 w-32 h-32 bg-emerald-500/10 rounded-full blur-2xl"></div>
        <div class="absolute -bottom-10 -left-10 w-32 h-32 bg-emerald-500/10 rounded-full blur-2xl"></div>

        <div class="flex flex-col items-center text-center">
            <svg class="h-10 w-10 text-emerald-500 mb-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <ellipse cx="12" cy="5" rx="9" ry="3"></ellipse>
                <path d="M3 5V19A9 3 0 0 0 21 19V5"></path>
                <path d="M3 12A9 3 0 0 0 21 12"></path>
            </svg>
            <h2 class="text-2xl font-bold text-white tracking-tight">Coderbase Studio</h2>
            <p class="text-xs text-zinc-550 mt-1.5">Masukkan kredensial admin untuk masuk ke console.</p>
        </div>

        {{if .Error}}
        <div class="p-3 bg-rose-500/10 border border-rose-500/20 text-rose-455 rounded-lg text-xs text-center font-medium">
            {{.Error}}
        </div>
        {{end}}

        <form action="/dashboard/login" method="POST" class="space-y-4">
            <div>
                <label class="block text-[10px] font-bold text-zinc-500 uppercase mb-1.5 tracking-wider">Username</label>
                <input type="text" name="username" required placeholder="admin" class="w-full rounded-lg bg-zinc-900 border border-zinc-800 px-3.5 py-2.5 text-sm text-white focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500/20 transition">
            </div>
            <div>
                <label class="block text-[10px] font-bold text-zinc-500 uppercase mb-1.5 tracking-wider">Password</label>
                <input type="password" name="password" required placeholder="••••••••" class="w-full rounded-lg bg-zinc-900 border border-zinc-800 px-3.5 py-2.5 text-sm text-white focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500/20 transition">
            </div>
            <button type="submit" class="w-full inline-flex justify-center items-center rounded-lg bg-emerald-500 hover:bg-emerald-400 py-3 text-sm font-semibold text-zinc-950 transition duration-150 shadow">
                Masuk Console
            </button>
        </form>
    </div>
</body>
</html>
`

// Layout dasar Coderbase dengan dynamic sidebar menu (Projects list vs Inside Project menu)
const layoutHTML = `
<!DOCTYPE html>
<html lang="en" class="h-full bg-[#121212] text-zinc-150">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Coderbase Studio</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <link href="https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
    <script>
        tailwind.config = {
            theme: {
                extend: {
                    fontFamily: {
                        sans: ['"Plus Jakarta Sans"', 'sans-serif'],
                        mono: ['"JetBrains Mono"', 'monospace'],
                    }
                }
            }
        }
        
        function copyToClipboard(text, elementId) {
            navigator.clipboard.writeText(text).then(function() {
                const tooltip = document.getElementById('tooltip-' + elementId);
                if (tooltip) {
                    const originalText = tooltip.innerText;
                    tooltip.innerText = 'Copied!';
                    tooltip.classList.remove('text-zinc-500');
                    tooltip.classList.add('text-emerald-400');
                    setTimeout(function() {
                        tooltip.innerText = originalText;
                        tooltip.classList.remove('text-emerald-400');
                        tooltip.classList.add('text-zinc-500');
                    }, 1500);
                }
            });
        }
    </script>
    <style>
        body { font-family: 'Plus Jakarta Sans', sans-serif; }
        .glass-panel {
            background: rgba(30, 30, 30, 0.6);
            backdrop-filter: blur(8px);
            border: 1px solid rgba(255, 255, 255, 0.05);
        }
        .border-premium {
            border-color: rgba(255, 255, 255, 0.05);
        }
        .bg-sidebar {
            background-color: #161616;
        }
        .bg-nav {
            background-color: #161616;
        }
    </style>
</head>
<body class="h-full flex flex-col overflow-hidden text-zinc-300">
    <!-- Navbar -->
    <nav class="bg-nav h-14 flex items-center justify-between px-6 z-40 shrink-0 border-b border-premium">
        <div class="flex items-center gap-3">
            <a href="/dashboard" class="flex items-center gap-2.5 font-semibold text-lg text-white">
                <svg class="h-5 w-5 text-emerald-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                    <ellipse cx="12" cy="5" rx="9" ry="3"></ellipse>
                    <path d="M3 5V19A9 3 0 0 0 21 19V5"></path>
                    <path d="M3 12A9 3 0 0 0 21 12"></path>
                </svg>
                <span class="tracking-tight text-xl font-bold text-white">Coderbase</span>
                <span class="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">STUDIO</span>
            </a>
        </div>
        <div class="flex items-center gap-4 text-xs text-zinc-400">
            <span class="flex items-center gap-2 bg-zinc-900 border border-premium px-2.5 py-1 rounded-full">
                <span class="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse"></span>
                <span>Active: {{.DBType}} DB</span>
            </span>
            <form action="/dashboard/logout" method="POST" class="inline">
                <button type="submit" class="text-zinc-500 hover:text-rose-455 transition font-semibold px-2.5 py-1 rounded hover:bg-rose-500/10">Logout</button>
            </form>
        </div>
    </nav>

    <!-- App Container -->
    <div class="flex-1 flex overflow-hidden">
        <!-- Sidebar -->
        <aside class="bg-sidebar w-60 hidden md:flex flex-col shrink-0 border-r border-premium">
            <div class="flex-1 py-6 px-4 space-y-7 overflow-y-auto">
                {{if .ActiveProjectID}}
                <!-- Menu khusus di dalam Project (Supabase Style) -->
                <div class="space-y-1.5">
                    <div class="px-3 mb-2">
                        <span class="text-[10px] font-bold text-zinc-500 uppercase tracking-wider block">Project Console</span>
                        <span class="text-xs font-bold text-zinc-300 truncate block mt-0.5">{{.ActiveProjectName}}</span>
                    </div>
                    <a href="/dashboard/projects/{{.ActiveProjectID}}" class="flex items-center gap-3 px-3 py-2 text-sm font-semibold rounded-lg text-zinc-300 hover:text-white hover:bg-zinc-800/40 transition">
                        <svg class="h-4 w-4 text-emerald-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <path d="M12 3v18"></path>
                            <rect width="18" height="18" x="3" y="3" rx="2"></rect>
                            <path d="M3 9h18"></path>
                            <path d="M3 15h18"></path>
                        </svg>
                        Database Tables
                    </a>
                    <a href="/dashboard/projects/{{.ActiveProjectID}}/docs" class="flex items-center gap-3 px-3 py-2 text-sm font-semibold rounded-lg text-zinc-300 hover:text-white hover:bg-zinc-800/40 transition">
                        <svg class="h-4 w-4 text-zinc-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1-2.5-2.5Z"></path>
                            <path d="M6 6h10"></path>
                            <path d="M6 10h10"></path>
                        </svg>
                        API Docs (Swagger)
                    </a>
                    <a href="/dashboard/projects/{{.ActiveProjectID}}/auth" class="flex items-center gap-3 px-3 py-2 text-sm font-semibold rounded-lg text-zinc-300 hover:text-white hover:bg-zinc-800/40 transition">
                        <svg class="h-4 w-4 text-zinc-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"></path>
                            <circle cx="9" cy="7" r="4"></circle>
                            <path d="M22 21v-2a4 4 0 0 0-3-3.87"></path>
                            <path d="M16 3.13a4 4 0 0 1 0 7.75"></path>
                        </svg>
                        Authentication (Users)
                    </a>
                </div>
                <div class="border-t border-zinc-800 my-4 pt-4">
                    <a href="/dashboard/projects-list" class="flex items-center gap-2 px-3 py-2 text-xs font-semibold rounded-lg text-zinc-500 hover:text-zinc-350 transition">
                        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg>
                        Back to Projects List
                    </a>
                </div>
                {{else}}
                <!-- Menu Navigasi Utama -->
                <div class="space-y-1.5">
                    <span class="text-[10px] font-bold text-zinc-500 uppercase tracking-wider px-3">Navigation</span>
                    <a href="/dashboard" class="flex items-center gap-3 px-3 py-2 text-sm font-semibold rounded-lg text-zinc-300 hover:text-white hover:bg-zinc-800/40 transition">
                        <svg class="h-4 w-4 text-emerald-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <rect width="7" height="9" x="3" y="3" rx="1"></rect>
                            <rect width="7" height="5" x="14" y="3" rx="1"></rect>
                            <rect width="7" height="9" x="14" y="12" rx="1"></rect>
                            <rect width="7" height="5" x="3" y="16" rx="1"></rect>
                        </svg>
                        Dashboard
                    </a>
                    <a href="/dashboard/projects-list" class="flex items-center gap-3 px-3 py-2 text-sm font-semibold rounded-lg text-zinc-300 hover:text-white hover:bg-zinc-800/40 transition">
                        <svg class="h-4 w-4 text-zinc-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"></path>
                        </svg>
                        Projects
                    </a>
                </div>
                {{end}}
            </div>
            <div class="p-4 border-t border-premium text-[11px] text-zinc-600 font-medium">
                Coderbase Studio Engine v1.0
            </div>
        </aside>

        <!-- Main Content View -->
        <main class="flex-1 overflow-y-auto bg-[#121212] p-8">
            <div class="max-w-6xl mx-auto">
                {{template "content" .}}
            </div>
        </main>
    </div>
</body>
</html>
`

// Content Dashboard Overview (Halaman Beranda Utama dengan Logs Analitik)
const dashboardHTML = `
{{define "content"}}
<div class="mb-8">
    <h1 class="text-3xl font-extrabold text-white tracking-tight">Dashboard Overview</h1>
    <p class="mt-1 text-sm text-zinc-500">Monitor status performa global server Coderbase BaaS Anda.</p>
</div>

<!-- Stats Grid -->
<div class="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4 mb-8">
    <div class="glass-panel p-6 rounded-xl flex flex-col justify-between hover:border-zinc-700 transition">
        <span class="text-[10px] font-bold text-zinc-500 uppercase tracking-wider">Total Projects</span>
        <span class="text-4xl font-black text-white mt-2">{{.Stats.Projects}}</span>
    </div>
    <div class="glass-panel p-6 rounded-xl flex flex-col justify-between hover:border-zinc-700 transition">
        <span class="text-[10px] font-bold text-zinc-500 uppercase tracking-wider">Total Tables</span>
        <span class="text-4xl font-black text-emerald-400 mt-2">{{.Stats.Tables}}</span>
    </div>
    <div class="glass-panel p-6 rounded-xl flex flex-col justify-between hover:border-zinc-700 transition">
        <span class="text-[10px] font-bold text-zinc-500 uppercase tracking-wider">Total Users</span>
        <span class="text-4xl font-black text-white mt-2">{{.Stats.Users}}</span>
    </div>
    <div class="glass-panel p-6 rounded-xl flex flex-col justify-between hover:border-zinc-700 transition">
        <span class="text-[10px] font-bold text-zinc-500 uppercase tracking-wider">Database Type</span>
        <span class="text-xl font-extrabold text-zinc-300 mt-3.5 uppercase tracking-tight">{{.DBType}}</span>
    </div>
</div>

<!-- Logs Analitik & Active Projects -->
<div class="grid grid-cols-1 lg:grid-cols-3 gap-8">
    <!-- Tabel Log System (Tabel) -->
    <div class="lg:col-span-2 rounded-xl glass-panel overflow-hidden flex flex-col self-start">
        <div class="p-5 border-b border-premium bg-zinc-900/40 flex justify-between items-center">
            <h3 class="text-sm font-bold text-white uppercase tracking-wider">Recent System Logs</h3>
            <span class="flex items-center gap-1.5 text-xs text-emerald-400 font-semibold">
                <span class="h-2 w-2 rounded-full bg-emerald-500 animate-ping"></span>
                <span>Live Traffic Monitor</span>
            </span>
        </div>
        <div class="overflow-x-auto">
            <table class="w-full text-left text-xs border-collapse">
                <thead>
                    <tr class="bg-[#141414] text-zinc-500 uppercase tracking-wider text-[8px] border-b border-premium font-bold">
                        <th class="p-3">Method</th>
                        <th class="p-3">Path</th>
                        <th class="p-3">Status</th>
                        <th class="p-3">Latency</th>
                        <th class="p-3">Time</th>
                    </tr>
                </thead>
                <tbody class="divide-y divide-zinc-800/40">
                    {{range .Logs}}
                    <tr class="hover:bg-zinc-850/20 transition">
                        <td class="p-3">
                            <span class="px-2 py-0.5 rounded text-[9px] font-bold 
                            {{if eq .Method "GET"}}bg-blue-500/10 text-blue-400 border border-blue-500/20
                            {{else if eq .Method "POST"}}bg-emerald-500/10 text-emerald-400 border border-emerald-500/20
                            {{else if eq .Method "PATCH"}}bg-amber-500/10 text-amber-400 border border-amber-500/20
                            {{else}}bg-rose-500/10 text-rose-400 border border-rose-500/20{{end}}">
                                {{.Method}}
                            </span>
                        </td>
                        <td class="p-3 text-zinc-300 font-mono text-[11px] truncate max-w-[180px]" title="{{.Path}}">{{.Path}}</td>
                        <td class="p-3">
                            <span class="font-bold {{if lt .Status 300}}text-emerald-400{{else if lt .Status 400}}text-blue-400{{else}}text-rose-400{{end}}">
                                {{.Status}}
                            </span>
                        </td>
                        <td class="p-3 font-mono text-zinc-400">{{.Latency}}ms</td>
                        <td class="p-3 text-[10px] text-zinc-500">{{.CreatedAt}}</td>
                    </tr>
                    {{else}}
                    <tr>
                        <td colspan="5" class="p-8 text-center text-zinc-600 font-light">Belum ada request lalu lintas data masuk.</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>

    <!-- Active Projects List Ringkas -->
    <div class="lg:col-span-1 glass-panel rounded-xl p-6 self-start">
        <div class="flex items-center justify-between mb-4 border-b border-premium pb-3">
            <h3 class="text-xs font-bold text-white uppercase tracking-wider">Active Projects</h3>
            <a href="/dashboard/projects-list" class="text-xs text-emerald-400 hover:text-emerald-300 font-semibold">Lihat Semua</a>
        </div>
        <ul class="divide-y divide-zinc-800/60 space-y-3">
            {{range .Projects}}
            <li class="pt-3 first:pt-0">
                <a href="/dashboard/projects/{{.ID}}" class="group block">
                    <span class="text-sm font-bold text-zinc-200 group-hover:text-emerald-400 transition">{{.Name}}</span>
                    <span class="block text-[10px] text-zinc-500 font-mono mt-1 truncate">{{.ID}}</span>
                </a>
            </li>
            {{else}}
            <li class="text-zinc-650 text-xs py-4 text-center">Belum ada project aktif.</li>
            {{end}}
        </ul>
    </div>
</div>
{{end}}
`

// Content List Projects (Tampilan Tabel)
const projectsHTML = `
{{define "content"}}
<div class="flex flex-col md:flex-row md:items-center md:justify-between mb-8 gap-4">
    <div>
        <h1 class="text-3xl font-extrabold tracking-tight text-white">Projects Console</h1>
        <p class="mt-1 text-sm text-zinc-500">Kelola credentials proyek dan struktur database di bawah ini.</p>
    </div>
    <div class="shrink-0">
        <form action="/dashboard/projects" method="POST" class="flex gap-2.5">
            <input type="text" name="name" required placeholder="Nama Project Baru" class="rounded-lg bg-zinc-900 border border-premium px-4 py-2 text-sm text-white focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500/20 transition">
            <button type="submit" class="inline-flex items-center gap-1.5 rounded-lg bg-emerald-500 hover:bg-emerald-400 px-4 py-2 text-sm font-semibold text-zinc-950 shadow-md transition duration-200">
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14"></path><path d="M12 5v14"></path></svg>
                Create Project
            </button>
        </form>
    </div>
</div>

<div class="rounded-xl glass-panel overflow-hidden">
    <div class="overflow-x-auto">
        <table class="w-full text-left text-xs border-collapse">
            <thead>
                <tr class="bg-zinc-900/60 text-zinc-400 uppercase tracking-wider text-[9px] border-b border-premium font-bold">
                    <th class="p-4">Project Name</th>
                    <th class="p-4">Project ID</th>
                    <th class="p-4">API Key</th>
                    <th class="p-4">Status</th>
                    <th class="p-4 text-right">Actions</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-zinc-800/40">
                {{range $i, $e := .Projects}}
                <tr class="hover:bg-zinc-800/20 transition">
                    <td class="p-4 font-bold text-white text-sm">{{.Name}}</td>
                    <td class="p-4 font-mono text-zinc-500">
                        <div class="flex items-center gap-2">
                            <span class="max-w-[120px] truncate" title="{{.ID}}">{{.ID}}</span>
                            <button onclick="copyToClipboard('{{.ID}}', 'id-{{$i}}')" class="text-zinc-600 hover:text-zinc-400 transition" title="Copy ID">
                                <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"></rect><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"></path></svg>
                            </button>
                            <span id="tooltip-id-{{$i}}" class="text-[9px] text-zinc-600 font-sans font-medium"></span>
                        </div>
                    </td>
                    <td class="p-4 font-mono text-emerald-400/80">
                        <div class="flex items-center gap-2">
                            <span class="max-w-[200px] truncate" title="{{.APIKey}}">{{.APIKey}}</span>
                            <button onclick="copyToClipboard('{{.APIKey}}', 'key-{{$i}}')" class="text-zinc-600 hover:text-zinc-400 transition" title="Copy Key">
                                <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"></rect><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"></path></svg>
                            </button>
                            <span id="tooltip-key-{{$i}}" class="text-[9px] text-zinc-600 font-sans font-medium"></span>
                        </div>
                    </td>
                    <td class="p-4">
                        <span class="px-2 py-0.5 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">ACTIVE</span>
                    </td>
                    <td class="p-4 text-right">
                        <a href="/dashboard/projects/{{.ID}}" class="inline-flex items-center rounded-md bg-zinc-800 hover:bg-zinc-700/80 px-3.5 py-1.5 text-xs font-bold text-zinc-300 border border-zinc-700/50 hover:border-zinc-600 transition">
                            Console →
                        </a>
                    </td>
                </tr>
                {{else}}
                <tr>
                    <td colspan="5" class="p-12 text-center text-zinc-500">
                        Belum ada project. Masukkan nama project di atas!
                    </td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>
</div>
{{end}}
`

// Halaman Detail Project - HANYA DATABASE TABLES
const projectHTML = `
{{define "content"}}
<!-- Breadcrumbs -->
<div class="mb-4">
    <nav class="flex text-xs font-semibold space-x-2 text-zinc-500">
        <a href="/dashboard" class="hover:text-zinc-300 transition">Dashboard</a>
        <span>/</span>
        <a href="/dashboard/projects-list" class="hover:text-zinc-300 transition">Projects</a>
        <span>/</span>
        <span class="text-zinc-300 font-bold">{{.Project.Name}}</span>
    </nav>
</div>

<div class="flex flex-col lg:flex-row lg:items-center lg:justify-between mb-8 pb-6 border-b border-premium gap-4">
    <div>
        <h1 class="text-3xl font-extrabold text-white tracking-tight">Database Tables</h1>
        <div class="mt-2 flex flex-wrap gap-x-6 gap-y-2 text-xs text-zinc-400">
            <span class="flex items-center gap-2">
                <span>Project ID:</span>
                <code class="text-zinc-300 font-mono bg-zinc-900 border border-premium px-2 py-0.5 rounded">{{.Project.ID}}</code>
                <button onclick="copyToClipboard('{{.Project.ID}}', 'proj-id')" class="text-zinc-600 hover:text-zinc-400 transition">
                    <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"></rect><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"></path></svg>
                </button>
                <span id="tooltip-proj-id" class="text-[9px] text-zinc-500"></span>
            </span>
            <span class="flex items-center gap-2">
                <span>API Key:</span>
                <code class="text-emerald-400 font-mono bg-zinc-900 border border-premium px-2 py-0.5 rounded">{{.Project.APIKey}}</code>
                <button onclick="copyToClipboard('{{.Project.APIKey}}', 'proj-key')" class="text-zinc-600 hover:text-emerald-400 transition">
                    <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"></rect><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"></path></svg>
                </button>
                <span id="tooltip-proj-key" class="text-[9px] text-zinc-500"></span>
            </span>
        </div>
    </div>
</div>

<div class="space-y-6">
    <div class="flex items-center justify-between">
        <h3 class="text-sm font-bold text-zinc-500 uppercase tracking-wider">Tabel Terdaftar</h3>
        <form action="/dashboard/projects/{{.Project.ID}}/tables" method="POST" class="flex gap-2">
            <input type="text" name="name" required placeholder="Nama Tabel Baru (e.g. posts)" class="rounded-lg bg-zinc-900 border border-premium px-3 py-1.5 text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-emerald-500">
            <button type="submit" class="inline-flex items-center gap-1.5 rounded-lg bg-emerald-500 hover:bg-emerald-400 px-3.5 py-1.5 text-xs font-semibold text-zinc-950 transition shadow">
                <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14"></path><path d="M12 5v14"></path></svg>
                Buat Tabel
            </button>
        </form>
    </div>

    <div class="rounded-xl glass-panel overflow-hidden">
        <table class="w-full text-left text-xs border-collapse">
            <thead>
                <tr class="bg-zinc-900/60 text-zinc-400 uppercase tracking-wider text-[9px] border-b border-premium font-bold">
                    <th class="p-4">Table Name</th>
                    <th class="p-4">Physical DB Name</th>
                    <th class="p-4 text-right">Actions</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-zinc-800/40">
                {{range $i, $e := .Tables}}
                <tr class="hover:bg-zinc-800/20 transition">
                    <td class="p-4 font-bold text-zinc-100 text-sm">
                        <span class="flex items-center gap-2 text-emerald-400/90">
                            <svg class="h-4 w-4 text-emerald-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                <path d="M12 3v18"></path>
                                <rect width="18" height="18" x="3" y="3" rx="2"></rect>
                                <path d="M3 9h18"></path>
                                <path d="M3 15h18"></path>
                            </svg>
                            <span class="text-zinc-100">{{.Name}}</span>
                        </span>
                    </td>
                    <td class="p-4 font-mono text-zinc-500">
                        <div class="flex items-center gap-1.5">
                            <span>p_{{.ProjectID | cleanUUID}}_{{.Name}}</span>
                            <button onclick="copyToClipboard('p_{{.ProjectID | cleanUUID}}_{{.Name}}', 'tbl-phys-{{$i}}')" class="text-zinc-600 hover:text-zinc-400 transition" title="Copy Physical Name">
                                <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"></rect><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"></path></svg>
                            </button>
                            <span id="tooltip-tbl-phys-{{$i}}" class="text-[8px] text-zinc-650"></span>
                        </div>
                    </td>
                    <td class="p-4 text-right">
                        <a href="/dashboard/projects/{{$.Project.ID}}/tables/{{.ID}}" class="inline-flex items-center gap-1.5 rounded-md bg-zinc-800 hover:bg-zinc-700/80 px-3.5 py-1.5 text-xs font-bold text-zinc-300 border border-zinc-700/50 hover:border-zinc-600 transition">
                            View Data
                            <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"></polyline></svg>
                        </a>
                    </td>
                </tr>
                {{else}}
                <tr>
                    <td colspan="3" class="p-12 text-center text-zinc-500">
                        Belum ada tabel yang terdefinisi. Buat tabel pertama Anda di atas!
                    </td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>
</div>
{{end}}
`

// Halaman Baru: USER AUTHENTICATION KHUSUS
const authHTML = `
{{define "content"}}
<!-- Breadcrumbs -->
<div class="mb-4">
    <nav class="flex text-xs font-semibold space-x-2 text-zinc-500">
        <a href="/dashboard" class="hover:text-zinc-300 transition">Dashboard</a>
        <span>/</span>
        <a href="/dashboard/projects-list" class="hover:text-zinc-300 transition">Projects</a>
        <span>/</span>
        <a href="/dashboard/projects/{{.Project.ID}}" class="hover:text-zinc-300 transition">{{.Project.Name}}</a>
        <span>/</span>
        <span class="text-zinc-300 font-bold">Authentication</span>
    </nav>
</div>

<div class="mb-8 border-b border-premium pb-6">
    <h1 class="text-3xl font-extrabold text-white tracking-tight">Authentication Console</h1>
    <p class="mt-1 text-sm text-zinc-500">Kelola pengguna terdaftar dan otorisasi JWT untuk project {{.Project.Name}}.</p>
</div>

<div class="grid grid-cols-1 lg:grid-cols-3 gap-8">
    <!-- Form Pendaftaran -->
    <div class="lg:col-span-1 rounded-xl glass-panel p-6 self-start space-y-4">
        <h3 class="text-sm font-bold text-white uppercase tracking-wider">Daftarkan User Baru</h3>
        <form action="/dashboard/projects/{{.Project.ID}}/users" method="POST" class="space-y-3">
            <div>
                <input type="email" name="email" required placeholder="Alamat Email" class="w-full rounded-lg bg-zinc-900 border border-premium px-3.5 py-2 text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-emerald-500 transition">
            </div>
            <div>
                <input type="password" name="password" required placeholder="Password" class="w-full rounded-lg bg-zinc-900 border border-premium px-3.5 py-2 text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-emerald-500 transition">
            </div>
            <button type="submit" class="w-full inline-flex justify-center items-center gap-1.5 rounded-lg bg-emerald-500 hover:bg-emerald-400 px-3 py-2.5 text-xs font-semibold text-zinc-950 transition shadow">
                <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"></path><circle cx="9" cy="7" r="4"></circle></svg>
                Tambahkan User
            </button>
        </form>
    </div>

    <!-- Tabel Daftar User -->
    <div class="lg:col-span-2 rounded-xl glass-panel overflow-hidden flex flex-col">
        <div class="p-5 border-b border-premium bg-zinc-900/40 flex justify-between items-center">
            <h3 class="text-sm font-bold text-white uppercase tracking-wider">User Registrations</h3>
            <span class="text-xs text-zinc-500 font-mono">Total: {{len .Users}}</span>
        </div>
        <div class="overflow-x-auto">
            <table class="w-full text-left text-xs border-collapse">
                <thead>
                    <tr class="bg-zinc-900/60 text-zinc-400 uppercase tracking-wider text-[9px] border-b border-premium font-bold">
                        <th class="p-4">User ID</th>
                        <th class="p-4">Email Address</th>
                        <th class="p-4">Registered At</th>
                    </tr>
                </thead>
                <tbody class="divide-y divide-zinc-800/40">
                    {{range $i, $u := .Users}}
                    <tr class="hover:bg-zinc-800/20 transition">
                        <td class="p-4 font-mono text-zinc-500">
                            <div class="flex items-center gap-1.5">
                                <span class="max-w-[100px] truncate" title="{{.ID}}">{{.ID}}</span>
                                <button onclick="copyToClipboard('{{.ID}}', 'u-id-{{$i}}')" class="text-zinc-700 hover:text-zinc-400 transition" title="Copy User ID">
                                    <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"></rect><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"></path></svg>
                                </button>
                                <span id="tooltip-u-id-{{$i}}" class="text-[8px] text-zinc-600"></span>
                            </div>
                        </td>
                        <td class="p-4 font-bold text-zinc-200 text-sm">{{.Email}}</td>
                        <td class="p-4 text-zinc-500 font-mono">{{.CreatedAt}}</td>
                    </tr>
                    {{else}}
                    <tr>
                        <td colspan="3" class="p-12 text-center text-zinc-500">
                            Belum ada user yang terdaftar dalam proyek ini.
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>
</div>
{{end}}
`

// Halaman Swagger UI kustom
const swaggerUIHTML = `
{{define "content"}}
<!-- Breadcrumbs -->
<div class="mb-4">
    <nav class="flex text-xs font-semibold space-x-2 text-zinc-500">
        <a href="/dashboard" class="hover:text-zinc-300 transition">Dashboard</a>
        <span>/</span>
        <a href="/dashboard/projects-list" class="hover:text-zinc-300 transition">Projects</a>
        <span>/</span>
        <a href="/dashboard/projects/{{.Project.ID}}" class="hover:text-zinc-300 transition">{{.Project.Name}}</a>
        <span>/</span>
        <span class="text-zinc-300 font-bold">API Docs (Swagger)</span>
    </nav>
</div>

<div class="mb-6 flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
    <div>
        <h1 class="text-3xl font-extrabold text-white tracking-tight">API Swagger Docs</h1>
        <p class="text-xs text-zinc-500 mt-2">OpenAPI Swagger spec yang digenerate otomatis menyesuaikan skema tabel project Anda.</p>
    </div>
    <a href="/api/projects/{{.Project.ID}}/swagger.json" target="_blank" class="inline-flex items-center gap-2 rounded-lg bg-zinc-800 hover:bg-zinc-700 px-3.5 py-2 text-xs font-semibold text-zinc-300 border border-premium transition">
        Open Raw swagger.json
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg>
    </a>
</div>

<div class="rounded-xl border border-premium overflow-hidden bg-[#1e1e1e] p-4 min-h-[600px] relative">
    <iframe src="/dashboard/projects/{{.Project.ID}}/swagger-iframe" class="w-full min-h-[650px] border-none" scrolling="yes"></iframe>
</div>
{{end}}
`

const swaggerIframeHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Coderbase API Specs</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui.css" />
    <style>
        body { background-color: #121212; color: #fff; margin: 0; padding: 20px; }
        .swagger-ui { 
            filter: invert(90%) hue-rotate(185deg); 
        }
        .swagger-ui .topbar { display: none !important; }
        .swagger-ui .info { margin: 20px 0 !important; }
        .swagger-ui .info .title { color: #fff !important; }
        .swagger-ui .opblock .opblock-summary-operation-id { color: #fff !important; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-bundle.js"></script>
    <script>
        window.onload = () => {
            window.ui = SwaggerUIBundle({
                url: '/api/projects/{{.ProjectID}}/swagger.json',
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                ],
            });
        };
    </script>
</body>
</html>
`

// Halaman Detail Tabel dengan Navigasi Tab (Ikon-ikon SVG baru dipasang di sini)
const tableHTML = `
{{define "content"}}
<!-- Breadcrumbs -->
<div class="mb-4">
    <nav class="flex text-xs font-semibold space-x-2 text-zinc-500">
        <a href="/dashboard" class="hover:text-zinc-300 transition">Dashboard</a>
        <span>/</span>
        <a href="/dashboard/projects-list" class="hover:text-zinc-300 transition">Projects</a>
        <span>/</span>
        <a href="/dashboard/projects/{{.Project.ID}}" class="hover:text-zinc-300 transition">{{.Project.Name}}</a>
        <span>/</span>
        <span class="text-zinc-300 font-bold">{{.Table.Name}}</span>
    </nav>
</div>

<!-- Header -->
<div class="mb-6 flex flex-col md:flex-row md:items-center md:justify-between gap-4 border-b border-premium pb-6">
    <div class="flex items-center gap-3">
        <!-- Ikon Database SVG -->
        <span class="p-2 rounded-lg bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 shrink-0">
            <svg class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <ellipse cx="12" cy="5" rx="9" ry="3"></ellipse>
                <path d="M3 5V19A9 3 0 0 0 21 19V5"></path>
                <path d="M3 12A9 3 0 0 0 21 12"></path>
            </svg>
        </span>
        <div>
            <h1 class="text-3xl font-extrabold text-white tracking-tight">{{.Table.Name}}</h1>
            <p class="text-xs text-zinc-500 mt-1.5 font-light">Physical Name: <code class="bg-zinc-950 px-2 py-0.5 rounded font-mono text-zinc-400">p_{{.Project.ID | cleanUUID}}_{{.Table.Name}}</code></p>
        </div>
    </div>
    
    <div class="shrink-0 flex items-center gap-4">
        <!-- RLS Status Badge -->
        <div class="inline-flex items-center gap-2 bg-zinc-900 border border-premium px-3 py-1.5 rounded-lg text-xs">
            <span class="font-bold text-zinc-500">RLS Status:</span>
            {{if .Policies}}
            <span class="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">ACTIVE</span>
            {{else}}
            <span class="px-2 py-0.5 rounded text-[10px] font-bold bg-amber-500/10 text-amber-400 border border-amber-500/20">INACTIVE</span>
            {{end}}
        </div>
    </div>
</div>

<!-- Tab Navigation Menu (Ikon SVG Premium) -->
<div class="mb-6 border-b border-zinc-800/80">
    <nav class="flex space-x-6 text-sm font-semibold" aria-label="Tabs">
        <button onclick="switchTab('tab-data')" id="btn-tab-data" class="tab-btn pb-4 px-1 border-b-2 border-emerald-500 text-emerald-400 transition inline-flex items-center gap-2">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <rect width="18" height="18" x="3" y="3" rx="2" ry="2"></rect>
                <path d="M3 9h18"></path>
                <path d="M3 15h18"></path>
                <path d="M9 3v18"></path>
                <path d="M15 3v18"></path>
            </svg>
            Data Viewer
        </button>
        <button onclick="switchTab('tab-schema')" id="btn-tab-schema" class="tab-btn pb-4 px-1 border-b-2 border-transparent text-zinc-400 hover:text-zinc-200 transition inline-flex items-center gap-2">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M12 3v18"></path>
                <rect width="18" height="18" x="3" y="3" rx="2"></rect>
                <path d="M3 9h18"></path>
                <path d="M3 15h18"></path>
            </svg>
            Schema Columns
        </button>
        <button onclick="switchTab('tab-rls')" id="btn-tab-rls" class="tab-btn pb-4 px-1 border-b-2 border-transparent text-zinc-400 hover:text-zinc-200 transition inline-flex items-center gap-2">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path>
            </svg>
            Security Policies (RLS)
        </button>
    </nav>
</div>

<!-- Tabs Contents -->
<div class="space-y-6">    <!-- Tab 1: DATA VIEWER CONTAINER -->
    <div id="content-tab-data" class="tab-content block">
        <div class="rounded-xl glass-panel overflow-hidden flex flex-col">
            <div class="p-5 border-b border-premium flex justify-between items-center bg-zinc-900/40">
                <h3 class="text-sm font-bold text-white uppercase tracking-wider">Data Viewer Grid</h3>
                <div class="flex items-center gap-3">
                    <button onclick="toggleImportPanel()" class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 border border-zinc-700/50 hover:border-zinc-600 text-xs font-semibold text-zinc-300 transition">
                        <svg class="h-3.5 w-3.5 text-emerald-450" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                            <polyline points="17 8 12 3 7 8"></polyline>
                            <line x1="12" y1="3" x2="12" y2="15"></line>
                        </svg>
                        Import JSON
                    </button>
                    <span class="text-xs text-zinc-500">Menampilkan 50 data terakhir</span>
                </div>
            </div>

            <!-- Panel Import JSON -->
            <div id="import-json-panel" class="hidden p-5 border-b border-premium bg-zinc-900/60 space-y-4">
                <div class="flex justify-between items-center">
                    <h4 class="text-xs font-bold text-white uppercase tracking-wider">Import Data dari JSON (Array atau Object)</h4>
                    <button onclick="toggleImportPanel()" class="text-zinc-500 hover:text-zinc-300">✕</button>
                </div>
                <form action="/dashboard/projects/{{.Project.ID}}/tables/{{.Table.ID}}/import-json" method="POST" class="space-y-3">
                    <textarea name="json_data" rows="6" required placeholder='Paste data JSON di sini...
Contoh:
[
  { "Actor": "Miu Shiromine", "Code": "DSOD-009", "Studio": "Das !" }
]' class="w-full font-mono text-xs rounded-lg bg-zinc-950 border border-premium p-3 text-zinc-300 placeholder-zinc-650 focus:outline-none focus:border-emerald-500"></textarea>
                    <div class="flex justify-end gap-3">
                        <button type="button" onclick="toggleImportPanel()" class="px-3.5 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-xs font-semibold text-zinc-300 transition">Cancel</button>
                        <button type="submit" class="px-3.5 py-1.5 rounded-lg bg-emerald-500 hover:bg-emerald-400 text-xs font-semibold text-zinc-950 transition shadow">Mulai Import</button>
                    </div>
                </form>
            </div>

            <div class="overflow-x-auto">
                <table class="w-full text-left text-xs border-collapse">
                    <thead>
                        <tr class="bg-zinc-900/60 text-zinc-400 uppercase tracking-wider text-[9px] border-b border-premium font-bold whitespace-nowrap">
                            <th class="p-4 font-mono">ID</th>
                            {{range .Columns}}
                            <th class="p-4">{{.Name}}</th>
                            {{end}}
                            <th class="p-4">Created At</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-zinc-800/40 bg-zinc-900/10">
                        {{range $i, $row := .Rows}}
                        <tr class="hover:bg-zinc-800/20 transition">
                            <td class="p-4 font-mono text-zinc-500">
                                <div class="flex items-center gap-1.5">
                                    <span class="max-w-[70px] truncate" title="{{index . "id"}}">{{index . "id"}}</span>
                                    <button onclick="copyToClipboard('{{index . "id"}}', 'row-id-{{$i}}')" class="text-zinc-755 hover:text-zinc-400 transition" title="Copy Record ID">
                                        <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"></rect><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"></path></svg>
                                    </button>
                                    <span id="tooltip-row-id-{{$i}}" class="text-[8px] text-zinc-655 font-sans"></span>
                                </div>
                            </td>
                            {{range $.Columns}}
                            <td class="p-4 text-zinc-300 whitespace-nowrap">
                                {{$val := index $row .Name}}
                                {{if $val}}
                                <div class="max-w-[280px] truncate font-sans text-xs" title="{{$val}}">{{$val}}</div>
                                {{else}}
                                <span class="text-zinc-650 italic select-none">null</span>
                                {{end}}
                            </td>
                            {{end}}
                            <td class="p-4 text-zinc-550 font-mono whitespace-nowrap">{{index . "created_at"}}</td>
                        </tr>
                        {{else}}
                        <tr>
                            <td colspan="100%" class="p-12 text-center text-zinc-600 font-light">
                                <span class="flex justify-center mb-2 text-zinc-600">
                                    <svg class="h-8 w-8" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                                        <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z"></path>
                                    </svg>
                                </span>
                                <span>Tabel ini masih kosong. Silakan isi data melalui REST API atau tombol Import JSON.</span>
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>
    </div>

    <!-- Tab 2: SCHEMA COLUMNS EDITOR -->
    <div id="content-tab-schema" class="tab-content hidden">
        <div class="grid grid-cols-1 lg:grid-cols-3 gap-8">
            <div class="lg:col-span-2 rounded-xl glass-panel overflow-hidden">
                <div class="p-5 border-b border-premium bg-zinc-900/40">
                    <h3 class="text-sm font-bold text-white uppercase tracking-wider">Daftar Kolom Skema</h3>
                </div>
                <div class="overflow-x-auto">
                    <table class="w-full text-left text-xs border-collapse">
                        <thead>
                            <tr class="bg-zinc-900/60 text-zinc-400 uppercase tracking-wider text-[9px] border-b border-premium font-bold">
                                <th class="p-4">Column Name</th>
                                <th class="p-4">Type</th>
                                <th class="p-4">Is Nullable</th>
                                <th class="p-4">Constraint</th>
                                <th class="p-4 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody class="divide-y divide-zinc-800/40">
                            <tr class="hover:bg-zinc-800/10">
                                <td class="p-4 font-mono font-bold text-zinc-300">id</td>
                                <td class="p-4 font-mono text-zinc-500">UUID</td>
                                <td class="p-4 text-zinc-550">NO</td>
                                <td class="p-4"><span class="px-2 py-0.5 text-[9px] rounded font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">PRIMARY KEY</span></td>
                                <td class="p-4 text-right text-zinc-650">—</td>
                            </tr>
                            <tr class="hover:bg-zinc-800/10">
                                <td class="p-4 font-mono font-bold text-zinc-300">project_id</td>
                                <td class="p-4 font-mono text-zinc-500">UUID</td>
                                <td class="p-4 text-zinc-550">NO</td>
                                <td class="p-4"><span class="px-2 py-0.5 text-[9px] rounded font-bold bg-zinc-800 text-zinc-400 border border-zinc-700/60">FOREIGN KEY</span></td>
                                <td class="p-4 text-right text-zinc-650">—</td>
                            </tr>
                            {{range .Columns}}
                            <tr class="hover:bg-zinc-800/10">
                                <td class="p-4 font-mono font-bold text-emerald-400">{{.Name}}</td>
                                <td class="p-4 font-mono text-zinc-450 uppercase">{{.Type}}</td>
                                <td class="p-4 font-mono text-zinc-500">{{if .IsNullable}}YES{{else}}NO{{end}}</td>
                                <td class="p-4 text-zinc-650">—</td>
                                <td class="p-4 text-right">
                                    <form action="/dashboard/projects/{{$.Project.ID}}/tables/{{$.Table.ID}}/columns/{{.ID}}/delete" method="POST" class="inline" onsubmit="return confirm('Apakah Anda yakin ingin menghapus kolom \'{{.Name}}\'? Data di kolom ini akan dihapus secara permanen.')">
                                        <button type="submit" class="text-rose-500 hover:text-rose-450 hover:bg-rose-500/10 p-1.5 rounded-md transition duration-150 inline-flex items-center justify-center" title="Delete Column">
                                            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                                <polyline points="3 6 5 6 21 6"></polyline>
                                                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                                            </svg>
                                        </button>
                                    </form>
                                </td>
                            </tr>
                            {{end}}
                            <tr class="hover:bg-zinc-800/10">
                                <td class="p-4 font-mono font-bold text-zinc-300">created_at</td>
                                <td class="p-4 font-mono text-zinc-500">DATETIME</td>
                                <td class="p-4 text-zinc-550">YES</td>
                                <td class="p-4 text-zinc-650">—</td>
                                <td class="p-4 text-right text-zinc-650">—</td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </div>

            <!-- Form Tambah Kolom -->
            <div class="lg:col-span-1 rounded-xl glass-panel p-6 self-start space-y-4">
                <h4 class="text-xs font-bold text-white uppercase tracking-wider">Tambah Kolom Baru</h4>
                <form action="/dashboard/projects/{{.Project.ID}}/tables/{{.Table.ID}}/columns" method="POST" class="space-y-4">
                    <div>
                        <label class="block text-[10px] font-bold text-zinc-500 uppercase mb-1.5">Column Name</label>
                        <input type="text" name="name" required placeholder="Contoh: price" class="w-full rounded-lg bg-zinc-900 border border-premium px-3.5 py-2 text-xs text-white focus:outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500/20 transition">
                    </div>
                    <div>
                        <label class="block text-[10px] font-bold text-zinc-500 uppercase mb-1.5">Data Type</label>
                        <select name="type" class="w-full rounded-lg bg-zinc-900 border border-premium px-3.5 py-2 text-xs text-white focus:outline-none focus:border-emerald-500 transition">
                            <option value="text">TEXT (String)</option>
                            <option value="integer">INTEGER (Angka)</option>
                            <option value="boolean">BOOLEAN (true/false)</option>
                            <option value="timestamp">TIMESTAMP (Tanggal/Waktu)</option>
                            <option value="jsonb">JSONB (Kompleks JSON)</option>
                        </select>
                    </div>
                    <div class="flex items-center gap-2">
                        <input type="checkbox" name="is_nullable" checked id="nullable_check_t" class="rounded border-zinc-800 bg-zinc-905 text-emerald-500 focus:ring-0">
                        <label for="nullable_check_t" class="text-xs text-zinc-400 select-none cursor-pointer">Boleh bernilai kosong (Nullable)</label>
                    </div>
                    <button type="submit" class="w-full inline-flex justify-center items-center gap-1.5 rounded-lg bg-emerald-500 hover:bg-emerald-400 px-3 py-2.5 text-xs font-bold text-zinc-955 transition shadow duration-150">
                        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"></path><polyline points="17 21 17 13 7 13 7 21"></polyline><polyline points="7 3 7 8 15 8"></polyline></svg>
                        Simpan Kolom
                    </button>
                </form>
            </div>
        </div>
    </div>

    <!-- Tab 3: SECURITY POLICIES (RLS) -->
    <div id="content-tab-rls" class="tab-content hidden space-y-6">
        <!-- Box Panduan RLS -->
        <div class="bg-emerald-500/5 border border-emerald-500/10 rounded-xl p-5 text-xs text-zinc-400 space-y-3 leading-relaxed flex gap-3">
            <span class="text-emerald-400 shrink-0">
                <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <circle cx="12" cy="12" r="10"></circle>
                    <line x1="12" y1="16" x2="12" y2="12"></line>
                    <line x1="12" y1="8" x2="12.01" y2="8"></line>
                </svg>
            </span>
            <div>
                <div class="text-emerald-400 font-bold mb-1">
                    Panduan Setup RLS untuk Pemula:
                </div>
                <p class="font-light">
                    Secara default, jika tabel **tidak memiliki policy** apa pun, maka **RLS tidak aktif (Public Access)**. Semua orang yang memegang API Key proyek Anda bisa membaca dan menulis data ke tabel ini. Begitu Anda menambahkan minimal 1 policy, RLS akan otomatis menyala dan mengunci akses selain yang diatur oleh policy.
                </p>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-6 pt-3 mt-3 border-t border-zinc-800/40">
                    <div>
                        <span class="font-semibold text-zinc-300 block mb-1">🔓 Skenario A: Akses Publik</span>
                        <p class="text-[11px] text-zinc-500 font-light leading-relaxed">
                            Cocok untuk data publik (e.g., artikel blog). Buat policy dengan: 
                            <br><strong>Role: ANON</strong>, <strong>Expression: true</strong>.
                        </p>
                    </div>
                    <div>
                        <span class="font-semibold text-zinc-300 block mb-1">🔐 Skenario B: Hak Milik (Owner Only)</span>
                        <p class="text-[11px] text-zinc-500 font-light leading-relaxed">
                            User hanya bisa melihat/mengubah data miliknya sendiri. Pastikan tabel memiliki kolom <code class="text-zinc-300 bg-zinc-900 px-1 py-0.5 rounded font-mono">user_id</code>, lalu buat policy dengan:
                            <br><strong>Role: AUTHENTICATED</strong>, <strong>Expression: auth.uid() = user_id</strong>.
                        </p>
                    </div>
                </div>
            </div>
        </div>

        <div class="grid grid-cols-1 lg:grid-cols-3 gap-8">
            <!-- List Policies -->
            <div class="lg:col-span-2 rounded-xl glass-panel overflow-hidden">
                <div class="p-5 border-b border-premium bg-zinc-900/40">
                    <h3 class="text-sm font-bold text-white uppercase tracking-wider">Active Security Policies</h3>
                </div>
                <div class="overflow-x-auto">
                    <table class="w-full text-left text-xs border-collapse">
                        <thead>
                            <tr class="bg-zinc-900/60 text-zinc-400 uppercase tracking-wider text-[9px] border-b border-premium font-bold">
                                <th class="p-4">Action</th>
                                <th class="p-4">Role</th>
                                <th class="p-4">Expression / SQL Rule</th>
                                <th class="p-4 text-right">Delete</th>
                            </tr>
                        </thead>
                        <tbody class="divide-y divide-zinc-800/40">
                            {{range .Policies}}
                            <tr class="hover:bg-zinc-800/20 transition">
                                <td class="p-4"><span class="px-2.5 py-0.5 rounded text-[10px] font-bold bg-zinc-800 text-zinc-300 border border-zinc-700/60">{{.Action}}</span></td>
                                <td class="p-4 font-semibold text-emerald-400">{{.Role}}</td>
                                <td class="p-4 font-mono text-zinc-400 text-[11px]">{{.Expression}}</td>
                                <td class="p-4 text-right">
                                    <form action="/dashboard/projects/{{$.Project.ID}}/tables/{{$.Table.ID}}/policies/delete" method="POST" class="inline">
                                        <input type="hidden" name="policy_id" value="{{.ID}}">
                                        <button type="submit" class="text-rose-500 hover:text-rose-450 hover:bg-rose-500/10 p-1.5 rounded-md transition duration-150 inline-flex items-center justify-center" title="Delete Policy">
                                            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                                <polyline points="3 6 5 6 21 6"></polyline>
                                                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                                            </svg>
                                        </button>
                                    </form>
                                </td>
                            </tr>
                            {{else}}
                            <tr>
                                <td colspan="4" class="p-12 text-center text-zinc-650 font-light">
                                    RLS belum aktif pada tabel ini (Akses terbuka bebas).
                                </td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                </div>
            </div>

            <!-- Form Tambah Policy -->
            <div class="lg:col-span-1 rounded-xl glass-panel p-6 self-start space-y-4">
                <h4 class="text-xs font-bold text-white uppercase tracking-wider">Buat Policy Baru</h4>
                <form action="/dashboard/projects/{{.Project.ID}}/tables/{{.Table.ID}}/policies" method="POST" class="space-y-4">
                    <div>
                        <label class="block text-[10px] font-bold text-zinc-500 uppercase mb-1.5">Action</label>
                        <select name="action" class="w-full rounded-lg bg-zinc-900 border border-premium px-3.5 py-2 text-xs text-white focus:outline-none focus:border-emerald-500 transition">
                            <option value="SELECT">SELECT (Membaca)</option>
                            <option value="INSERT">INSERT (Menulis)</option>
                            <option value="UPDATE">UPDATE (Mengubah)</option>
                            <option value="DELETE">DELETE (Menghapus)</option>
                        </select>
                    </div>
                    <div>
                        <label class="block text-[10px] font-bold text-zinc-500 uppercase mb-1.5">Target Role</label>
                        <select name="role" class="w-full rounded-lg bg-zinc-900 border border-premium px-3.5 py-2 text-xs text-white focus:outline-none focus:border-emerald-500 transition">
                            <option value="authenticated">AUTHENTICATED (Wajib Login)</option>
                            <option value="anon">ANON (Publik)</option>
                        </select>
                    </div>
                    <div>
                        <label class="block text-[10px] font-bold text-zinc-500 uppercase mb-1.5">Filter Rule</label>
                        <select name="expression" class="w-full rounded-lg bg-zinc-900 border border-premium px-3.5 py-2 text-xs text-white focus:outline-none focus:border-emerald-500 transition">
                            <option value="auth.uid() = user_id">auth.uid() = user_id (Hanya Pemilik Data)</option>
                            <option value="true">true (Diizinkan Bebas)</option>
                        </select>
                    </div>
                    <button type="submit" class="w-full inline-flex justify-center items-center gap-1.5 rounded-lg bg-emerald-500 hover:bg-emerald-400 px-3 py-2.5 text-xs font-bold text-zinc-950 transition shadow">
                        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path></svg>
                        Simpan Policy
                    </button>
                </form>
            </div>
        </div>
    </div>

</div>

<!-- Client Side Javascript Tab Switcher -->
<script>
    function switchTab(tabId) {
        document.querySelectorAll('.tab-content').forEach(function(content) {
            content.classList.remove('block');
            content.classList.add('hidden');
        });

        document.querySelectorAll('.tab-btn').forEach(function(btn) {
            btn.classList.remove('border-emerald-500', 'text-emerald-400');
            btn.classList.add('border-transparent', 'text-zinc-400');
        });

        const activeContent = document.getElementById('content-' + tabId);
        if (activeContent) {
            activeContent.classList.remove('hidden');
            activeContent.classList.add('block');
        }

        const activeBtn = document.getElementById('btn-' + tabId);
        if (activeBtn) {
            activeBtn.classList.remove('border-transparent', 'text-zinc-400');
            activeBtn.classList.add('border-emerald-500', 'text-emerald-400');
        }
    }
    function toggleImportPanel() {
        const panel = document.getElementById('import-json-panel');
        if (panel) {
            panel.classList.toggle('hidden');
        }
    }
</script>
{{end}}
`

// Tipe data view structs
type Project struct {
	ID        string
	Name      string
	APIKey    string
	CreatedAt string
}

type Table struct {
	ID        string
	ProjectID string
	Name      string
	CreatedAt string
}

type Column struct {
	ID         string
	TableID    string
	Name       string
	Type       string
	IsNullable bool
}

type User struct {
	ID        string
	ProjectID string
	Email     string
	CreatedAt string
}

type LogItem struct {
	Method    string
	Path      string
	Status    int
	Latency   int64
	CreatedAt string
}

type StatsInfo struct {
	Projects int
	Tables   int
	Users    int
}

func getSessionToken() string {
	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = "admin123"
	}
	// Gunakan HMAC-SHA256 agar password tidak bisa di-reverse dari cookie value
	h := hmac.New(sha256.New, []byte("coderbase_session_secret_key_v1"))
	h.Write([]byte(adminPass))
	return fmt.Sprintf("cb_sess_%x", h.Sum(nil))
}

func DashboardAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dashboard/login" {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie("coderbase_session")
		expected := getSessionToken()
		if err != nil || cookie.Value != expected {
			http.Redirect(w, r, "/dashboard/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RegisterDashboardRoutes(r chi.Router) {
	r.Route("/dashboard", func(r chi.Router) {
		r.Use(DashboardAuthMiddleware)
		r.Get("/login", handleDashboardLoginGet)
		r.Post("/login", handleDashboardLoginPost)
		r.Post("/logout", handleDashboardLogout)

		r.Get("/", handleIndex)
		r.Get("/projects-list", handleProjectsList)
		r.Post("/projects", handleDashboardCreateProject)
		r.Get("/projects/{project_id}", handleDashboardProjectDetail)
		r.Get("/projects/{project_id}/auth", handleDashboardProjectAuth)
		r.Get("/projects/{project_id}/docs", handleDashboardProjectDocs)
		r.Get("/projects/{project_id}/swagger-iframe", handleDashboardSwaggerIframe)
		r.Post("/projects/{project_id}/users", handleDashboardCreateUser)
		r.Post("/projects/{project_id}/tables", handleDashboardCreateTable)
		r.Get("/projects/{project_id}/tables/{table_id}", handleDashboardTableDetail)
		r.Post("/projects/{project_id}/tables/{table_id}/columns", handleDashboardAddColumn)
		r.Post("/projects/{project_id}/tables/{table_id}/columns/{column_id}/delete", handleDashboardDeleteColumn)
		r.Post("/projects/{project_id}/tables/{table_id}/import-json", handleDashboardImportJSON)
		r.Post("/projects/{project_id}/tables/{table_id}/policies", handleDashboardCreatePolicy)
		r.Post("/projects/{project_id}/tables/{table_id}/policies/delete", handleDashboardDeletePolicy)
	})
}

func cleanUUIDFunc(uuidStr string) string {
	return strings.ReplaceAll(uuidStr, "-", "_")
}

func renderWithLayout(w http.ResponseWriter, name string, contentTmpl string, data map[string]interface{}) {
	data["DBType"] = "PostgreSQL"

	tmpl, err := template.New("layout").Funcs(template.FuncMap{
		"cleanUUID": cleanUUIDFunc,
	}).Parse(layoutHTML)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl, err = tmpl.Parse(contentTmpl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	var stats StatsInfo

	_ = db.DB.QueryRow("SELECT COUNT(*) FROM projects").Scan(&stats.Projects)
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM tables").Scan(&stats.Tables)
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.Users)

	rows, err := db.DB.Query("SELECT id, name, api_key, created_at FROM projects ORDER BY created_at DESC LIMIT 5")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.APIKey, &p.CreatedAt); err == nil {
			projects = append(projects, p)
		}
	}

	logRows, err := db.DB.Query("SELECT method, path, status, latency_ms, created_at FROM logs ORDER BY created_at DESC LIMIT 15")
	logs := []LogItem{}
	if err == nil {
		defer logRows.Close()
		for logRows.Next() {
			var l LogItem
			if err := logRows.Scan(&l.Method, &l.Path, &l.Status, &l.Latency, &l.CreatedAt); err == nil {
				logs = append(logs, l)
			}
		}
	}

	renderWithLayout(w, "dashboard", dashboardHTML, map[string]interface{}{
		"Stats":    stats,
		"Projects": projects,
		"Logs":     logs,
	})
}

func handleProjectsList(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query("SELECT id, name, api_key, created_at FROM projects ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.APIKey, &p.CreatedAt); err == nil {
			projects = append(projects, p)
		}
	}

	renderWithLayout(w, "projects", projectsHTML, map[string]interface{}{
		"Projects": projects,
	})
}

func handleDashboardCreateProject(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if name != "" {
		_, _, _ = schema.CreateProject(name)
	}
	http.Redirect(w, r, "/dashboard/projects-list", http.StatusSeeOther)
}

func handleDashboardProjectDetail(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")

	var p Project
	err := db.DB.QueryRow("SELECT id, name, api_key, created_at FROM projects WHERE id = $1", projectID).Scan(&p.ID, &p.Name, &p.APIKey, &p.CreatedAt)
	if err != nil {
		http.Error(w, "Project tidak ditemukan", http.StatusNotFound)
		return
	}

	rows, err := db.DB.Query("SELECT id, project_id, name, created_at FROM tables WHERE project_id = $1 ORDER BY name ASC", projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tables := []Table{}
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Name, &t.CreatedAt); err == nil {
			tables = append(tables, t)
		}
	}

	renderWithLayout(w, "project", projectHTML, map[string]interface{}{
		"ActiveProjectID":   p.ID,
		"ActiveProjectName": p.Name,
		"Project":           p,
		"Tables":            tables,
	})
}

func handleDashboardProjectAuth(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")

	var p Project
	err := db.DB.QueryRow("SELECT id, name, api_key, created_at FROM projects WHERE id = $1", projectID).Scan(&p.ID, &p.Name, &p.APIKey, &p.CreatedAt)
	if err != nil {
		http.Error(w, "Project tidak ditemukan", http.StatusNotFound)
		return
	}

	uRows, err := db.DB.Query("SELECT id, project_id, email, created_at FROM users WHERE project_id = $1 ORDER BY created_at DESC", projectID)
	users := []User{}
	if err == nil {
		defer uRows.Close()
		for uRows.Next() {
			var u User
			if err := uRows.Scan(&u.ID, &u.ProjectID, &u.Email, &u.CreatedAt); err == nil {
				users = append(users, u)
			}
		}
	}

	renderWithLayout(w, "auth", authHTML, map[string]interface{}{
		"ActiveProjectID":   p.ID,
		"ActiveProjectName": p.Name,
		"Project":           p,
		"Users":             users,
	})
}

func handleDashboardProjectDocs(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")

	var p Project
	err := db.DB.QueryRow("SELECT id, name, api_key FROM projects WHERE id = $1", projectID).Scan(&p.ID, &p.Name, &p.APIKey)
	if err != nil {
		http.Error(w, "Project tidak ditemukan", http.StatusNotFound)
		return
	}

	renderWithLayout(w, "docs", swaggerUIHTML, map[string]interface{}{
		"ActiveProjectID":   p.ID,
		"ActiveProjectName": p.Name,
		"Project":           p,
	})
}

func handleDashboardSwaggerIframe(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	
	tmpl, err := template.New("iframe").Parse(swaggerIframeHTML)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = tmpl.Execute(w, map[string]string{"ProjectID": projectID})
}

func handleDashboardCreateUser(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	email := r.FormValue("email")
	password := r.FormValue("password")

	if email != "" && password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err == nil {
			userID := uuid.New().String()
			query := `INSERT INTO users (id, project_id, email, password_hash) VALUES ($1, $2, $3, $4)`
			_, _ = db.DB.Exec(query, userID, projectID, email, string(hashedPassword))
		}
	}

	http.Redirect(w, r, fmt.Sprintf("/dashboard/projects/%s/auth", projectID), http.StatusSeeOther)
}

func handleDashboardCreateTable(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	name := r.FormValue("name")
	if name != "" {
		_, _ = schema.CreateTable(projectID, name)
	}
	http.Redirect(w, r, fmt.Sprintf("/dashboard/projects/%s", projectID), http.StatusSeeOther)
}

func handleDashboardTableDetail(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	tableID := chi.URLParam(r, "table_id")

	var p Project
	_ = db.DB.QueryRow("SELECT id, name, api_key FROM projects WHERE id = $1", projectID).Scan(&p.ID, &p.Name, &p.APIKey)

	var t Table
	err := db.DB.QueryRow("SELECT id, project_id, name FROM tables WHERE id = $1 AND project_id = $2", tableID, projectID).Scan(&t.ID, &t.ProjectID, &t.Name)
	if err != nil {
		http.Error(w, "Tabel tidak ditemukan", http.StatusNotFound)
		return
	}

	rows, err := db.DB.Query("SELECT id, table_id, name, type, is_nullable FROM columns WHERE table_id = $1 ORDER BY name ASC", tableID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	columns := []Column{}
	for rows.Next() {
		var col Column
		if err := rows.Scan(&col.ID, &col.TableID, &col.Name, &col.Type, &col.IsNullable); err == nil {
			columns = append(columns, col)
		}
	}

	pRows, err := db.DB.Query("SELECT id, table_id, action, role, expression FROM policies WHERE table_id = $1 ORDER BY action ASC", tableID)
	policies := []policy.Policy{}
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var pol policy.Policy
			if err := pRows.Scan(&pol.ID, &pol.TableID, &pol.Action, &pol.Role, &pol.Expression); err == nil {
				policies = append(policies, pol)
			}
		}
	}

	physicalTable := schema.FormatPhysicalTableName(projectID, t.Name)
	var dbRows []map[string]interface{}
	
	queryStr := fmt.Sprintf("SELECT * FROM %s WHERE project_id = $1 LIMIT 50", physicalTable)
	dataRows, err := db.DB.Query(queryStr, projectID)
	if err == nil {
		defer dataRows.Close()
		colNames, _ := dataRows.Columns()
		for dataRows.Next() {
			cols := make([]interface{}, len(colNames))
			colPtrs := make([]interface{}, len(colNames))
			for i := range cols {
				colPtrs[i] = &cols[i]
			}
			if err := dataRows.Scan(colPtrs...); err == nil {
				rowMap := make(map[string]interface{})
				for i, colName := range colNames {
					val := cols[i]
					if b, ok := val.([]byte); ok {
						rowMap[colName] = string(b)
					} else {
						rowMap[colName] = val
					}
				}
				dbRows = append(dbRows, rowMap)
			}
		}
	}

	renderWithLayout(w, "table", tableHTML, map[string]interface{}{
		"ActiveProjectID":   p.ID,
		"ActiveProjectName": p.Name,
		"Project":           p,
		"Table":             t,
		"Columns":           columns,
		"Policies":          policies,
		"Rows":              dbRows,
	})
}

func handleDashboardAddColumn(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	tableID := chi.URLParam(r, "table_id")
	name := r.FormValue("name")
	colType := r.FormValue("type")
	isNullable := r.FormValue("is_nullable") == "on"

	if name != "" && colType != "" {
		_, _ = schema.AddColumn(projectID, tableID, name, colType, isNullable)
	}
	http.Redirect(w, r, fmt.Sprintf("/dashboard/projects/%s/tables/%s", projectID, tableID), http.StatusSeeOther)
}

func handleDashboardCreatePolicy(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	tableID := chi.URLParam(r, "table_id")
	action := r.FormValue("action")
	role := r.FormValue("role")
	expression := r.FormValue("expression")

	if action != "" && role != "" && expression != "" {
		_, _ = policy.CreatePolicy(tableID, action, role, expression)
	}
	http.Redirect(w, r, fmt.Sprintf("/dashboard/projects/%s/tables/%s", projectID, tableID), http.StatusSeeOther)
}

func handleDashboardDeletePolicy(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	tableID := chi.URLParam(r, "table_id")
	policyID := r.FormValue("policy_id")

	if policyID != "" {
		query := `DELETE FROM policies WHERE id = $1 AND table_id = $2`
		_, _ = db.DB.Exec(query, policyID, tableID)
	}
	http.Redirect(w, r, fmt.Sprintf("/dashboard/projects/%s/tables/%s", projectID, tableID), http.StatusSeeOther)
}

func handleDashboardDeleteColumn(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	tableID := chi.URLParam(r, "table_id")
	columnID := chi.URLParam(r, "column_id")

	if projectID != "" && tableID != "" && columnID != "" {
		_ = schema.DropColumn(projectID, tableID, columnID)
	}
	http.Redirect(w, r, fmt.Sprintf("/dashboard/projects/%s/tables/%s", projectID, tableID), http.StatusSeeOther)
}

func handleDashboardImportJSON(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	tableID := chi.URLParam(r, "table_id")
	jsonData := r.FormValue("json_data")

	var t Table
	err := db.DB.QueryRow("SELECT id, project_id, name FROM tables WHERE id = $1 AND project_id = $2", tableID, projectID).Scan(&t.ID, &t.ProjectID, &t.Name)
	if err != nil {
		http.Error(w, "Tabel tidak ditemukan", http.StatusNotFound)
		return
	}

	if jsonData != "" {
		var rowsArray []map[string]interface{}
		if err := json.Unmarshal([]byte(jsonData), &rowsArray); err != nil {
			var singleRow map[string]interface{}
			if err2 := json.Unmarshal([]byte(jsonData), &singleRow); err2 == nil {
				rowsArray = append(rowsArray, singleRow)
			} else {
				http.Error(w, "Format JSON tidak valid. Harus berupa Object atau Array of Objects.", http.StatusBadRequest)
				return
			}
		}

		colRows, err := db.DB.Query("SELECT name FROM columns WHERE table_id = $1", tableID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer colRows.Close()

		validCols := map[string]bool{"id": true, "project_id": true, "created_at": true, "updated_at": true}
		for colRows.Next() {
			var colName string
			if err := colRows.Scan(&colName); err == nil {
				validCols[colName] = true
			}
		}

		physicalTable := schema.FormatPhysicalTableName(projectID, t.Name)

		tx, err := db.DB.Begin()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		for _, input := range rowsArray {
			rowID := uuid.New().String()
			columns := []string{"id", "project_id"}
			placeholders := []string{"$1", "$2"}
			values := []interface{}{rowID, projectID}

			paramIndex := 3
			for col, val := range input {
				normalizedCol := strings.ReplaceAll(col, " ", "_")
				matchedCol := ""
				for validColName := range validCols {
					if strings.EqualFold(validColName, normalizedCol) {
						matchedCol = validColName
						break
					}
				}

				if matchedCol == "" || matchedCol == "id" || matchedCol == "project_id" || matchedCol == "created_at" || matchedCol == "updated_at" {
					continue
				}

				columns = append(columns, matchedCol)
				placeholders = append(placeholders, fmt.Sprintf("$%d", paramIndex))

				switch v := val.(type) {
				case map[string]interface{}, []interface{}:
					jsonBytes, _ := json.Marshal(v)
					values = append(values, string(jsonBytes))
				default:
					values = append(values, val)
				}
				paramIndex++
			}

			queryStr := fmt.Sprintf(
				"INSERT INTO %s (%s) VALUES (%s)",
				physicalTable,
				strings.Join(columns, ", "),
				strings.Join(placeholders, ", "),
			)
			_, err = tx.Exec(queryStr, values...)
			if err != nil {
				http.Error(w, fmt.Sprintf("Gagal menyimpan baris: %v", err), http.StatusBadRequest)
				return
			}
		}

		if err = tx.Commit(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, fmt.Sprintf("/dashboard/projects/%s/tables/%s", projectID, tableID), http.StatusSeeOther)
}

func handleDashboardLoginGet(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.New("login").Parse(loginHTML)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = tmpl.Execute(w, nil)
}

func handleDashboardLoginPost(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	adminUser := os.Getenv("ADMIN_USERNAME")
	adminPass := os.Getenv("ADMIN_PASSWORD")

	if adminUser == "" {
		adminUser = "admin"
		log.Println("⚠️  PERINGATAN: ADMIN_USERNAME tidak di-set, menggunakan default 'admin'. Set env ADMIN_USERNAME di production!")
	}
	if adminPass == "" {
		adminPass = "admin123"
		log.Println("⚠️  PERINGATAN: ADMIN_PASSWORD tidak di-set, menggunakan default 'admin123'. Set env ADMIN_PASSWORD di production!")
	}

	if username != adminUser || password != adminPass {
		tmpl, _ := template.New("login").Parse(loginHTML)
		_ = tmpl.Execute(w, map[string]interface{}{
			"Error": "Kombinasi username atau password salah.",
		})
		return
	}

	expectedToken := getSessionToken()
	http.SetCookie(w, &http.Cookie{
		Name:     "coderbase_session",
		Value:    expectedToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 jam
	})

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func handleDashboardLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "coderbase_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/dashboard/login", http.StatusSeeOther)
}
