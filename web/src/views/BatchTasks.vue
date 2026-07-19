<template>
  <div>
    <div class="header-row">
      <h1 class="page-title">Batch Tasks</h1>
      <button class="btn btn-primary" @click="showCreate=true">+ New</button>
    </div>
    <table class="table" v-if="tasks.length">
      <thead><tr><th>ID</th><th>Type</th><th>Status</th><th>Progress</th><th>Time</th><th>Actions</th></tr></thead>
      <tbody>
        <tr v-for="t in tasks" :key="t.id">
          <td>{{ t.id }}</td><td>{{ t.task_type }}</td>
          <td><span class="badge" :class="t.status">{{ t.status }}</span></td>
          <td>{{ t.completed }}/{{ t.total }} (fail:{{ t.failed }})</td>
          <td>{{ t.created_at }}</td>
          <td>
            <button class="btn-sm" v-if="t.status==='pending'" @click="doStart(t.id)">Start</button>
            <button class="btn-sm" v-if="t.status==='running'" @click="doPause(t.id)">Pause</button>
            <button class="btn-sm btn-danger-sm" @click="doCancel(t.id)">Cancel</button>
          </td>
        </tr>
      </tbody>
    </table>
    <div v-else class="empty">No batch tasks</div>
    <div class="modal-overlay" v-if="showCreate" @click.self="showCreate=false">
      <div class="modal" style="width:500px">
        <h2>New Batch Task</h2>
        <select v-model="form.task_type" class="input">
          <option value="">Select type</option>
          <option value="send_message">Send Message</option>
          <option value="join_group">Join Groups</option>
          <option value="invite_users">Invite Users</option>
          <option value="status_check">Status Check</option>
        </select>
        <textarea v-model="form.account_ids" placeholder="Account IDs (comma: 1,2,3)" class="input" rows="3"></textarea>
        <textarea v-model="form.config" placeholder="JSON config" class="input" rows="6"></textarea>
        <div class="modal-actions">
          <button class="btn" @click="showCreate=false">Cancel</button>
          <button class="btn btn-primary" @click="doCreate">Create</button>
        </div>
      </div>
    </div>
  </div>
</template>
<script>
import { ref, onMounted } from 'vue'
import { api } from '../api/client'
export default {
  setup() {
    const tasks = ref([]); const showCreate = ref(false)
    const form = ref({ task_type:'', account_ids:'', config:'{}' })
    const load = async () => { try { tasks.value = await api.listBatchTasks() } catch {} }
    const doCreate = async () => {
      const ids = form.value.account_ids.split(',').map(s=>parseInt(s.trim())).filter(n=>n)
      await api.createBatchTask({ task_type: form.value.task_type, account_ids: ids, config: JSON.parse(form.value.config) })
      showCreate.value = false; load()
    }
    const doStart = async (id) => { await api.startBatchTask(id); load() }
    const doPause = async (id) => { await api.pauseBatchTask(id); load() }
    const doCancel = async (id) => { await api.cancelBatchTask(id); load() }
    onMounted(load)
    return { tasks, showCreate, form, doCreate, doStart, doPause, doCancel }
  }
}
</script>
<style>
.badge.pending { background:#334155; color:#94a3b8 }
.badge.running { background:#1e3a5f; color:#3b82f6 }
.badge.completed { background:#166534; color:#22c55e }
.badge.failed { background:#7f1d1d; color:#ef4444 }
.badge.canceled { background:#334155; color:#64748b }
</style>
