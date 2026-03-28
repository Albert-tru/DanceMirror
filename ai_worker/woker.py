import json
import pika
import cv2
import mediapipe as mp
mp_pose = mp.solutions.pose
import numpy as np
import redis
import requests
import os
import time
import random
import google.generativeai as genai  # ✅ 新增引入

# 配置 (最好从环境变量读)
RABBITMQ_HOST = os.getenv('RABBITMQ_HOST', 'rabbitmq')
REDIS_HOST = os.getenv('REDIS_HOST', 'redis')
REDIS_PORT = 6379

# ✅ 配置 Gemini API (从环境变量读取，如果没有则留空)
GEMINI_API_KEY = os.getenv('GEMINI_API_KEY', '')  # ⚠️ 请确保在 docker-compose 或环境变量中设置此值
if GEMINI_API_KEY:
    genai.configure(api_key=GEMINI_API_KEY)

# 初始化 Redis
r_cache = redis.Redis(host=REDIS_HOST, port=REDIS_PORT, db=0)

# 初始化 MediaPipe
mp_pose = mp.solutions.pose
pose = mp_pose.Pose(min_detection_confidence=0.5, min_tracking_confidence=0.5)

def calculate_angle(a, b, c):
    """计算关节夹角"""
    a = np.array(a)
    b = np.array(b)
    c = np.array(c)
    radians = np.arctan2(c[1]-b[1], c[0]-b[0]) - np.arctan2(a[1]-b[1], a[0]-b[0])
    angle = np.abs(radians*180.0/np.pi)
    if angle > 180.0: angle = 360-angle
    return angle

# AI 教练 Agent 函数
def ask_ai_coach(score, error_log):
    """
    调用 Google Gemini API 生成个性化点评
    error_log 示例：
    {
      "right_arm_low": 12,
      "left_arm_low": 8,
      "torso_tilt": 6,
      "rhythm_off": 4
    }
    """
    error_labels = {
        "right_arm_low": "右臂抬高不够",
        "left_arm_low": "左臂抬高不够",
        "torso_tilt": "身体中轴不稳",
        "rhythm_off": "节奏不够稳定"
    }

    ranked = sorted(
        [(k, v) for k, v in error_log.items() if v > 0],
        key=lambda x: x[1],
        reverse=True
    )

    if not GEMINI_API_KEY:
        if ranked:
            top = ranked[0]
            top_label = error_labels.get(top[0], top[0])
            return f"分数 {score} 分。主要问题是{top_label}（{top[1]}次），先把这一点练稳，进步会很快。💪"
        elif score >= 85:
            return f"太棒了！分数 {score} 分，动作非常标准！🔥"
        elif score >= 70:
            return f"不错！分数 {score} 分，继续保持！👍"
        else:
            return f"分数 {score} 分，动作基础在了，再把幅度做开一点会更好！💃"

    try:
        top_text = "、".join([f"{error_labels.get(k, k)}({v}次)" for k, v in ranked[:3]]) if ranked else "未发现明显错误"
        prompt = f"""
你是一位专业街舞教练。
本次评分：{score}/100
主要问题：{top_text}

请输出100字以内中文点评，要求：
1) 口语化，有鼓励，也有明确改进点
2) 优先给出现次数最多问题的纠正建议
3) 不要使用 markdown
"""
        model = genai.GenerativeModel('gemini-1.5-flash')
        response = model.generate_content(prompt)
        return (response.text or "").strip() or f"分数 {score} 分，继续加油！"
    except Exception as e:
        print(f"❌ LLM 调用失败: {e}")
        return f"AI 教练正忙，但他看到你得了 {score} 分，继续加油！"
# 示例输出：
# "你的 vibe 很足，分数 75 分。不过右臂抬得还是有点低，
#  再往上提 10° 左右就完美了。加油兄弟，下次突破 85！"


