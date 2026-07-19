<template>
  <div>
    <h1 class="page-title">设置</h1>
    <div class="detail-card">
      <h2 style="margin-bottom:16px">系统信息</h2>
      <div class="detail-grid">
        <div class="detail-item"><span class="label">版本</span><span>2.0.0</span></div>
        <div class="detail-item"><span class="label">WebSocket</span><span>已连接</span></div>
        <div class="detail-item"><span class="label">Go + gotd/td</span><span>MTProto</span></div>
        <div class="detail-item"><span class="label">数据库</span><span>SQLite (pure Go)</span></div>
      </div>
    </div>
    <div class="detail-card" style="margin-top:16px">
      <h2 style="margin-bottom:16px">配置说明</h2>
      <p style="color:#94a3b8;font-size:14px">
        启动参数: <code>tgcloud.exe --config config.json</code><br>
        环境变量: <code>TGCLOUD_API_ID</code>, <code>TGCLOUD_API_HASH</code>, <code>TGCLOUD_LISTEN</code><br>
        配置文件: <code>config.json</code> (api_id, api_hash, listen_addr, data_dir, admin_user, admin_pass)
      </p>
    </div>
  </div>
</template>