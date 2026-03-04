# DanceMirror 性能测试手册（k6 + Docker）

> 适用对象：不了解项目的人  
> 目标：用可复现方式完成 API 压测，产出可对比数据（RPS、P95/P99、错误率、资源占用），用于性能评估与简历量化。

---

## 1. 目标与范围

### 1.1 目标
通过压测量化后端 API 性能：
- 验证系统在一定并发下的**稳定性**（错误率、超时）
- 衡量吞吐与时延（**RPS**、**P95/P99**）
- 通过“优化前/优化后”对比形成**可写简历**的量化结果

### 1.2 核心指标（必须记录）
- **吞吐**：`http_reqs`（req/s）
- **延迟**：`http_req_duration` 的 `p95/p99`（不要只看 avg）
- **稳定性**：`http_req_failed`（失败率）
- **资源指标（建议）**：容器 CPU/内存/网络，DB 连接数，Redis stats，RabbitMQ 队列堆积与 ack/publish 速率

### 1.3 测试范围（当前脚本覆盖）
目录：`perf/k6/`
- `read_videos.js`：视频列表/详情混合读（基线首选）
- `search_videos.js`：搜索读
- `get_analysis.js`：查询分析结果读
- `analyze_video.js`：触发分析任务（写 + MQ）
- `crop_video.js`：触发裁剪任务（写 + MQ）
- `upload_video.js`：上传视频（最重，最后跑）
- `run_all.sh`：一键运行并落盘结果到 `perf/results/<timestamp>/`

---

## 2. 环境准备

### 2.1 必备组件
- Docker（服务依赖在容器：MariaDB、Redis、RabbitMQ、ES、MinIO…）
- k6（建议装在 WSL/Linux 环境运行）
- 后端对外地址：`http://localhost:8080`

### 2.2 检查容器状态
```bash
docker ps
```

至少应包含并处于 `Up`：
- `dancemirror-app`（8080）
- `dancemirror-db`（3306）
- `dancemirror-redis`（6379）
- `dancemirror-rabbitmq`（5672/15672）

### 2.3 安装 k6（WSL/Ubuntu）
```bash
sudo apt-get update
sudo apt-get install -y k6
k6 version
```

---

## 3. 固定数据规模（确保可复现/可对比）

### 3.1 为什么必须固定数据规模
数据规模变化会影响：
- DB 查询成本
- 响应体大小（网络吞吐）
- 缓存命中率  
导致压测数据不可对比，无法证明优化收益。

### 3.2 当前数据种子脚本
文件：`perf/sql/seed_videos.sql`  
用途：为固定用户（如 `userId=2`）生成固定数量视频记录（例如 5000 条）。

执行：
```bash
cd ~/go/DanceMirror
docker exec -i dancemirror-db mariadb -uroot -pMySQL666 dancemirror < perf/sql/seed_videos.sql
```

验证：
```bash
docker exec -it dancemirror-db mariadb -uroot -pMySQL666 dancemirror \
  -e "SELECT COUNT(*) FROM videos WHERE userId=2;"
```

### 3.3 数据规模档位建议
建议准备 3 档（每档都跑同样脚本）：
- 1k（快速回归）
- 5k（当前基线）
- 10k 或 50k（放大瓶颈，利于证明优化价值）

---

## 4. 并发用户量（VU）如何设计

### 4.1 VU 是什么
VU（Virtual User）是并发执行脚本的虚拟用户。  
**VU 不等于 QPS/RPS**，RPS 由接口响应时间、脚本逻辑与 `sleep` 决定。

### 4.2 设计原则（推荐）
- **爬坡（Ramp-up）**：逐步升并发，观察饱和点与拐点
- **稳态（Steady-state）**：在目标并发保持 60–120s，观察 P95/P99、错误率与资源
- **回落（Ramp-down）**：避免硬切导致统计偏差

### 4.3 推荐并发档位（按场景）
- 读（列表/详情/搜索）：50 → 200 → 500（按机器承受逐步提升）
- 写（analyze/crop）：5 → 20 → 50（注意限流与 MQ/worker 影响）
- 上传：1 → 5 → 10（重场景，最后执行）

---

## 5. 请求次数/时长如何设计

### 5.1 不建议固定“请求次数”
固定次数可能在系统未到稳态时就结束，无法观察：
- 缓存预热后变化
- DB 连接池稳定状态
- GC/内存抖动
- MQ 堆积与消费速率

### 5.2 推荐“固定时长 + 爬坡”
例如脚本常用结构：
- 20s 爬坡到 50 VUs
- 60s 爬坡到 200 VUs
- 60s 稳态 200 VUs
- 20s 回落

### 5.3 统计口径
每个场景建议：
- **同配置跑 3 次**，取中位数（减小偶然抖动）
- 同时记录**冷缓存**与**热缓存**两组结果

---

## 6. 冷缓存 vs 热缓存（必须区分）

