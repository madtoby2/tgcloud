<template>
  <div>
    <h1 class="page-title">Import Accounts</h1>
    <div class="import-methods">
      <div class="import-card">
        <h2>Telethon Session</h2>
        <p>Upload .session file (Base64 encoded)</p>
        <textarea v-model="telethonData" placeholder="Paste base64 session data..." class="input" rows="8"></textarea>
        <input v-model="phoneNum" placeholder="Phone number" class="input" />
        <button class="btn btn-primary" @click="importTelethon">Import</button>
      </div>
      <div class="import-card">
        <h2>TData</h2>
        <p>Upload tdata archive (Base64 encoded)</p>
        <textarea v-model="tdataData" placeholder="Paste base64 tdata..." class="input" rows="8"></textarea>
        <input v-model="phoneNum2" placeholder="Phone number" class="input" />
        <button class="btn btn-primary" @click="importTdata">Import</button>
      </div>
    </div>
  </div>
</template>
<script>
import { ref } from 'vue'
import { api } from '../api/client'
export default {
  setup() {
    const telethonData = ref(''); const tdataData = ref('')
    const phoneNum = ref(''); const phoneNum2 = ref('')
    const importTelethon = async () => {
      try { await api.importTelethon({ phone: phoneNum.value, session: telethonData.value }); alert('Imported') } catch(e) { alert(e.message) }
    }
    const importTdata = async () => {
      try { await api.importTData({ phone: phoneNum2.value, data: tdataData.value }); alert('Imported') } catch(e) { alert(e.message) }
    }
    return { telethonData, tdataData, phoneNum, phoneNum2, importTelethon, importTdata }
  }
}
</script>
<style>
.import-methods { display:grid; grid-template-columns:1fr 1fr; gap:24px }
.import-card { background:#1e293b; border-radius:12px; padding:24px }
.import-card h2 { margin-bottom:8px; font-size:18px }
.import-card p { color:#64748b; font-size:14px; margin-bottom:16px }
</style>
