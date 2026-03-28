(function () {
  const token = localStorage.getItem('token');
  if (!token) {
    alert('请先登录');
    location.href = '/static/index.html';
    return;
  }

  const headers = { Authorization: `Bearer ${token}` };

  async function api(path) {
    const res = await fetch(`/api/v1${path}`, { headers });
    if (!res.ok) {
      const txt = await res.text();
      throw new Error(txt || `HTTP ${res.status}`);
    }
    return res.json();
  }

  function renderTable(el, columns, rows) {
    const thead = `<thead><tr>${columns.map(c => `<th>${c.label}</th>`).join('')}</tr></thead>`;
    const tbody = `<tbody>${(rows || []).map(row => `<tr>${columns.map(c => `<td>${row[c.key] ?? ''}</td>`).join('')}</tr>`).join('')}</tbody>`;
    el.innerHTML = thead + tbody;
  }

  function switchTab(tab) {
    document.querySelectorAll('.tab').forEach(btn => btn.classList.toggle('active', btn.dataset.tab === tab));
    ['users', 'video', 'ai', 'trouble'].forEach(name => {
      const panel = document.getElementById(`panel-${name}`);
      panel.style.display = name === tab ? 'block' : 'none';
    });
  }

  document.querySelectorAll('.tab').forEach(btn => {
    btn.addEventListener('click', () => switchTab(btn.dataset.tab));
  });

  async function loadUsers() {
    const rows = await api('/admin/users/overview');
    renderTable(document.getElementById('usersTable'), [
      { key: 'userId', label: '用户ID' },
      { key: 'phone', label: '手机号' },
      { key: 'nickname', label: '昵称' },
      { key: 'createdAt', label: '注册时间' },
      { key: 'lastLoginAt', label: '最后登录' },
      { key: 'accountStatus', label: '账号状态' },
      { key: 'uploadVideoCount', label: '上传视频数' }
    ], rows);
  }

  async function loadVideoStats() {
    const rows = await api('/admin/stats/videos');
    renderTable(document.getElementById('videoTable'), [
      { key: 'userId', label: '用户ID' },
      { key: 'phone', label: '手机号' },
      { key: 'nickname', label: '昵称' },
      { key: 'totalUploadCount', label: '总上传量' },
      { key: 'latestUploadAt', label: '最近上传时间' },
      { key: 'totalFileSizeMB', label: '累计文件大小(MB)' },
      { key: 'failedUploadCount', label: '失败上传次数' },
      { key: 'cropRequestCount', label: '发起裁剪请求次数' }
    ], rows);
  }

  async function loadAIStats() {
    const rows = await api('/admin/stats/ai');
    renderTable(document.getElementById('aiTable'), [
      { key: 'userId', label: '用户ID' },
      { key: 'phone', label: '手机号' },
      { key: 'nickname', label: '昵称' },
      { key: 'launchCount', label: '发起次数' },
      { key: 'failCount', label: '失败次数' },
      { key: 'failRate', label: '失败率(%)' },
      { key: 'lastStatus', label: '最近状态' },
      { key: 'lastError', label: '最近错误' },
      { key: 'lastUpdatedAt', label: '最近更新时间' }
    ], rows);
  }

  async function loadErrors() {
    const requestId = document.getElementById('qRequestId').value.trim();
    const userId = document.getElementById('qUserId').value.trim();
    const videoId = document.getElementById('qVideoId').value.trim();
    const hours = document.getElementById('qHours').value;

    const qs = new URLSearchParams();
    if (requestId) qs.set('requestId', requestId);
    if (userId) qs.set('userId', userId);
    if (videoId) qs.set('videoId', videoId);
    qs.set('hours', hours);

    const rows = await api(`/admin/troubleshoot/errors?${qs.toString()}`);
    renderTable(document.getElementById('errorTable'), [
      { key: 'id', label: 'ID' },
      { key: 'requestId', label: 'Request ID' },
      { key: 'userId', label: '用户ID' },
      { key: 'videoId', label: '视频ID' },
      { key: 'method', label: '方法' },
      { key: 'path', label: '路径' },
      { key: 'statusCode', label: '状态码' },
      { key: 'errorCode', label: '错误码' },
      { key: 'message', label: '消息' },
      { key: 'createdAt', label: '时间' }
    ], rows);
  }

  async function loadFailedTasks() {
    const rows = await api('/admin/troubleshoot/failed-tasks?limit=100');
    renderTable(document.getElementById('failedTable'), [
      { key: 'id', label: 'ID' },
      { key: 'userId', label: '用户ID' },
      { key: 'videoId', label: '视频ID' },
      { key: 'status', label: '状态' },
      { key: 'durationMs', label: '耗时(ms)' },
      { key: 'errorReason', label: '错误原因' },
      { key: 'requestId', label: 'Request ID' },
      { key: 'updatedAt', label: '更新时间' }
    ], rows);
  }

  document.getElementById('refreshUsers').onclick = () => loadUsers().catch(err => alert(err.message));
  document.getElementById('refreshVideo').onclick = () => loadVideoStats().catch(err => alert(err.message));
  document.getElementById('refreshAI').onclick = () => loadAIStats().catch(err => alert(err.message));
  document.getElementById('searchErrors').onclick = () => loadErrors().catch(err => alert(err.message));
  document.getElementById('refreshFailed').onclick = () => loadFailedTasks().catch(err => alert(err.message));

  Promise.all([loadUsers(), loadVideoStats(), loadAIStats(), loadErrors(), loadFailedTasks()]).catch(err => {
    alert(`后台加载失败: ${err.message}`);
  });
})();
