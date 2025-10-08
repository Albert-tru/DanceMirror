// Service Worker v1.0
const CACHE_NAME = 'dancemirror-v1';
const urlsToCache = [
  '/static/index.html',
  '/static/css/common.css',
  '/static/css/auth.css',
  '/static/js/api.js',
  '/static/js/utils.js'
];

// 安装：缓存资源
self.addEventListener('install', event => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then(cache => cache.addAll(urlsToCache))
      .then(() => self.skipWaiting())
  );
});

// 激活：清理旧缓存
self.addEventListener('activate', event => {
  event.waitUntil(
    caches.keys().then(keys => {
      return Promise.all(
        keys.filter(key => key !== CACHE_NAME)
           .map(key => caches.delete(key))
      );
    }).then(() => self.clients.claim())
  );
});

// 拦截请求：缓存优先
self.addEventListener('fetch', event => {
  // API 请求：网络优先
  if (event.request.url.includes('/api/')) {
    event.respondWith(
      fetch(event.request)
        .catch(() => new Response('{"error":"网络错误"}', {
          headers: { 'Content-Type': 'application/json' }
        }))
    );
    return;
  }
  
  // 静态资源：缓存优先
  event.respondWith(
    caches.match(event.request)
      .then(response => response || fetch(event.request))
  );
});
