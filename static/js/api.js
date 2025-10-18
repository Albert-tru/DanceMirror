/**
 * DanceMirror API 客户端
 * 封装所有 API 调用和认证逻辑
 */

const DanceMirrorAPI = (function() {
    'use strict';

    // 配置
    const config = {
        apiBase: 'http://192.168.20.116:8080/api/v1',
        tokenKey: 'token',
        userKey: 'user'
    };

    // 获取 token
    function getToken() {
        return localStorage.getItem(config.tokenKey);
    }

    // 设置 token
    function setToken(token) {
        localStorage.setItem(config.tokenKey, token);
    }

    // 清除 token
    function clearToken() {
        localStorage.removeItem(config.tokenKey);
        localStorage.removeItem(config.userKey);
    }

    // 获取当前用户
    function getCurrentUser() {
        const userStr = localStorage.getItem(config.userKey);
        return userStr ? JSON.parse(userStr) : null;
    }

    // 设置当前用户
    function setCurrentUser(user) {
        localStorage.setItem(config.userKey, JSON.stringify(user));
    }

    // 通用请求方法
    async function request(endpoint, options = {}) {
        const url = `${config.apiBase}${endpoint}`;
        const token = getToken();
        const headers = {
            ...options.headers
        };
        if (token && !options.skipAuth) {
            headers['Authorization'] = `Bearer ${token}`;
        }
        const fetchOptions = {
            ...options,
            headers
        };
        return fetch(url, fetchOptions).then(async res => {
            if (!res.ok) {
                let errText = await res.text();
                throw new Error(errText || res.statusText);
            }
            try {
                return await res.json();
            } catch (e) {
                return await res.text();
            }
        });
    }

    // API 对象
    const api = {
        // 注册
        register: async function(phone, firstName, lastName, password) {
            return await request('/register', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ phone, firstName, lastName, password }),
                skipAuth: true
            });
        },

        // 登录
        login: async function(phone, password) {
            return await request('/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ phone, password }),
                skipAuth: true
            }).then(data => {
                if (data.token) {
                    setToken(data.token);
                    if (data.user) setCurrentUser(data.user);
                }
                return data;
            });
        },

        // 获取视频列表
        getVideos: async function() {
            return await request('/videos', { method: 'GET' });
        },

        // 获取单个视频
        getVideo: async function(id) {
            return await request(`/videos/${id}`, { method: 'GET' });
        },

        // 上传视频（支持多种签名）
        uploadVideo: async function(arg1, arg2, arg3, arg4) {
            let file, opts = {}, onProgress;
            if (arg1 && typeof arg1 === 'object' && arg1.file) {
                file = arg1.file;
                opts.title = arg1.title;
                opts.description = arg1.description;
                onProgress = arg2;
            } else if (typeof arg1 === 'string') {
                opts.title = arg1;
                opts.description = arg2;
                file = arg3;
                onProgress = arg4;
            } else {
                throw new Error('参数错误');
            }
            if (!file) throw new Error('未提供文件');
            const form = new FormData();
            form.append('file', file, file.name || ('upload_' + Date.now() + '.webm'));
            if (opts.title) form.append('title', opts.title);
            if (opts.description) form.append('description', opts.description);

            if (onProgress) {
                return new Promise((resolve, reject) => {
                    const xhr = new XMLHttpRequest();
                    xhr.upload.addEventListener('progress', (e) => {
                        if (e.lengthComputable) {
                            const percent = Math.round((e.loaded / e.total) * 100);
                            onProgress(percent);
                        }
                    });
                    xhr.addEventListener('load', () => {
                        if (xhr.status >= 200 && xhr.status < 300) {
                            try {
                                const data = JSON.parse(xhr.responseText || '{}');
                                resolve(data);
                            } catch (e) {
                                resolve(xhr.responseText);
                            }
                        } else {
                            reject(new Error('上传失败: ' + xhr.status + ' ' + xhr.statusText));
                        }
                    });
                    xhr.addEventListener('error', () => {
                        reject(new Error('上传失败: 网络错误'));
                    });
                    xhr.open('POST', `${config.apiBase}/videos`);
                    const token = getToken();
                    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);
                    xhr.send(form);
                });
            } else {
                return await request('/videos', {
                    method: 'POST',
                    body: form
                });
            }
        },

        // 删除视频
        deleteVideo: async function(id) {
            return await request(`/videos/${id}`, { method: 'DELETE' });
        },

        // 下载文件
        downloadFile: function(path) {
            window.open(path, '_blank');
        },

        // 检查是否已登录
        isLoggedIn: function() {
            return !!getToken();
        },

        // 获取当前用户
        getCurrentUser: getCurrentUser,

        // 获取 token
        getToken: getToken,

        // 设置 token
        setToken: setToken,

        // 清除 token
        clearToken: clearToken,

        // 设置当前用户
        setCurrentUser: setCurrentUser
    };

    // 挂载到全局
    window.DanceMirrorAPI = api;
    return api;
})();
