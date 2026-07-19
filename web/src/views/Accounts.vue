<template>
  <div>
    <div class="header-row">
      <h1 class="page-title">账号管理</h1>
      <div class="actions">
        <button class="btn btn-primary" @click="showAdd = true">+ 添加账号</button>
        <button class="btn btn-danger" @click="batchDeleteWaste">🗑 清除废号</button>
      </div>
    </div>

    <table class="table" v-if="accounts.length">
      <thead>
        <tr>
          <th>ID</th><th>手机号</th><th>用户名</th><th>状态</th><th>TG状态</th><th>代理</th><th>分类</th><th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="a in accounts" :key="a.id">
          <td>{{ a.id }}</td>
          <td>{{ a.phone }}</td>
          <td>{{ a.username || '-' }}</td>
          <td><span class="badge" :class="a.status">{{ a.status }}</span></td>
          <td>
            <span class="badge" :class="a.telegram_status" v-if="a.telegram_status">
              {{ a.telegram_status }}
            </span>
            <span v-else>-</span>
          </td>
          <td>{{ a.proxy || '-' }}</td>
          <td>{{ a.category_name || '-' }}</td>
          <td class="actions-col">
            <button class="btn-sm" @click="$router.push('/accounts/' + a.id)">详情</button>
            <button class="btn-sm" v-if="a.status !== 'online'" @click="doLogin(a.id)">登录</button>
            <button class="btn-sm btn-danger-sm" @click="doDelete(a.id)">删除</button>
          </td>
        </tr>
      </tbody>
    </table>
    <div v-else class="empty">暂无账号，点击"添加账号"开始</div>

    <!-- Add dialog -->
    <div class="modal-overlay" v-if="showAdd" @click.self="showAdd=false">
      <div class="modal">
        <h2>添加账号</h2>
        <input v-model="addForm.phone" placeholder="手机号 (+8613800138000)" class="input" />
        <input v-model="addForm.proxy" placeholder="代理 (可选)" class="input" />
        <div class="modal-actions">
          <button class="btn" @click="showAdd=false">取消</button>
          <button class="btn btn-primary" @click="doAdd">确定</button>
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
    const accounts = ref([])
    const showAdd = ref(false)
    const addForm = ref({ phone: '', proxy: '' })

    const load = async () => {
      try { const r = await api.listAccounts({ page_size: 100 }); accounts.value = r.items } catch {}
    }

    const doAdd = async () => {
      try { await api.addAccount(addForm.value); showAdd.value = false; addForm.value = { phone: '', proxy: '' }; load() } catch (e) { alert(e.message) }
    }

    const doLogin = async (id) => {
      try { await api.loginAccount(id); alert('验证码已发送') } catch (e) { alert(e.message) }
    }

    const doDelete = async (id) => {
      if (!confirm('确认删除?')) return
      try { await api.deleteAccount(id); load() } catch (e) { alert(e.message) }
    }

    const batchDeleteWaste = async () => {
      if (!confirm('确认清除所有废号(封禁/注销/失效)?')) return
      try { await api.batchDeleteWaste(); load() } catch (e) { alert(e.message) }
    }

    onMounted(load)
    return { accounts, showAdd, addForm, doAdd, doLogin, doDelete, batchDeleteWaste }
  }
}
</script>

<style>
.header-row { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px }
.page-title { font-size: 24px; font-weight: 700; color: #f1f5f9 }
.actions { display: flex; gap: 8px }
.table { width: 100%; border-collapse: collapse }
.table th { text-align: left; padding: 10px 12px; background: #1e293b; color: #94a3b8; font-size: 12px; text-transform: uppercase }
.table td { padding: 10px 12px; border-bottom: 1px solid #1e293b; font-size: 14px }
.table tr:hover { background: #1e293b }
.badge {
  padding: 2px 8px; border-radius: 4px; font-size: 12px; font-weight: 600;
}
.badge.online { background: #166534; color: #22c55e }
.badge.offline { background: #334155; color: #94a3b8 }
.badge.error { background: #7f1d1d; color: #ef4444 }
.badge.banned { background: #7f1d1d; color: #ef4444 }
.badge.restricted { background: #78350f; color: #f59e0b }
.badge.deactivated { background: #581c87; color: #a855f7 }
.badge.frozen { background: #1e3a5f; color: #3b82f6 }
.badge.ok { background: #166534; color: #22c55e }
.actions-col { display: flex; gap: 4px }
.btn { padding: 8px 16px; border: 1px solid #334155; border-radius: 8px; background: #1e293b; color: #e2e8f0; cursor: pointer; font-size: 14px }
.btn:hover { background: #334155 }
.btn-primary { background: #6366f1; border-color: #6366f1 }
.btn-primary:hover { background: #4f46e5 }
.btn-danger { background: #7f1d1d; border-color: #7f1d1d; color: #fca5a5 }
.btn-sm { padding: 4px 8px; font-size: 12px; border: 1px solid #334155; border-radius: 4px; background: #1e293b; color: #e2e8f0; cursor: pointer }
.btn-sm:hover { background: #334155 }
.btn-danger-sm { color: #fca5a5; border-color: #7f1d1d }
.input { width: 100%; padding: 10px; background: #0f172a; border: 1px solid #334155; border-radius: 8px; color: #e2e8f0; font-size: 14px; margin-bottom: 12px }
.empty { text-align: center; padding: 60px; color: #64748b; font-size: 16px }
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.6); display: flex; align-items: center; justify-content: center; z-index: 100 }
.modal { background: #1e293b; border-radius: 12px; padding: 24px; width: 400px; max-width: 90vw }
.modal h2 { margin-bottom: 16px; font-size: 18px }
.modal-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 16px }
</style>
