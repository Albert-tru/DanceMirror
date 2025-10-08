# 📱 手机无法访问排查指南

## 当前状态
✅ Windows 浏览器可以访问 http://192.168.20.116:8080
✅ 端口转发已配置
❌ 手机 Safari 显示"伺服器停止回应"

---

## 🔍 问题诊断树

### 第一步：确认基础配置

```bash
# 在 Windows PowerShell (管理员) 运行
netsh interface portproxy show all

# 应该看到：
侦听 ipv4:             连接 ipv4:
地址        端口        地址            端口
0.0.0.0     8080        192.168.20.116  8080
```

如果**没有看到**上面的规则，说明端口转发没生效，重新配置。

---

### 第二步：获取正确的访问地址

**关键：手机应该访问 Windows IP，不是 WSL IP！**

```powershell
# Windows PowerShell 运行
ipconfig | findstr IPv4

# 找到类似这样的：
IPv4 地址 . . . . . . . . . . . . : 192.168.16.1  ← 这是 Windows IP
```

手机访问：`http://192.168.16.1:8080` （你的 Windows IP）

---

### 第三步：检查 WiFi 连接

| 检查项 | 方法 |
|--------|------|
| **同一 WiFi？** | 电脑和手机都连接"XX路由器_5G"吗？ |
| **WiFi 名称完全相同？** | 不要一个连 2.4G，一个连 5G |
| **VPN 是否开启？** | 手机关闭 VPN 再试 |

---

### 第四步：测试路由器是否隔离设备

#### 方法A：手机 ping 测试

1. 手机下载"网络工具"App（iOS：Network Ping Lite）
2. Ping 你的 Windows IP（例如 192.168.16.1）
3. 结果：
   - ✅ **有响应** → 网络通畅，继续排查
   - ❌ **请求超时** → 路由器开启了 AP 隔离

#### 方法B：电脑 ping 手机

1. 查看手机 IP：设置 → WiFi → 点击已连接的网络 → 查看 IP
2. Windows CMD 运行：`ping 手机IP`
3. 结果同上

---

### 第五步：关闭可能的防火墙阻止

#### Windows 防火墙配置

```powershell
# Windows PowerShell (管理员)

# 完全允许端口 8080
netsh advfirewall firewall add rule name="WSL2 DanceMirror" dir=in action=allow protocol=TCP localport=8080 profile=any

# 查看规则
netsh advfirewall firewall show rule name="WSL2 DanceMirror"
```

#### 临时测试：关闭防火墙

⚠️ 仅用于测试！

```
Win + I → 更新和安全 → Windows 安全中心 → 防火墙和网络保护
→ 分别关闭"域网络"、"专用网络"、"公用网络"
```

测试手机能否访问：
- ✅ **能访问** → 是防火墙问题，重新配置规则
- ❌ **还是不行** → 不是防火墙问题，重新打开

---

### 第六步：检查路由器 AP 隔离

如果前面的步骤都正常，但手机还是访问不了，可能是路由器开启了 AP 隔离。

#### 检查方法

1. 浏览器访问路由器管理页面（通常是 `192.168.1.1` 或 `192.168.0.1`）
2. 登录（用户名/密码通常在路由器背面）
3. 找到"无线设置" → "高级设置"
4. 查找以下选项：
   - AP 隔离
   - 无线隔离
   - 客户端隔离
   - WiFi 隔离
5. 如果开启了，**关闭它**

#### 常见路由器品牌

| 品牌 | 管理地址 | 默认密码 |
|------|---------|---------|
| TP-Link | 192.168.1.1 | admin/admin |
| 小米路由器 | 192.168.31.1 | WiFi密码 |
| 华为路由器 | 192.168.3.1 | WiFi密码 |
| 华硕路由器 | 192.168.1.1 | admin/admin |

---

### 第七步：使用手机热点测试

如果路由器无法修改，使用手机热点排除问题：

1. 手机A 开启热点
2. 电脑连接手机A的热点
3. 重新配置端口转发（因为 Windows IP 变了）
4. 手机B 连接手机A的热点
5. 手机B 访问新的 Windows IP

能访问 → 是路由器的问题  
还是不行 → 继续排查

---

## 🛠 完整修复流程

### 自动化脚本（推荐）

1. 复制这个文件到 Windows：
   ```
   \\wsl$\Ubuntu\tmp\fix-wsl-network.bat
   ```

2. 右键 → **以管理员身份运行**

3. 脚本会自动：
   - 获取 WSL IP 和 Windows IP
   - 配置端口转发
   - 配置防火墙
   - 显示手机访问地址

### 手动配置（如果脚本失败）

