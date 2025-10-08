// 工具函数库
const Utils = (function() {
    'use strict';
    
    return {
        showMessage(elementId, message, type = 'success', duration = 5000) {
            const msgEl = document.getElementById(elementId);
            if (!msgEl) return;
            
            msgEl.textContent = message;
            msgEl.className = `message ${type}`;
            msgEl.style.display = 'block';
            
            if (duration > 0) {
                setTimeout(() => msgEl.style.display = 'none', duration);
            }
        },
        
        formatTime(seconds) {
            const mins = Math.floor(seconds / 60);
            const secs = Math.floor(seconds % 60);
            return `${mins}:${secs.toString().padStart(2, '0')}`;
        },
        
        formatDate(date) {
            return new Date(date).toLocaleString('zh-CN', {
                year: 'numeric',
                month: '2-digit',
                day: '2-digit',
                hour: '2-digit',
                minute: '2-digit'
            });
        },
        
        formatFileSize(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
        },
        
        escapeHtml(str) {
            const div = document.createElement('div');
            div.textContent = str;
            return div.innerHTML;
        },
        
        validatePassword(password) {
            if (password.length < 6) {
                return { valid: false, message: '密码至少需要 6 个字符' };
            }
            return { valid: true, message: '密码强度合格' };
        }
    };
})();
