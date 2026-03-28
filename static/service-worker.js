// 版本号从 SW URL 参数获取（由注册端自动带上）
const SW_VERSION = new URL(self.location.href).searchParams.get('v') || 'v1';
const CACHE_NAME = `dancemirror-${SW_VERSION}`;

const urlsToCache = [
  '/static/manifest.json',
  '/static/css/common.css',
  '/static/css/auth.css'
];

const isNetworkFirstStatic = (pathname) =>
  pathname.startsWith('/static/') &&
  (pathname.endsWith('.html') || pathname.endsWith('.js') || pathname.endsWith('.css'));

self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // API: 网络优先，不缓存 API 结果（避免旧 analysis/process 状态）
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(fetch(request));
    return;
  }

  // HTML/JS/CSS: 网络优先，失败才回退缓存
  if (request.method === 'GET' && isNetworkFirstStatic(url.pathname)) {
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response && response.status === 200) {
            const clone = response.clone();
            caches.open(CACHE_NAME).then((cache) => cache.put(request, clone));
          }
          return response;
        })
        .catch(() => caches.match(request))
    );
    return;
  }

  // 其它静态资源: 缓存优先
  event.respondWith(
    caches.match(request).then((cached) => {
      if (cached) return cached;
      return fetch(request).then((response) => {
        if (response && response.status === 200 && request.method === 'GET') {
          const clone = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(request, clone));
        }
        return response;
      });
    })
  );
});