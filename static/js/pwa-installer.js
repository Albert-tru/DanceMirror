// PWA 安装管理器（精简版）
class PWAInstaller {
  constructor() {
    this.deferredPrompt = null;
    this.init();
  }

  init() {
    // 注册 Service Worker
    if ('serviceWorker' in navigator) {
      navigator.serviceWorker.register('/static/service-worker.js')
        .then(reg => console.log('✅ Service Worker 注册成功', reg.scope))
        .catch(err => console.error('❌ Service Worker 注册失败:', err));
    }

    // 监听安装提示
    window.addEventListener('beforeinstallprompt', (e) => {
      e.preventDefault();
      this.deferredPrompt = e;
      this.showInstallBanner();
    });

    // 监听安装成功
    window.addEventListener('appinstalled', () => {
      console.log('🎉 PWA 安装成功');
      this.hideInstallBanner();
    });
  }

  showInstallBanner() {
    const banner = document.createElement('div');
    banner.id = 'pwa-install-banner';
    banner.innerHTML = `
      <div style="position:fixed;bottom:0;left:0;right:0;background:white;padding:15px;box-shadow:0 -2px 10px rgba(0,0,0,0.1);z-index:9999;display:flex;justify-content:space-between;align-items:center;">
        <div>
          <strong>📱 安装 DanceMirror</strong>
          <p style="margin:5px 0 0 0;font-size:12px;color:#666;">添加到主屏幕，像 App 一样使用</p>
        </div>
        <button onclick="pwaInstaller.install()" style="background:#667eea;color:white;border:none;padding:10px 20px;border-radius:20px;font-weight:bold;cursor:pointer;">安装</button>
      </div>
    `;
    document.body.appendChild(banner);
  }

  hideInstallBanner() {
    const banner = document.getElementById('pwa-install-banner');
    if (banner) banner.remove();
  }

  async install() {
    if (!this.deferredPrompt) return;
    this.deferredPrompt.prompt();
    const { outcome } = await this.deferredPrompt.userChoice;
    console.log(`用户${outcome === 'accepted' ? '接受' : '拒绝'}安装`);
    this.deferredPrompt = null;
    this.hideInstallBanner();
  }
}

// 创建全局实例
const pwaInstaller = new PWAInstaller();