def analyze_video(video_url, task_id):
    """
    完整的视频分析流程：
    1. 初始化模型
    2. 逐帧处理视频
    3. 检测关键点
    4. 计算关节角度
    5. 判断动作质量
    6. 生成弹幕建议
    7. 调用 LLM 生成点评
    """

    """
    离线分析：多维度建议 + 去重节流 + 更丰富文案
    """
    print(f"📥 下载视频: {video_url}")
    cap = cv2.VideoCapture(video_url)

    fps = cap.get(cv2.CAP_PROP_FPS)
    if not fps or fps <= 1:
        fps = 30.0

    suggestions = []
    frame_count = 0
    total_score = 0.0
    valid_frames = 0

    error_stats = {
        "right_arm_low": 0,
        "left_arm_low": 0,
        "torso_tilt": 0,
        "rhythm_off": 0,
    }

    message_pool = {
        "right_arm_low": [
            "右手再抬高一点，动作会更有力量感！",
            "右臂幅度偏小，肩线再打开一点。",
            "右侧上肢不够到位，试试把肘部再提起来。",
            "右手位偏低，想象在推开上方空气。",
            "右臂线条还可以更舒展，再上提一点点。"
        ],
        "left_arm_low": [
            "左手再抬高一点，左右会更平衡！",
            "左臂幅度偏小，注意和右侧保持一致。",
            "左手位稍低，建议抬到肩线以上。",
            "左侧发力有点保守，动作再放开一点。",
            "左臂再展开，你的整体气场会更好。"
        ],
        "torso_tilt": [
            "身体中轴有点歪，核心再收紧。",
            "上身稳定性稍弱，注意骨盆和肩膀对齐。",
            "躯干有明显侧倾，先把重心稳住。",
            "核心控制再加强，动作会更干净。",
            "身体线条有点跑偏，注意立住中轴。"
        ],
        "rhythm_off": [
            "节奏有点赶，试着跟拍点慢半拍校准。",
            "卡点不够稳，先稳住节拍再提速。",
            "动作切换偏急，节奏感再放松一点。",
            "拍点有些漂，建议先做分段练习。",
            "节奏起伏偏大，先把基础拍踩实。"
        ]
    }

    last_emit_sec = {
        "right_arm_low": -99.0,
        "left_arm_low": -99.0,
        "torso_tilt": -99.0,
        "rhythm_off": -99.0
    }

    prev_right_angle = None
    prev_left_angle = None

    def maybe_emit(err_type, now_sec, extra_text=None):
        cooldown_sec = 3.0
        if now_sec - last_emit_sec[err_type] < cooldown_sec:
            return
        text = random.choice(message_pool[err_type])
        if extra_text:
            text = f"{text}（{extra_text}）"
        suggestions.append(f"{now_sec:.1f}秒: {text}")
        last_emit_sec[err_type] = now_sec

    suggestions.append("1.0秒: AI 教练已开始为你分析动作，请注意动作幅度！")

    while cap.isOpened():
        ret, frame = cap.read()
        if not ret:
            break

        frame_count += 1
        if frame_count % 10 != 0:
            continue

        now_sec = frame_count / fps
        image = cv2.cvtColor(frame, cv2.COLOR_BGR2RGB)
        results = pose.process(image)

        if not results.pose_landmarks:
            continue

        valid_frames += 1
        landmarks = results.pose_landmarks.landmark

        r_shoulder = [landmarks[12].x, landmarks[12].y]
        r_elbow = [landmarks[14].x, landmarks[14].y]
        r_hip = [landmarks[24].x, landmarks[24].y]

        l_shoulder = [landmarks[11].x, landmarks[11].y]
        l_elbow = [landmarks[13].x, landmarks[13].y]
        l_hip = [landmarks[23].x, landmarks[23].y]

        right_arm_angle = calculate_angle(r_elbow, r_shoulder, r_hip)
        left_arm_angle = calculate_angle(l_elbow, l_shoulder, l_hip)

        # 躯干侧倾角（越大越歪）
        mid_shoulder = [(l_shoulder[0] + r_shoulder[0]) / 2, (l_shoulder[1] + r_shoulder[1]) / 2]
        mid_hip = [(l_hip[0] + r_hip[0]) / 2, (l_hip[1] + r_hip[1]) / 2]
        dx = abs(mid_shoulder[0] - mid_hip[0])
        dy = abs(mid_shoulder[1] - mid_hip[1]) + 1e-6
        torso_tilt_deg = abs(np.degrees(np.arctan2(dx, dy)))

        frame_score = 0.0

        if right_arm_angle < 45:
            error_stats["right_arm_low"] += 1
            maybe_emit("right_arm_low", now_sec, f"右臂角度{int(right_arm_angle)}°")
        else:
            frame_score += 1.0

        if left_arm_angle < 45:
            error_stats["left_arm_low"] += 1
            maybe_emit("left_arm_low", now_sec, f"左臂角度{int(left_arm_angle)}°")
        else:
            frame_score += 1.0

        if torso_tilt_deg > 18:
            error_stats["torso_tilt"] += 1
            maybe_emit("torso_tilt", now_sec, f"侧倾{int(torso_tilt_deg)}°")
        else:
            frame_score += 1.0

        if prev_right_angle is not None and prev_left_angle is not None:
            right_jump = abs(right_arm_angle - prev_right_angle)
            left_jump = abs(left_arm_angle - prev_left_angle)
            if right_jump > 35 or left_jump > 35:
                error_stats["rhythm_off"] += 1
                maybe_emit("rhythm_off", now_sec)

        prev_right_angle = right_arm_angle
        prev_left_angle = left_arm_angle

        total_score += frame_score / 3.0

    cap.release()

    final_score = int((total_score / valid_frames) * 100) if valid_frames > 0 else 60
    final_score = min(100, max(60, final_score))

    print("🤖 正在请求 AI 教练生成点评...")
    ai_comment = ask_ai_coach(final_score, error_stats)

    if len(suggestions) <= 1:
        suggestions.append("2.0秒: 动作整体不错，继续保持当前节奏！")

    suggestions.insert(0, f"0.5秒: 【AI总评】{ai_comment}")

    result = {
        "score": final_score,
        "suggestions": suggestions,
        "ai_summary": ai_comment,
        "status": "finished",
        "create_time": time.time(),
        "error_stats": error_stats
    }
    print(f"DEBUG suggestions count: {len(suggestions)}")
    return result
    
