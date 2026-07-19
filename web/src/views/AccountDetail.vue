<template>
  <div>
    <div class="header-row">
      <button class="btn" @click="$router.back()">&larr; Back</button>
      <h1 class="page-title">Account Detail</h1>
    </div>
    <div v-if="account" class="detail-card">
      <div class="detail-grid">
        <div class="detail-item"><span class="label">ID</span><span>{{ account.id }}</span></div>
        <div class="detail-item"><span class="label">Phone</span><span>{{ account.phone }}</span></div>
        <div class="detail-item"><span class="label">Username</span><span>{{ account.username || '-' }}</span></div>
        <div class="detail-item"><span class="label">Name</span><span>{{ account.first_name }} {{ account.last_name }}</span></div>
        <div class="detail-item"><span class="label">Status</span><span class="badge" :class="account.status">{{ account.status }}</span></div>
        <div class="detail-item"><span class="label">TG Status</span><span>{{ account.telegram_status || 'N/A' }}</span></div>
        <div class="detail-item"><span class="label">Proxy</span><span>{{ account.proxy || '-' }}</span></div>
        <div class="detail-item"><span class="label">Remark</span><span>{{ account.remark || '-' }}</span></div>
      </div>
      <div class="actions" style="margin-top:20px">
        <button class="btn btn-primary" v-if="account.status!=='online'" @click="doLogin">Login</button>
        <button class="btn" @click="doCheckStatus">Check Status</button>
        <button class="btn" @click="doEstimateReg">Estimate Reg</button>
      </div>
    </div>
    <div v-else class="empty">Loading...</div>
  </div>
</template>
<script>
import { ref, onMounted } from 'vue'
import { api } from '../api/client'
export default {
  props: ['id'],
  setup(props) {
    const account = ref(null)
    const load = async () => { try { account.value = await api.getAccount(props.id) } catch {} }
    const doLogin = async () => { await api.loginAccount(props.id); alert('Verification code sent') }
    const doCheckStatus = async () => { await api.checkStatus(props.id); alert('Check started'); load() }
    const doEstimateReg = async () => { alert('WIP') }
    onMounted(load)
    return { account, doLogin, doCheckStatus, doEstimateReg }
  }
}
</script>
<style>
.detail-card { background:#1e293b; border-radius:12px; padding:24px }
.detail-grid { display:grid; grid-template-columns:1fr 1fr; gap:16px }
.detail-item { display:flex; flex-direction:column; gap:4px }
.label { font-size:12px; color:#64748b; text-transform:uppercase }
.header-row { display:flex; align-items:center; gap:16px; margin-bottom:20px }
</style>
