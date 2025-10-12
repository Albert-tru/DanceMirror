const DanceMirrorAPI = (function() {
    const config = {
        apiBase: window.location.origin + '/api/v1',
        tokenKey: 'dancemirror_token',
        userKey: 'dancemirror_user'
    };

    function getToken() {
        return localStorage.getItem(config.tokenKey);
    }

    function setToken(token) {
        localStorage.setItem(config.tokenKey, token);
    }

    function clearToken() {
        localStorage.removeItem(config.tokenKey);
        localStorage.removeItem(config.userKey);
    }

    function getCurrentUser() {
        const userStr = localStorage.getItem(config.userKey);
        return userStr ? JSON.parse(userStr) : null;
    }

    function setCurrentUser(user) {
        localStorage.setItem(config.userKey, JSON.stringify(user));
    }

    async function request(endpoint, options = {}) {
        const url = `${config.apiBase}${endpoint}`;
        const token = getToken();
        
        const headers = { ...options.headers };
        if (token && !options.skipAuth) {
            headers['Authorization'] = `Bearer ${token}`;
        }
        
        // 如果有 body 且不是 FormData，添加 Content-Type
        if (options.body && !(options.body instanceof FormData)) {
            headers['Content-Type'] = 'application/json';
        }
        
        const fetchConfig = {
            method: options.method || 'GET',
            headers,
            body: options.body
        };

        try {
            console.log('发起请求:', url, fetchConfig.method);
            const response = await fetch(url, fetchConfig);
            
            // 处理空响应
            const responseText = await response.text();
            let data = null;
            try {
                data = responseText ? JSON.parse(responseText) : {};
            } catch (e) {
                console.error('JSON 解析失败:', responseText);
                data = { error: '服务器响应格式错误' };
            }
            
            if (!response.ok) {
                const errorMsg = data.error || data.message || responseText || `请求失败 (${response.status})`;
                console.error('请求失败:', errorMsg);
                throw new Error(errorMsg);
            }
            
            return data;
        } catch (error) {
            console.error('API 请求错误:', error);
            // 如果是网络错误，提供更友好的提示
            if (error.message === 'Failed to fetch' || error.message === 'Load failed') {
                throw new Error('网络连接失败，请检查网络后重试');
            }
            throw error;
        }
    }

    // 带进度的上传函数
    async function uploadWithProgress(endpoint, formData, onProgress) {
        const url = `${config.apiBase}${endpoint}`;
        const token = getToken();
        
        return new Promise((resolve, reject) => {
            const xhr = new XMLHttpRequest();
            
            xhr.upload.addEventListener('progress', (e) => {
                if (e.lengthComputable && onProgress) {
                    const percent = Math.round((e.loaded / e.total) * 100);
                    onProgress(percent);
                }
            });
            
            xhr.addEventListener('load', () => {
                if (xhr.status >= 200 && xhr.status < 300) {
                    try {
                        const data = JSON.parse(xhr.responseText);
                        resolve(data);
                    } catch (e) {
                        resolve({ success: true });
                    }
                } else {
                    try {
                        const error = JSON.parse(xhr.responseText);
                        reject(new Error(error.error || error.message || '上传失败'));
                    } catch (e) {
                        reject(new Error(`上传失败 (${xhr.status})`));
                    }
                }
            });
            
            xhr.addEventListener('error', () => {
                reject(new Error('网络错误，上传失败'));
            });
            
            xhr.open('POST', url);
            if (token) {
                xhr.setRequestHeader('Authorization', `Bearer ${token}`);
            }
            xhr.send(formData);
        });
    }

    return {
        async register(phone, firstName, lastName, password) {
            const data = await request('/register', {
                method: 'POST',
                body: JSON.stringify({ phone, firstName, lastName, password }),
                skipAuth: true
            });
            
            if (data.token) {
                setToken(data.token);
                setCurrentUser({ phone, firstName, lastName });
            }
            
            return data;
        },

        async login(phone, password) {
            const data = await request('/login', {
                method: 'POST',
                body: JSON.stringify({ phone, password }),
                skipAuth: true
            });
            
            if (data.token) {
                setToken(data.token);
                setCurrentUser({ phone });
            }
            
            return data;
        },

        logout() {
            clearToken();
        },

        isLoggedIn() {
            return !!getToken();
        },

        getCurrentUser,
        getToken,

        // ========== 视频相关 API ==========
        
        async getVideos() {
            console.log('正在获取视频列表...');
            return request('/videos', { method: 'GET' });
        },

        async uploadVideo(title, description, file, onProgress) {
            // 支持两种调用方式：
            // 1. uploadVideo(title, description, file, onProgress) - 传统方式
            // 2. uploadVideo(file, {title, description, onProgress}) - 新方式（用于录制上传）
            let videoFile, videoTitle, videoDescription, progressCallback;
            
            if (title instanceof File || title instanceof Blob) {
                // 新方式：第一个参数是 File/Blob，第二个参数是配置对象
                videoFile = title;
                const options = description || {};
                videoTitle = options.title || 'Untitled';
                videoDescription = options.description || '';
                progressCallback = options.onProgress;
            } else {
                // 传统方式
                videoFile = file;
                videoTitle = title;
                videoDescription = description || '';
                progressCallback = onProgress;
            }
            
            console.log('开始上传视频:', videoTitle, videoFile.name || 'blob');
            const formData = new FormData();
            formData.append('file', videoFile);  // 使用 'file' 字段（后端支持）
            formData.append('title', videoTitle);
            formData.append('description', videoDescription);
            
            return uploadWithProgress('/videos', formData, progressCallback);
        },

        async deleteVideo(videoId) {
            console.log('删除视频:', videoId);
            return request(`/videos/${videoId}`, { method: 'DELETE' });
        }
    };
})();