def start_worker():
    print("🔧 Worker 启动中...")
    retry_count = 0
    max_retries = 15
    
    while retry_count < max_retries:
        try:
            print(f"🐰 正在连接 RabbitMQ: {RABBITMQ_HOST}:5672 (尝试 {retry_count + 1}/{max_retries})...")
            connection = pika.BlockingConnection(
                pika.ConnectionParameters(
                    host=RABBITMQ_HOST,
                    connection_attempts=3,
                    retry_delay=2
                )
            )
            print("✅ RabbitMQ 连接成功！")
            channel = connection.channel()
            
            # ✅ 声明死信交换机
            channel.exchange_declare(
                exchange='dlx_exchange',
                exchange_type='direct',
                durable=True
            )
            
            # ✅ 声明死信队列
            channel.queue_declare(
                queue='video_analyze_queue_dlq',
                durable=True
            )
            
            # ✅ 绑定死信队列
            channel.queue_bind(
                exchange='dlx_exchange',
                queue='video_analyze_queue_dlq',
                routing_key='video_analyze_queue_dlq'
            )
            
            # ✅ 声明主队列
            channel.queue_declare(
                queue='video_analyze_queue',
                durable=True,
                arguments={
                    'x-dead-letter-exchange': 'dlx_exchange',
                    'x-dead-letter-routing-key': 'video_analyze_queue_dlq'
                }
            )
            print("✅ 队列系统声明完毕")
            
            channel.basic_qos(prefetch_count=1)
            channel.basic_consume(queue='video_analyze_queue', on_message_callback=callback)
            
            print(' [*] ✅ Worker 已准备就绪，等待任务...')
            channel.start_consuming()
            
        except pika.exceptions.AMQPConnectionError as e:
            retry_count += 1
            print(f"❌ RabbitMQ 连接失败: {e}，{5}秒后重试...")
            time.sleep(5)
            
        except Exception as e:
            retry_count += 1
            print(f"❌ Worker 发生错误: {e}，{5}秒后重试...")
            time.sleep(5)
    
    print(f"❌ Worker 启动失败：超过最大重试次数 ({max_retries})")

if __name__ == '__main__':
    print("🚀 DanceMirror AI Worker 启动")
    if not GEMINI_API_KEY:
        print("⚠️ 警告: 未检测到 GEMINI_API_KEY，智能点评功能将不可用")
    else:
        print("✨ Gemini AI Agent 已激活")
    start_worker()