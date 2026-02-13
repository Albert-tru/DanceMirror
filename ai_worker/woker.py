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

# ✅ 新增：AI 教练 Agent 函数
def ask_ai_coach(score, error_log):
    """调用 LLM 生成点评"""
    if not GEMINI_API_KEY:
        return "AI 教练提示：请在后台配置 Gemini API Key 以解锁智能点评功能。"

    try:
        # 构建 Prompt
        prompt = f"""
        你是一位专业的街舞教练。
        我刚刚跳了一段舞，机器评分是 {score}/100。
        主要检测到的问题列表：{str(error_log)}
        
        请你生成一段简短的点评（100字以内）：
        1. 语气要像真人口语，有时严厉有时鼓励，带一点街舞圈的黑话（比如 vibe, swish, power）。
        2. 先根据分数给个整体评价。
        3. 针对那个出现最多次的问题给一个具体的修改建议。
        4. 不要使用 markdown 格式。
        """
        
        model = genai.GenerativeModel('gemini-1.5-flash')
        response = model.generate_content(prompt)
        return response.text.strip()
    except Exception as e:
        print(f"❌ LLM 调用失败: {e}")
        return f"AI 教练正忙，但他看到你得了 {score} 分，继续加油！"

def analyze_video(video_url, task_id):
    print(f"📥 下载视频: {video_url}")
    
    # 下载视频流
    cap = cv2.VideoCapture(video_url)
    
    suggestions = []
    frame_count = 0
    total_score = 0
    valid_frames = 0
    
    # ✅ 记录具体的错误类型 tally
    error_stats = {
        "arm_too_low": 0,
        "timing_off": 0
    }

    # ✅ 无论如何都加一条弹幕建议
    suggestions.append("1.0秒: AI 教练已开始为你分析动作，请注意动作幅度！")

    
    while cap.isOpened():
        ret, frame = cap.read()
        if not ret: break
        
        frame_count += 1
        # 每 10 帧分析一次，节省 CPU
        if frame_count % 10 != 0: continue
            
        image = cv2.cvtColor(frame, cv2.COLOR_BGR2RGB)
        results = pose.process(image)
        
        if results.pose_landmarks:
            valid_frames += 1
            landmarks = results.pose_landmarks.landmark
            
            # --- 核心算法逻辑：检测“举手”动作 ---
            # 获取右侧：肩(12), 肘(14), 腕(16)
            r_shoulder = [landmarks[12].x, landmarks[12].y]
            r_elbow = [landmarks[14].x, landmarks[14].y]
            r_wrist = [landmarks[16].x, landmarks[16].y]
            
            # 计算腋下角度 (躯干-肩-肘) 近似算法
            # 这里简化为计算 肘-肩-髋 的角度来判断是否抬起
            r_hip = [landmarks[24].x, landmarks[24].y]
            armpit_angle = calculate_angle(r_elbow, r_shoulder, r_hip)
            
            # 规则：如果你在做“扩胸”或“举手”，角度应该 > 80度
            # 这里的规则可以写得很复杂，为了演示简单点：
            if armpit_angle < 45:
                # 限制弹幕密度：每2秒（约60帧）最多发一条同样的
                if frame_count % 60 == 0:
                    suggestions.append(f"{frame_count/30:.1f}秒: 右胳膊抬得不够高哦！({int(armpit_angle)}°)")
                error_stats["arm_too_low"] += 1
            else:
                total_score += 1 # 简单的加分逻辑
                
    cap.release()
    

    # 计算最终得分 (0-100)
    final_score = int((total_score / valid_frames) * 100) if valid_frames > 0 else 0
    final_score = min(100, max(60, final_score)) # 修正到 60-100 区间鼓励用户
    
    # ✅ 调用 AI Agent 生成最终点评
    print("🤖 正在请求 AI 教练生成点评...")
    ai_comment = ask_ai_coach(final_score, error_stats)
    
    # 将 AI 点评作为第 0 秒的弹幕，或者放在前端特定的显示的区域
    # 这里我们把它加进 suggestions 列表的最前面，用特殊前缀标识
    suggestions.insert(0, f"0.5秒: 【AI总评】{ai_comment}")

    # 结果打包
    print(f"DEBUG suggestions count: {len(suggestions)}")
    result = {
        "score": final_score,
        "suggestions": suggestions,  
        "ai_summary": ai_comment, # ✅ 单独存一个字段方便前端读取
        "status": "finished",
        "create_time": time.time()
    }
    
    return result

def callback(ch, method, properties, body):
    task = json.loads(body)
    print(f"🚀 收到任务: Video {task['video_id']}")
    
    try:
        # 1. 运行 AI 分析
        report = analyze_video(task['input_path'], task['video_id'])
        
        # 2. 存入 Redis (过期时间 24小时)
        redis_key = f"analysis:video:{task['video_id']}"
        r_cache.set(redis_key, json.dumps(report), ex=86400)
        print(f"✅ 分析完成，结果已存入 Redis: {redis_key}")
        
    except Exception as e:
        print(f"❌ 分析失败: {e}")
    
    ch.basic_ack(delivery_tag=method.delivery_tag)

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