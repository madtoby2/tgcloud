<template>
  <div>
    <h1 class="page-title">仪表盘</h1>
    <div class="stats-grid">
      <div class="stat-card" v-for="s in stats" :key="s.label" :style="{ borderLeftColor: s.color }">
        <div class="stat-value">{{ s.value }}</div>
        <div class="stat-label">{{ s.label }}</div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, onMounted } from 'vue'
import { api } from '../api/client'

export default {
  setup() {
    const stats = ref([
      { value: 0, label: '总账号', color: '#6366f1' },
      { value: 0, label: '在线', color: '#22c55e' },
      { value: 0, label: '正常', color: '#3b82f6' },
      { value: 0, label: '封禁', color: '#ef4444' },
      { value: 0, label: '受限', color: '#f59e0b' },
      { value: 0, label: '冻结', color: '#8b5cf6' },
    ])

    onMounted(async () => {
      try {
        const d = await api.dashboard()
        stats.value[0].value = d.total
        stats.value[1].value = d.online
        stats.value[2].value = d.normal
        stats.value[3].value = d.banned
        stats.value[4].value = d.restricted
        stats.value[5].value = d.frozen
      } catch {}
    })

    return { stats }
  }
}
</script>

<style>
.page-title { font-size: 24px; font-weight: 700; margin-bottom: 24px; color: #f1f5f9 }
.stats-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 16px }
.stat-card {
  background: #1e293b; border-radius: 12px; padding: 20px;
  border-left: 4px solid #6366f1; transition: transform 0.15s;
}
.stat-card:hover { transform: translateY(-2px) }
.stat-value { font-size: 32px; font-weight: 700; color: #f1f5f9 }
.stat-label { font-size: 14px; color: #94a3b8; margin-top: 4px }
</style>
