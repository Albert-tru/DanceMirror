// Minimal frontend API client used by video-player.html
(function(window){
    const API_BASE = ''; // same origin
    function getToken() {
        return localStorage.getItem('dm_token') || '';
    }
    function authHeaders() {
        const t = getToken();
        return t ? { 'Authorization': 'Bearer ' + t } : {};
    }

    async function handleJSONResponse(res) {
        const txt = await res.text();
        try { return JSON.parse(txt || '{}'); } catch(e){ return txt; }
    }

    const DanceMirrorAPI = {
        isLoggedIn: function(){ return !!getToken(); },

        // GET /api/videos  -> returns [{ id, title, description, filePath, createdAt }, ...]
        getVideos: async function(){
            const paths = ['/api/videos', '/videos', '/api/v1/videos'];
            for (const p of paths) {
                try {
                    const res = await fetch(API_BASE + p, { headers: authHeaders() });
                    if (!res.ok) continue;
                    return await handleJSONResponse(res);
                } catch(e){ /* try next */ }
            }
            throw new Error('无法获取视频列表');
        },

        // uploadVideo(fileOrBlob, opts)
        // 支持： uploadVideo(file, {title, description})
        // 兼容旧签名： uploadVideo(title, description, file)
        uploadVideo: async function(arg1, arg2){
            let file = null, opts = {};
            // legacy: (title, description, file)
            if (typeof arg1 === 'string' && arguments.length >= 3) {
                opts.title = arg1;
                opts.description = arg2;
                file = arguments[2];
            } else if (arg1 instanceof File || arg1 instanceof Blob) {
                file = arg1;
                opts = arg2 || {};
            } else if (arg1 && arg1.file) {
                file = arg1.file;
                opts = arg2 || {};
            } else {
                throw new Error('参数错误：需提供 File/Blob');
            }
            if (!file) throw new Error('未提供文件');
            const form = new FormData();
            form.append('file', file, file.name || ('upload_' + Date.now() + '.webm'));
            if (opts.title) form.append('title', opts.title);
            if (opts.description) form.append('description', opts.description);

            const paths = ['/api/videos/upload', '/api/videos', '/upload/video'];
            let lastErr = null;
            for (const p of paths) {
                try {
                    const res = await fetch(API_BASE + p, {
                        method: 'POST',
                        headers: authHeaders(),
                        body: form
                    });
                    if (!res.ok) {
                        lastErr = new Error('上传失败: ' + res.status + ' ' + res.statusText);
                        // try next possible endpoint
                        continue;
                    }
                    return await handleJSONResponse(res);
                } catch (e) { lastErr = e; }
            }
            throw lastErr || new Error('上传失败');
        },

        // helper: download file by path
        downloadFile: function(path){ window.open(path, '_blank'); }
    };

    window.DanceMirrorAPI = DanceMirrorAPI;
})(window);