### 6.1 冷缓存（Cold）
测试“首次访问/缓存失效”的性能。  
操作：压测前清 Redis：
```bash
docker exec -it dancemirror-redis redis-cli FLUSHALL
```
保存结果为 `*_cold.txt`。

### 6.2 热缓存（Hot）
测试“缓存命中”的性能。  
操作：
1) 先跑一次预热（输出丢弃）  
2) 再跑正式压测（保存为 `*_hot.txt`）

---

## 7. 执行流程（新手照做版）

### 7.1 进入项目根目录
```bash
cd ~/go/DanceMirror
```

### 7.2 固定数据规模（示例：5k）
```bash
docker exec -i dancemirror-db mariadb -uroot -pMySQL666 dancemirror < perf/sql/seed_videos.sql
```

### 7.3 读场景（冷缓存）
```bash
docker exec -it dancemirror-redis redis-cli FLUSHALL
k6 run perf/k6/read_videos.js \
  -e BASE_URL="http://localhost:8080" \
  -e PHONE="18131162480" \
  -e PASS="666666" \
  | tee perf/results/read_5k_cold.txt
```

### 7.4 读场景（热缓存）
```bash
k6 run perf/k6/read_videos.js \
  -e BASE_URL="http://localhost:8080" \
  -e PHONE="18131162480" \
  -e PASS="666666" > /dev/null

k6 run perf/k6/read_videos.js \
  -e BASE_URL="http://localhost:8080" \
  -e PHONE="18131162480" \
  -e PASS="666666" \
  | tee perf/results/read_5k_hot.txt
```

### 7.5 一键跑全部（可选）
```bash
chmod +x perf/k6/run_all.sh
PHONE=18131162480 PASS=666666 bash perf/k6/run_all.sh
```

结果目录：
- `perf/results/<timestamp>/`

---

## 8. 如何解读 k6 输出

关注这些指标：
- `http_reqs`：总请求数与 req/s（吞吐）
- `http_req_failed`：失败率（建议 < 1%）
- `http_req_duration`：重点看 `p(95)`、`p(99)`（尾延迟）

### 8.1 阈值失败是什么意思
当你看到：
```
ERRO[...] thresholds on metrics 'http_req_duration' have been crossed
```
含义：脚本阈值（如 `p95<250ms`）没有达标，k6 用非 0 退出码表示“性能目标失败”。  
**这不等于服务挂了**，只是“性能指标未达标”。

---

## 9. 资源与依赖侧证据（建议同步采集）

压测同时另开终端采集快照，并保存到 `perf/results/...`：

### 9.1 Docker 资源快照
```bash
docker stats --no-stream > perf/results/docker_stats_snapshot.txt
```

### 9.2 MariaDB 连接数
```bash
docker exec -it dancemirror-db mariadb -uroot -pMySQL666 dancemirror \
  -e "SHOW GLOBAL STATUS LIKE 'Threads_connected';" \
  > perf/results/mysql_threads_connected.txt
```

### 9.3 Redis 统计
```bash
docker exec -it dancemirror-redis redis-cli INFO stats > perf/results/redis_info_stats.txt
```

### 9.4 RabbitMQ（写场景时）
打开管理界面：`http://localhost:15672`  
观察并截图：
- 队列 Ready/Unacked
- publish/ack rate

---

## 10. “优化前后对比”怎么做（写简历的关键）

### 10.1 对比原则
- **同一数据规模**（例如固定 5k）
- **同一脚本、同一 VU 配置**
- **同一冷/热缓存条件**
- 每次只改一个点（便于归因）

### 10.2 优化方向建议（优先级）
针对读接口 P95 秒级场景，优先：
1) 列表接口加分页（默认 `size=20`），避免一次返回 5000 条
2) 列表字段裁剪（只返回必要字段）
3) 索引优化（如 `userId + createdAt`）
4) gzip 压缩（降低 `data_received`，减轻带宽压力）

### 10.3 简历写法模板
把 A/B/C/D 换成你的数据：
- 使用 k6 在 **200 VUs / 135s** 下对视频读接口压测，错误率保持 **0%**；通过 **分页 + 字段裁剪 + 索引优化**，将吞吐从 **A req/s 提升到 B req/s（+X%）**，将尾延迟 **P95 从 C 降到 D（-Y%）**。

---

## 11. 常见问题

### 11.1 为什么 `data_received` 很大（十几 GB）？
可能是 `/videos` 一次返回大量记录（例如单用户 5000 条），导致响应体巨大，网络带宽和序列化成为瓶颈，P95/P99 会显著升高。

### 11.2 为什么结束时 VU 显示很少（如 008/200）？
k6 会按 stages 动态调整并发；结束时处于回落阶段，显示的即时 VU 变小是正常现象。

---

## 12. 目录说明
- `perf/sql/seed_videos.sql`：固定数据规模（生成视频记录）
- `perf/k6/*.js`：k6 场景脚本
- `perf/k6/shared.js`：公共工具
- `perf/k6/run_all.sh`：一键执行脚本
- `perf/results/`：压测输出结果（txt）