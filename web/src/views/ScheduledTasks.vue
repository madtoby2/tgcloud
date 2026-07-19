<template>
  <div>
    <div class="header-row">
      <h1 class="page-title">Scheduled Tasks</h1>
      <button class="btn btn-primary" @click="showCreate=true">+ New</button>
    </div>
    <table class="table" v-if="tasks.length">
      <thead><tr><th>Name</th><th>Type</th><th>Cron</th><th>Enabled</th><th>Actions</th></tr></thead>
      <tbody>
        <tr v-for="t in tasks" :key="t.id">
          <td>{{ t.name }}</td><td>{{ t.task_type }}</td><td><code>{{ t.cron_expr }}</code></td>
          <td><span :style="{color:t.is_enabled?'#22c55e':'#64748b'}">{{ t.is_enabled ? 'Yes' : 'No' }}</span></td>
          <td>
            <button class="btn-sm" @click="doToggle(t.id)">{{ t.is_enabled ? 'Disable' : 'Enable' }}</button>
            <button class="btn-sm btn-danger-sm" @click="doDelete(t.id)">Delete</button>
          </td>
        </tr>
      </tbody>
    </table>
    <div v-else class="empty">No scheduled tasks</div>
    <div class="modal-overlay" v-if="showCreate" @click.self="showCreate=false">
      <div class="modal" style="width:500px">
        <h2>New Scheduled Task</h2>
        <input v-model="form.name" placeholder="Name" class="input" />
        <select v-model="form.task_type" class="input"><option value="status_check">Status Check</option><option value="send_message">Send</option><option value="join_group">Join</option></select>
        <input v-model="form.cron_expr" placeholder="Cron (e.g. 0 8 * * *)" class="input" />
        <textarea v-model="form.account_ids" placeholder="Account IDs" class="input" rows="3"></textarea>
        <textarea v-model="form.config" placeholder="JSON config" class="input" rows="4"></textarea>
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
    const form = ref({ name:'', task_type:'status_check', cron_expr:'0 8 * * *', account_ids:'', config:'{}' })
    const load = async () => { try { tasks.value = await api.listScheduledTasks() } catch {} }
    const doCreate = async () => {
      const ids = form.value.account_ids.split(',').map(s=>parseInt(s.trim())).filter(n=>n)
      await api.createScheduledTask({...form.value, account_ids:ids, config:JSON.parse(form.value.config)})
      showCreate.value=false; load()
    }
    const doToggle = async (id) => { await api.toggleScheduledTask(id); load() }
    const doDelete = async (id) => { await api.deleteScheduledTask(id); load() }
    onMounted(load)
    return { tasks, showCreate, form, doCreate, doToggle, doDelete }
  }
}
</script>
