#!/bin/bash

# 创建新的 video-player.html
cat > video-player.html << 'HTMLEOF'
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>视频对比播放器 - DanceMirror</title>
    <link rel="stylesheet" href="/static/css/common.css">
    <link rel="stylesheet" href="/static/css/video-player.css">
    <style>
        .compare-container { display: flex; gap: 20px; margin: 20px 0; align-items: flex-start; }
        .video-box { flex: 1; display: flex; flex-direction: column; }
        .video-box h3 { margin: 0 0 10px 0; padding: 10px; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; border-radius: 8px; text-align: center; }
        .video-box video { width: 100%; background: #000; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.1); }
        .record-controls { margin-top: 12px; display: flex; gap: 8px; flex-wrap: wrap; align-items: center; }
        .record-timer { margin-left: 8px; padding: 6px 12px; background: #f0f0f0; border-radius: 4px; font-family: monospace; font-size: 14px; font-weight: bold; color: #e74c3c; }
        .sync-controls { background: #f8f9fa; padding: 15px; border-radius: 8px; margin: 15px 0; }
        .offset-control { display: flex; gap: 10px; align-items: center; margin-top: 10px; }
        .offset-control label { font-weight: 600; min-width: 200px; }
        .offset-control input[type="range"] { flex: 1; min-width: 200px; }
        .offset-control input[type="number"] { width: 80px; padding: 5px; border: 1px solid #ddd; border-radius: 4px; text-align: center; }
        .master-controls { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 10px; }
        @media (max-width: 768px) { .compare-container { flex-direction: column; } }
        .recording-indicator { display: inline-block; width: 10px; height: 10px; background: #e74c3c; border-radius: 50%; margin-right: 5px; animation: blink 1s infinite; }
        @keyframes blink { 0%, 50% { opacity: 1; } 51%, 100% { opacity: 0.3; } }
        .video-box video.mirrored { transform: scaleX(-1); }
    </style>
</head>
<body>
    <div class="container">
        <a href="/static/index.html" class="nav-link">← 返回首页</a>
        <h1>🎬 视频对比播放器（录制 & 同步）</h1>
        <p class="subtitle">左侧观看原视频，右侧录制你的动作，支持同步播放和偏移调整</p>
        <div class="message" id="message"></div>
        <div class="compare-container">
            <div class="video-box">
                <h3>📺 原视频（主控）</h3>
                <video id="originalVideo" controls crossorigin="anonymous"></video>
                <div class="video-info" style="margin-top: 10px;">
                    <h4 id="videoTitle" style="margin: 5px 0;">请选择视频</h4>
                    <p id="videoDescription" style="margin: 5px 0; color: #666; font-size: 13px;">暂无描述</p>
                </div>
            </div>
            <div class="video-box">
                <h3>🎥 你的录制</h3>
                <video id="userVideo" controls muted></video>
                <div class="record-controls">
                    <button id="startRecBtn" class="btn btn-success">🔴 开始录制</button>
                    <button id="stopRecBtn" class="btn btn-secondary" disabled>⏹️ 停止录制</button>
                    <button id="downloadRecBtn" class="btn" disabled>💾 下载</button>
                    <button id="uploadRecBtn" class="btn" disabled>☁️ 上传</button>
                    <span class="record-timer" id="recTimer">0:00</span>
                </div>
            </div>
        </div>
        <div class="sync-controls">
            <h3>🎛️ 同步控制</h3>
            <div class="master-controls">
                <button class="btn" id="masterPlay">▶️ 播放</button>
                <button class="btn" id="masterPause">⏸️ 暂停</button>
                <button class="btn" id="masterRestart">⏮️ 重新开始</button>
            </div>
            <div class="offset-control">
                <label>⏱️ 同步偏移（右侧视频延迟/提前秒数）：</label>
                <input id="offsetRange" type="range" min="-5" max="5" step="0.1" value="0" />
                <input id="offsetValue" type="number" step="0.1" value="0" />
                <span id="offsetLabel">0.0s</span>
            </div>
        </div>
        <div class="controls-panel">
            <div class="control-group">
                <h3>⏱️ 播放速度</h3>
                <div class="speed-buttons">
                    <button class="btn" onclick="setSpeed(0.5)">0.5x</button>
                    <button class="btn" onclick="setSpeed(0.75)">0.75x</button>
                    <button class="btn active" onclick="setSpeed(1.0)">1.0x</button>
                    <button class="btn" onclick="setSpeed(1.25)">1.25x</button>
                    <button class="btn" onclick="setSpeed(1.5)">1.5x</button>
                </div>
            </div>
            <div class="control-group">
                <h3>🪞 镜面翻转</h3>
                <button class="btn" onclick="toggleMirror('original')">翻转原视频</button>
                <button class="btn" onclick="toggleMirror('user')">翻转录制视频</button>
            </div>
            <div class="control-group">
                <h3>🔄 AB 循环（主控）</h3>
                <button class="btn btn-secondary" onclick="setPointA()">设置 A 点</button>
                <button class="btn btn-secondary" onclick="setPointB()">设置 B 点</button>
                <button class="btn btn-success" onclick="startABLoop()">开始循环</button>
                <button class="btn" onclick="clearABLoop()">清除循环</button>
                <div class="ab-loop-info" id="abInfo">A: 未设置 | B: 未设置</div>
            </div>
        </div>
        <h2>📹 视频列表</h2>
        <div class="video-list" id="videoList">
            <p style="text-align: center; color: #666;">加载中...</p>
        </div>
    </div>
    <script src="/static/js/utils.js"></script>
    <script src="/static/js/api.js"></script>
HTMLEOF

# 追加JavaScript代码（第1部分）
cat >> video-player.html << 'JSEOF'
    <script>
        const originalVideo = document.getElementById('originalVideo');
        const userVideo = document.getElementById('userVideo');
        const offsetRange = document.getElementById('offsetRange');
        const offsetValue = document.getElementById('offsetValue');
        const offsetLabel = document.getElementById('offsetLabel');
        const recTimer = document.getElementById('recTimer');
        
        let syncOffset = 0;
        let pointA = null;
        let pointB = null;
        let currentVideoId = null;
        let mediaRecorder = null;
        let recordedChunks = [];
        let recStartTime = 0;
        let recTimerInterval = null;
        let recordedBlob = null;
        
        window.onload = function() {
            if (!DanceMirrorAPI.isLoggedIn()) {
                Utils.showMessage('message', '请先登录！正在跳转...', 'error');
                setTimeout(() => window.location.href = '/static/index.html', 2000);
                return;
            }
            initSyncControls();
            initRecordingControls();
            loadVideos();
        };
        
        function initSyncControls() {
            offsetRange.addEventListener('input', (e) => {
                syncOffset = parseFloat(e.target.value);
                offsetValue.value = syncOffset;
                offsetLabel.textContent = syncOffset.toFixed(1) + 's';
            });
            offsetValue.addEventListener('change', (e) => {
                syncOffset = parseFloat(e.target.value) || 0;
                offsetRange.value = syncOffset;
                offsetLabel.textContent = syncOffset.toFixed(1) + 's';
            });
            document.getElementById('masterPlay').addEventListener('click', () => { originalVideo.play(); });
            document.getElementById('masterPause').addEventListener('click', () => { originalVideo.pause(); });
            document.getElementById('masterRestart').addEventListener('click', () => {
                originalVideo.currentTime = 0;
                syncUserVideoTime();
            });
            originalVideo.addEventListener('play', () => {
                if (userVideo.src && !userVideo.paused) return;
                try { userVideo.play(); } catch (e) { console.log('用户视频播放失败:', e); }
            });
            originalVideo.addEventListener('pause', () => { try { userVideo.pause(); } catch (e) {} });
            originalVideo.addEventListener('seeked', () => { syncUserVideoTime(); });
            originalVideo.addEventListener('timeupdate', () => { syncUserVideoTime(); });
        }
        
        function syncUserVideoTime() {
            if (!userVideo.src || userVideo.readyState < 2) return;
            const targetTime = originalVideo.currentTime + syncOffset;
            const clampedTime = Math.max(0, Math.min(userVideo.duration || Infinity, targetTime));
            if (Math.abs(userVideo.currentTime - clampedTime) > 0.2) {
                try { userVideo.currentTime = clampedTime; } catch (e) { console.log('同步失败:', e); }
            }
        }
        
        function initRecordingControls() {
            document.getElementById('startRecBtn').addEventListener('click', startRecording);
            document.getElementById('stopRecBtn').addEventListener('click', stopRecording);
            document.getElementById('downloadRecBtn').addEventListener('click', downloadRecording);
            document.getElementById('uploadRecBtn').addEventListener('click', uploadRecording);
        }
        
        async function startRecording() {
            try {
                const stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
                recordedChunks = [];
                const options = { mimeType: 'video/webm;codecs=vp8,opus' };
                if (!MediaRecorder.isTypeSupported(options.mimeType)) { options.mimeType = 'video/webm'; }
                mediaRecorder = new MediaRecorder(stream, options);
                mediaRecorder.ondataavailable = (e) => {
                    if (e.data && e.data.size > 0) { recordedChunks.push(e.data); }
                };
                mediaRecorder.onstop = () => {
                    stream.getTracks().forEach(track => track.stop());
                    recordedBlob = new Blob(recordedChunks, { type: 'video/webm' });
                    const url = URL.createObjectURL(recordedBlob);
                    userVideo.srcObject = null;
                    userVideo.src = url;
                    userVideo.muted = false;
                    userVideo.load();
                    document.getElementById('downloadRecBtn').disabled = false;
                    document.getElementById('uploadRecBtn').disabled = false;
                    Utils.showMessage('message', '✅ 录制完成！可以播放、下载或上传', 'success', 3000);
                };
                mediaRecorder.start();
                recStartTime = Date.now();
                startRecordingTimer();
                userVideo.srcObject = stream;
                userVideo.muted = true;
                userVideo.play();
                document.getElementById('startRecBtn').disabled = true;
                document.getElementById('startRecBtn').innerHTML = '<span class="recording-indicator"></span> 录制中...';
                document.getElementById('stopRecBtn').disabled = false;
                Utils.showMessage('message', '🔴 录制已开始！', 'success', 2000);
            } catch (err) {
                Utils.showMessage('message', '❌ 无法访问摄像头/麦克风: ' + err.message, 'error', 4000);
                console.error('录制错误:', err);
            }
        }
        
        function stopRecording() {
            if (mediaRecorder && mediaRecorder.state !== 'inactive') { mediaRecorder.stop(); }
            stopRecordingTimer();
            document.getElementById('startRecBtn').disabled = false;
            document.getElementById('startRecBtn').innerHTML = '🔴 开始录制';
            document.getElementById('stopRecBtn').disabled = true;
        }
        
        function startRecordingTimer() {
            recTimer.textContent = '0:00';
            recTimerInterval = setInterval(() => {
                const elapsed = Math.floor((Date.now() - recStartTime) / 1000);
                const mins = Math.floor(elapsed / 60);
                const secs = elapsed % 60;
                recTimer.textContent = mins + ':' + String(secs).padStart(2, '0');
            }, 500);
        }
        
        function stopRecordingTimer() {
            if (recTimerInterval) { clearInterval(recTimerInterval); recTimerInterval = null; }
        }
        
        function downloadRecording() {
            if (!recordedBlob) { Utils.showMessage('message', '没有可下载的录制', 'error', 2000); return; }
            const url = URL.createObjectURL(recordedBlob);
            const a = document.createElement('a');
            a.href = url;
            a.download = 'recording_' + Date.now() + '.webm';
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
            Utils.showMessage('message', '💾 下载已开始', 'success', 2000);
        }
        
        async function uploadRecording() {
            if (!recordedBlob) { Utils.showMessage('message', '没有可上传的录制', 'error', 2000); return; }
            try {
                Utils.showMessage('message', '⏳ 正在上传...', 'info');
                const file = new File([recordedBlob], 'recording_' + Date.now() + '.webm', { type: 'video/webm' });
                const response = await DanceMirrorAPI.uploadVideo(file, {
                    title: '录制_' + new Date().toLocaleString('zh-CN'),
                    description: '用户录制对比视频'
                });
                Utils.showMessage('message', '✅ 上传成功！', 'success', 3000);
                setTimeout(() => loadVideos(), 1000);
            } catch (err) {
                Utils.showMessage('message', '❌ 上传失败: ' + err.message, 'error', 4000);
                console.error('上传错误:', err);
            }
        }
        
        async function loadVideos() {
            const listEl = document.getElementById('videoList');
            try {
                const videos = await DanceMirrorAPI.getVideos();
                if (videos && videos.length > 0) {
                    listEl.innerHTML = '';
                    videos.forEach((video, index) => {
                        const item = document.createElement('div');
                        item.className = 'video-item';
                        item.innerHTML = '<h4>' + Utils.escapeHtml(video.title) + '</h4>' +
                            '<p>' + Utils.escapeHtml(video.description || '暂无描述') + '</p>' +
                            '<p style="font-size: 11px; color: #999; margin-top: 5px;">' +
                            Utils.formatDate(video.createdAt) + '</p>';
                        item.onclick = () => selectVideo(video, item);
                        listEl.appendChild(item);
                        if (index === 0) { selectVideo(video, item); }
                    });
                } else {
                    listEl.innerHTML = '<p style="text-align:center;color:#666;">暂无视频</p>';
                }
            } catch (error) {
                Utils.showMessage('message', '加载失败: ' + error.message, 'error');
                listEl.innerHTML = '<p style="text-align:center;color:#666;">加载失败</p>';
            }
        }
        
        function selectVideo(video, itemElement) {
            currentVideoId = video.id;
            originalVideo.src = '/' + video.filePath;
            originalVideo.load();
            document.getElementById('videoTitle').textContent = video.title;
            document.getElementById('videoDescription').textContent = video.description || '暂无描述';
            document.querySelectorAll('.video-item').forEach(item => { item.classList.remove('active'); });
            itemElement.classList.add('active');
            clearABLoop();
        }
        
        function setSpeed(speed) {
            originalVideo.playbackRate = speed;
            userVideo.playbackRate = speed;
            document.querySelectorAll('.speed-buttons .btn').forEach(btn => { btn.classList.remove('active'); });
            event.target.classList.add('active');
            Utils.showMessage('message', '播放速度: ' + speed + 'x', 'success', 2000);
        }
        
        function toggleMirror(target) {
            const video = target === 'original' ? originalVideo : userVideo;
            video.classList.toggle('mirrored');
            const isMirrored = video.classList.contains('mirrored');
            const name = target === 'original' ? '原视频' : '录制视频';
            Utils.showMessage('message', name + '镜像: ' + (isMirrored ? '✅ 开启' : '❌ 关闭'), 'success', 2000);
        }
        
        function setPointA() {
            pointA = originalVideo.currentTime;
            updateABInfo();
            Utils.showMessage('message', 'A 点已设置: ' + Utils.formatTime(pointA), 'success', 2000);
        }
        
        function setPointB() {
            pointB = originalVideo.currentTime;
            updateABInfo();
            Utils.showMessage('message', 'B 点已设置: ' + Utils.formatTime(pointB), 'success', 2000);
        }
        
        function startABLoop() {
            if (pointA === null || pointB === null) {
                Utils.showMessage('message', '请先设置 A 点和 B 点', 'error', 3000);
                return;
            }
            if (pointA >= pointB) {
                Utils.showMessage('message', 'A 点必须在 B 点之前', 'error', 3000);
                return;
            }
            originalVideo.removeEventListener('timeupdate', handleABLoop);
            originalVideo.currentTime = pointA;
            originalVideo.play();
            originalVideo.addEventListener('timeupdate', handleABLoop);
            Utils.showMessage('message', '✅ AB 循环已开始', 'success', 2000);
        }
        
        function handleABLoop() {
            if (originalVideo.currentTime >= pointB) {
                originalVideo.currentTime = pointA;
            }
        }
        
        function clearABLoop() {
            pointA = null;
            pointB = null;
            originalVideo.removeEventListener('timeupdate', handleABLoop);
            updateABInfo();
            Utils.showMessage('message', 'AB 循环已清除', 'success', 2000);
        }
        
        function updateABInfo() {
            const info = document.getElementById('abInfo');
            const aText = pointA !== null ? Utils.formatTime(pointA) : '未设置';
            const bText = pointB !== null ? Utils.formatTime(pointB) : '未设置';
            info.textContent = 'A: ' + aText + ' | B: ' + bText;
        }
        
        document.addEventListener('keydown', (e) => {
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;
            switch(e.code) {
                case 'Space':
                    e.preventDefault();
                    if (originalVideo.paused) originalVideo.play();
                    else originalVideo.pause();
                    break;
                case 'ArrowLeft':
                    e.preventDefault();
                    originalVideo.currentTime = Math.max(0, originalVideo.currentTime - 5);
                    break;
                case 'ArrowRight':
                    e.preventDefault();
                    originalVideo.currentTime = Math.min(originalVideo.duration, originalVideo.currentTime + 5);
                    break;
                case 'KeyM':
                    e.preventDefault();
                    toggleMirror('original');
                    break;
                case 'KeyA':
                    e.preventDefault();
                    setPointA();
                    break;
                case 'KeyB':
                    e.preventDefault();
                    setPointB();
                    break;
                case 'KeyL':
                    e.preventDefault();
                    startABLoop();
                    break;
            }
        });
    </script>
</body>
</html>
JSEOF

echo "✅ video-player.html 更新完成！"
echo "备份文件: video-player.html.bak"
wc -l video-player.html

