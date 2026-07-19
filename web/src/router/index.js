import { createRouter, createWebHashHistory } from 'vue-router'

const routes = [
  { path: '/', name: 'Dashboard', component: () => import('../views/Dashboard.vue') },
  { path: '/accounts', name: 'Accounts', component: () => import('../views/Accounts.vue') },
  { path: '/accounts/:id', name: 'AccountDetail', component: () => import('../views/AccountDetail.vue') },
  { path: '/import', name: 'AccountImport', component: () => import('../views/AccountImport.vue') },
  { path: '/categories', name: 'Categories', component: () => import('../views/Categories.vue') },
  { path: '/channels', name: 'Channels', component: () => import('../views/Channels.vue') },
  { path: '/groups', name: 'Groups', component: () => import('../views/Groups.vue') },
  { path: '/operations', name: 'Operations', component: () => import('../views/Operations.vue') },
  { path: '/batch-tasks', name: 'BatchTasks', component: () => import('../views/BatchTasks.vue') },
  { path: '/scheduled', name: 'ScheduledTasks', component: () => import('../views/ScheduledTasks.vue') },
  { path: '/settings', name: 'Settings', component: () => import('../views/Settings.vue') },
]

export default createRouter({ history: createWebHashHistory(), routes })