```powershell
# 1. 获取 WSL IP
wsl hostname -I
# 假设输出：192.168.20.116

# 2. 获取 Windows IP
ipconfig | findstr "192.168"
# 假设输出：192.168.16.1

# 3. 删除旧规则
netsh interface portproxy delete v4tov4 listenport=8080 listenaddress=0.0.0.0

# 4. 添加端口转发（替换成你的 WSL IP）
netsh interface portproxy add v4tov4 listenport=8080 listenaddress=0.0.0.0 connectport=8080 connectaddress=192.168.20.116

# 5. 配置防火墙
netsh advfirewall firewall add rule name="WSL2 DanceMirror" dir=in action=allow protocol=TCP localport=8080 profile=any

# 6. 验证配置
netsh interface portproxy show all

# 7. 测试 Windows 访问
curl http://localhost:8080
```

---

## 📊 测试清单

### Windows 测试

```
浏览器测试：
□ http://localhost:8080 → ✅ 能访问
□ http://127.0.0.1:8080 → ✅ 能访问
□ http://192.168.20.116:8080 (WSL IP) → ✅ 能访问
□ http://192.168.16.1:8080 (Windows IP) → ✅ 能访问

如果 Windows IP 访问失败，检查端口转发配置
```

### 手机测试

```
手机 Safari 测试：
□ http://192.168.16.1:8080 (Windows IP)

错误类型：
- "无法连接到伺服器" → 网络不通，检查 WiFi 或路由器
- "伺服器停止回应" → 超时，可能是防火墙或 AP 隔离
- "找不到伺服器" → IP 地址错误，重新确认
```

---

## 🎯 快速排查命令合集

复制到 PowerShell (管理员) 一次性执行：

```powershell
Write-Host "=== DanceMirror 网络诊断 ===" -ForegroundColor Cyan
Write-Host ""

# WSL IP
$wslIP = (wsl hostname -I).Trim()
Write-Host "WSL IP: $wslIP" -ForegroundColor Green

# Windows IP
$winIP = (Get-NetIPAddress -AddressFamily IPv4 | Where-Object {$_.IPAddress -like "192.168.*"}).IPAddress[0]
Write-Host "Windows IP: $winIP" -ForegroundColor Green
Write-Host ""

# 当前端口转发规则
Write-Host "端口转发规则：" -ForegroundColor Yellow
netsh interface portproxy show all
Write-Host ""

# 防火墙规则
Write-Host "防火墙规则：" -ForegroundColor Yellow
Get-NetFirewallRule -DisplayName "*DanceMirror*" | Format-Table DisplayName,Enabled,Direction,Action
Write-Host ""

# 测试本地访问
Write-Host "测试本地访问..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "http://localhost:8080" -Method Head -TimeoutSec 3
    Write-Host "✓ localhost:8080 可访问" -ForegroundColor Green
} catch {
    Write-Host "✗ localhost:8080 无法访问" -ForegroundColor Red
}

# 测试 Windows IP 访问
try {
    $response = Invoke-WebRequest -Uri "http://$winIP:8080" -Method Head -TimeoutSec 3
    Write-Host "✓ $winIP:8080 可访问" -ForegroundColor Green
} catch {
    Write-Host "✗ $winIP:8080 无法访问" -ForegroundColor Red
}

Write-Host ""
Write-Host "手机访问地址：http://$winIP:8080" -ForegroundColor Cyan
```

---

## 💡 最可能的原因

根据你的情况：

1. **80% 可能性：路由器 AP 隔离**
   - 症状：Windows 能访问，手机超时
   - 解决：路由器管理页面关闭 AP 隔离

2. **15% 可能性：防火墙阻止公用网络**
   - 症状：同上
   - 解决：防火墙规则添加 `profile=any`

3. **5% 可能性：手机使用了 VPN 或代理**
   - 症状：部分网站能访问，局域网不行
   - 解决：关闭 VPN

---

## 🆘 终极解决方案

如果以上都不行，使用 ngrok 内网穿透（临时）：

```bash
# WSL 中运行
curl -s https://ngrok-agent.s3.amazonaws.com/ngrok.asc | sudo tee /etc/apt/trusted.gpg.d/ngrok.asc >/dev/null
echo "deb https://ngrok-agent.s3.amazonaws.com buster main" | sudo tee /etc/apt/sources.list.d/ngrok.list
sudo apt update && sudo apt install ngrok

# 注册并获取 token：https://ngrok.com
ngrok authtoken 你的token

# 启动隧道
ngrok http 8080

# 会显示类似：
# Forwarding  https://xxxx.ngrok.io -> http://localhost:8080

# 手机访问 https://xxxx.ngrok.io
```

优点：任何地方都能访问  
缺点：需要网络、有延迟

---

## 📞 仍然无法解决？

提供以下信息我继续帮你：

1. PowerShell 诊断脚本的输出
2. 路由器型号和设置截图
3. 手机显示的具体错误信息
4. 手机能否 ping 通 Windows IP

---

立即运行 `/tmp/fix-wsl-network.bat` 脚本试试！
