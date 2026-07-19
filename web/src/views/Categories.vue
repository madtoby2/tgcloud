<template>
  <div>
    <div class="header-row">
      <h1 class="page-title">Categories</h1>
      <button class="btn btn-primary" @click="showAdd=true">+ New</button>
    </div>
    <div class="cat-list">
      <div v-for="c in cats" :key="c.id" class="cat-item">
        <span class="cat-dot" :style="{background:c.color}"></span>
        <span>{{ c.name }}</span>
        <button class="btn-sm btn-danger-sm" @click="doDelete(c.id)">Delete</button>
      </div>
    </div>
    <div class="modal-overlay" v-if="showAdd" @click.self="showAdd=false">
      <div class="modal">
        <h2>New Category</h2>
        <input v-model="form.name" placeholder="Name" class="input" />
        <input v-model="form.color" placeholder="Color (#ef4444)" class="input" />
        <div class="modal-actions">
          <button class="btn" @click="showAdd=false">Cancel</button>
          <button class="btn btn-primary" @click="doAdd">Create</button>
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
    const cats = ref([]); const showAdd = ref(false); const form = ref({ name:'', color:'#6366f1' })
    const load = async () => { try { cats.value = await api.listCategories() } catch {} }
    const doAdd = async () => { await api.createCategory(form.value); showAdd.value=false; load() }
    const doDelete = async (id) => { await api.deleteCategory(id); load() }
    onMounted(load)
    return { cats, showAdd, form, doAdd, doDelete }
  }
}
</script>
<style>
.cat-list { display:flex; flex-direction:column; gap:8px }
.cat-item { display:flex; align-items:center; gap:12px; padding:12px 16px; background:#1e293b; border-radius:8px }
.cat-dot { width:12px; height:12px; border-radius:50% }
</style>
