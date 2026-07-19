<template>
  <div class="app">
    <nav class="sidebar">
      <div class="brand" @click="$router.push('/')">
        <span class="logo">☁</span>
        <span class="name">tgcloud</span>
      </div>
      <div class="nav-items">
        <router-link to="/" class="nav-item"><span class="icon">📊</span> 仪表盘</router-link>
        <router-link to="/accounts" class="nav-item"><span class="icon">👤</span> 账号管理</router-link>
        <router-link to="/import" class="nav-item"><span class="icon">📥</span> 导入账号</router-link>
        <router-link to="/categories" class="nav-item"><span class="icon">📁</span> 分类管理</router-link>
        <router-link to="/channels" class="nav-item"><span class="icon">📢</span> 频道</router-link>
        <router-link to="/groups" class="nav-item"><span class="icon">💬</span> 群组</router-link>
        <router-link to="/operations" class="nav-item"><span class="icon">⚡</span> 操作面板</router-link>
        <router-link to="/batch-tasks" class="nav-item"><span class="icon">📦</span> 批量任务</router-link>
        <router-link to="/scheduled" class="nav-item"><span class="icon">🕐</span> 定时任务</router-link>
        <router-link to="/settings" class="nav-item"><span class="icon">⚙</span> 设置</router-link>
      </div>
      <div class="ws-status" :class="{ connected: wsConnected }">
        {{ wsConnected ? '🟢 已连接' : '🔴 断开' }}
      </div>
    </nav>
    <main class="content">
      <router-view />
    </main>
  </div>
</template>

<script>
import { ref, onMounted } from 'vue'
import { connectWS } from './api/client'

export default {
  setup() {
    const wsConnected = ref(false)
    onMounted(() => {
      const ws = connectWS(() => { wsConnected.value = true })
      ws.onopen = () => { wsConnected.value = true }
      ws.onclose = () => { wsConnected.value = false }
    })
    return { wsConnected }
  }
}
</script>

<style>
.app { display: flex; min-height: 100vh }
.sidebar {
  width: 220px; background: #1e293b; display: flex; flex-direction: column;
  padding: 16px 0; border-right: 1px solid #334155; position: sticky; top: 0; height: 100vh;
}
.brand {
  padding: 12px 20px; cursor: pointer; display: flex; align-items: center; gap: 10px;
  border-bottom: 1px solid #334155; margin-bottom: 8px;
}
.logo { font-size: 24px }
.name { font-size: 18px; font-weight: 700; color: #6366f1 }
.nav-items { flex: 1; overflow-y: auto }
.nav-item {
  display: flex; align-items: center; gap: 10px; padding: 10px 20px;
  color: #94a3b8; text-decoration: none; font-size: 14px; transition: all 0.15s;
  border-left: 3px solid transparent;
}
.nav-item:hover { background: #334155; color: #e2e8f0 }
.nav-item.router-link-active { background: #334155; color: #6366f1; border-left-color: #6366f1 }
.icon { font-size: 18px }
.ws-status { padding: 12px 20px; font-size: 12px; color: #ef4444; border-top: 1px solid #334155 }
.ws-status.connected { color: #22c55e }
.content { flex: 1; padding: 24px; overflow-y: auto }
</style>
