<template>
  <div>
    <div class="header-row">
      <h1 class="page-title">操作面板</h1>
      <button class="btn btn-primary" @click="showForm=true">+ 新建操作</button>
    </div>
    <p class="empty">选择一个账号，执行操作: 群发消息、加群、拉人、炒群、爬成员、手机过滤、搜索群组</p>
    <div class="modal-overlay" v-if="showForm" @click.self="showForm=false">
      <div class="modal" style="width:500px">
        <h2>新建操作</h2>
        <select v-model="form.type" class="input">
          <option value="">选择操作类型</option>
          <option value="send_message">群发消息</option>
          <option value="join_group">批量加群</option>
          <option value="invite_users">批量拉人</option>
          <option value="farming">养号炒群</option>
          <option value="scrape_members">爬成员</option>
          <option value="phone_filter">手机过滤</option>
          <option value="search_groups">搜索群组</option>
        </select>
        <input v-model.number="form.account_id" placeholder="账号ID" class="input" type="number" />
        <textarea v-model="form.params" placeholder='JSON参数 (如: {"targets":["@user1"],"message":"hello"})' class="input" rows="6"></textarea>
        <div class="modal-actions">
          <button class="btn" @click="showForm=false">取消</button>
          <button class="btn btn-primary" @click="doCreate">执行</button>
        </div>
      </div>
    </div>
  </div>
</template>
<script>
import { ref } from 'vue'
import { api } from '../api/client'
export default {
  setup() {
    const showForm = ref(false)
    const form = ref({ type: '', account_id: 0, params: '{}' })
    const doCreate = async () => {
      try {
        await api.createOperation({
          account_id: form.value.account_id,
          type: form.value.type,
          params: JSON.parse(form.value.params),
        })
        showForm.value = false; alert('操作已创建')
      } catch(e) { alert(e.message) }
    }
    return { showForm, form, doCreate }
  }
}
</script>