# 🌐 DanceMirror 前端使用指南

## ✅ 已完成
- ✅ 后端 API 服务运行中
- ✅ 前端页面已创建
- ✅ 用户认证功能完整
- ✅ 视频管理功能可用

---

## 🚀 快速开始

### 1. 确保服务器正在运行
```bash
cd ~/go/DanceMirror
./bin/dancemirror
```

### 2. 在浏览器中打开
```
http://localhost:8080/static/index.html
```

---

## 📋 功能说明

### 🔐 用户功能
1. **注册新用户**
   - 点击 "注册" 标签
   - 填写邮箱、名字、姓氏、密码
   - 点击注册按钮

2. **登录**
   - 点击 "登录" 标签
   - 输入邮箱和密码
   - 点击登录按钮
   - 登录成功后会自动跳转到视频列表

3. **退出登录**
   - 点击右上角的 "退出登录" 按钮

### 📹 视频功能
1. **查看视频列表**
   - 登录后点击 "视频列表" 标签
   - 可以看到所有已上传的视频
   - 点击播放按钮观看视频

2. **上传视频**
   - 点击 "上传视频" 标签
   - 填写视频标题和描述
   - 点击选择视频文件
   - 点击上传按钮
   - 上传成功后自动跳转到视频列表

---

## 🎨 界面特点

### 设计亮点
- 🎨 渐变紫色主题，现代美观
- 📱 响应式设计，适配各种屏幕
- ✨ 平滑动画过渡效果
- 🎯 直观的标签导航
- 💫 悬停效果增强交互体验

### 用户体验
- 自动保存登录状态（使用 localStorage）
- 实时错误提示
- 成功操作反馈
- 加载状态提示
- 表单验证

---

## 🔧 技术实现

### 前端技术栈
- **HTML5**: 语义化结构
- **CSS3**: 现代样式和动画
- **JavaScript (Vanilla)**: 无框架，纯原生 JS
- **Fetch API**: 与后端 API 通信

### API 调用
- 基础 URL: `http://localhost:8080/api/v1`
- 认证方式: JWT Token (Bearer Authentication)
- Token 存储: localStorage

### 核心功能实现
```javascript
// 注册
POST /api/v1/register
Body: { email, firstName, lastName, password }

// 登录
POST /api/v1/login
Body: { email, password }
Response: { token }

// 获取视频列表
GET /api/v1/videos
Headers: Authorization: Bearer <token>

// 上传视频
POST /api/v1/videos/upload
Headers: Authorization: Bearer <token>
Body: FormData (title, description, video file)
```

---

## 📝 测试账号

### 已有测试用户
- 邮箱: `test@dancemirror.com`
- 密码: `Test123456`

或者注册一个新用户进行测试。

---

## 🐛 故障排查

### 页面无法打开
1. 确认服务器正在运行
   ```bash
   ps aux | grep dancemirror
   ```

2. 检查端口是否被占用
   ```bash
   lsof -i :8080
   ```

3. 查看服务器日志
   ```bash
   tail -f ~/go/DanceMirror/server.log
   ```

### 无法登录
1. 检查邮箱和密码是否正确
2. 查看浏览器控制台错误信息 (F12)
3. 确认后端 API 正常工作

### 视频上传失败
1. 检查视频文件格式（推荐 MP4）
2. 确认文件大小不超过限制
3. 检查 uploads 目录权限
   ```bash
   mkdir -p ~/go/DanceMirror/uploads
   chmod 755 ~/go/DanceMirror/uploads
   ```

### 视频无法播放
1. 确认视频格式浏览器支持（推荐 H.264/MP4）
2. 检查上传是否完整
3. 查看浏览器控制台网络请求

---

## 🎯 下一步改进

### 功能增强
- [ ] 用户个人主页
- [ ] 视频搜索和过滤
- [ ] 视频评论功能
- [ ] 点赞和收藏
- [ ] 视频分类标签
- [ ] 用户关注系统

### UI/UX 优化
- [ ] 添加视频缩略图
- [ ] 进度条显示上传进度
- [ ] 更好的错误提示
- [ ] 暗色模式支持
- [ ] 多语言支持

### 技术优化
- [ ] 视频压缩和转码
- [ ] CDN 加速
- [ ] 懒加载优化
- [ ] PWA 支持
- [ ] WebSocket 实时通知

---

## 📱 移动端访问

在同一网络下，可以通过以下方式在手机访问：

1. 查看电脑 IP 地址
   ```bash
   ip addr show | grep inet
   ```

2. 在手机浏览器中访问
   ```
   http://<你的IP地址>:8080/static/index.html
   ```

---

## 📞 技术支持

如果遇到问题，可以：
1. 查看 `server.log` 日志文件
2. 检查浏览器控制台 (F12)
3. 参考 `API_TESTING.md` 测试 API
4. 查看 `JWT_FIX_REPORT.md` 了解认证问题

---

## 🎉 开始使用

现在就打开浏览器，访问：
**http://localhost:8080/static/index.html**

享受您的舞蹈分享之旅！💃🕺

