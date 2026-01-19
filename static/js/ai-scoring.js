(function () {
    console.log("🚀 AI 骨架系统已加载 (V4.0-Final)");

    // 1. 定义骨架连线
    const POSE_CONNECTIONS = [
        [0, 1], [1, 2], [2, 3], [3, 7], [0, 4], [4, 5], [5, 6], [6, 8], [9, 10],
        [11, 12], [11, 13], [13, 15], [15, 17], [15, 19], [15, 21], [17, 19],
        [12, 14], [14, 16], [16, 18], [16, 20], [16, 22], [18, 20], [11, 23],
        [12, 24], [23, 24], [23, 25], [24, 26], [25, 27], [26, 28], [27, 29],
        [28, 30], [29, 31], [30, 32], [27, 31], [28, 32]
    ];

    // 2. AI 核心对象
    const AIContext = {
        isEnabled: false,
        teachPose: null, // 用于处理教学视频（单视频 or 左侧视频）
        userPose: null,  // 用于处理用户摄像头（仅右侧视频）
        
        async init() {
            if (this.teachPose) return;
            
            // 简单的依赖检查
            if (!window.Pose) {
                alert("❌ MediaPipe 库未加载，请检查网络！");
                return;
            }

            console.log("⚙️ 初始化 AI 模型...");
            
            // 初始化教学视频模型
            this.teachPose = new window.Pose({
                locateFile: (file) => `https://cdn.jsdelivr.net/npm/@mediapipe/pose/${file}`
            });
            this.teachPose.setOptions({ modelComplexity: 1, smoothLandmarks: true });
            this.teachPose.onResults(this.onTeachResults.bind(this));

            // 初始化用户视频模型 (只在对比模式用，但也先初始化好)
            this.userPose = new window.Pose({
                locateFile: (file) => `https://cdn.jsdelivr.net/npm/@mediapipe/pose/${file}`
            });
            this.userPose.setOptions({ modelComplexity: 1, smoothLandmarks: true });
            this.userPose.onResults(this.onUserResults.bind(this));
        },

        async start() {
    console.log("🚀 开始启动 AI...");
    
    // 显示加载提示
    if(typeof showMessage === 'function') {
        showMessage("⏳ 正在下载 AI 模型 (约10MB)，首次使用需要10-20秒...", "info", 15000);
    } else {
        alert("⏳ 正在下载 AI 模型，请等待 10-20 秒...");
    }
    
    try {
        await this.init();
        this.isEnabled = true;
        this.loop();
        console.log("✅ AI 启动成功");
        
        if(typeof showMessage === 'function') {
            showMessage("✅ AI 已启动，骨架将出现在视频上！", "success", 3000);
        }
    } catch(e) {
        console.error("❌ 启动失败:", e);
        if(typeof showMessage === 'function') {
            showMessage("❌ AI 启动失败: " + e.message, "error", 5000);
        } else {
            alert("AI 启动失败: " + e.message);
        }
    }
        },

        stop() {
            this.isEnabled = false;
            // 清理所有 Canvas
            this.clearCanvas('singleSkeletonCanvas');
            this.clearCanvas('teachSkeletonCanvas');
            this.clearCanvas('userSkeletonCanvas');
            
            const btn = document.getElementById('aiToggle');
            if (btn) btn.closest('label').style.color = "";
            console.log("🛑 AI 已停止");
        },

        // 核心循环：决定处理哪个视频
        async loop() {
            if (!this.isEnabled) return;

            // 1. 判断当前模式
            const isSingleMode = document.body.classList.contains('mode-single');
            
            // 2. 获取对应的视频元素
            // 单模式 -> singleVideo
            // 对比模式 -> originalVideo
            const teachVideo = isSingleMode 
                ? document.getElementById('singleVideo') 
                : document.getElementById('originalVideo');

            const userVideo = document.getElementById('userVideo');

            // 3. 发送处理请求 (MediaPipe)
            try {
                // 处理教学视频骨架
                if (teachVideo && !teachVideo.paused && teachVideo.readyState >= 2) {
                    await this.teachPose.send({ image: teachVideo });
                }

                // 处理用户视频骨架 (仅在对比模式且用户开启了摄像头时)
                if (!isSingleMode && userVideo && !userVideo.paused && userVideo.readyState >= 2) {
                    // 简单的检测：如果 userVideo 正在播放 (不管是摄像头还是文件)
                    await this.userPose.send({ image: userVideo });
                }
            } catch (e) {
                // 忽略偶尔的掉帧错误
            }
            
            // 下一帧
            if (this.isEnabled) {
                requestAnimationFrame(this.loop.bind(this));
            }
        },

        // 回调：绘制教学视频骨架
        onTeachResults(results) {
            // 智能判断画在哪里
            const isSingleMode = document.body.classList.contains('mode-single');
            const canvasId = isSingleMode ? 'singleSkeletonCanvas' : 'teachSkeletonCanvas';
            this.draw(results, canvasId);
        },

        // 回调：绘制用户视频骨架
        onUserResults(results) {
            // 单模式下不画用户骨架
            if (document.body.classList.contains('mode-single')) return;
            this.draw(results, 'userSkeletonCanvas');
        },

        // 通用绘制函数
        draw(results, canvasId) {
            const canvas = document.getElementById(canvasId);
            if (!canvas) return;

            const ctx = canvas.getContext('2d');
            
            // 🔧 强制由于 CSS 缩放导致的 Canvas 尺寸失调修复
            // 我们使用 offsetWidth/Height 作为渲染分辨率
            if (canvas.width !== canvas.offsetWidth || canvas.height !== canvas.offsetHeight) {
                canvas.width = canvas.offsetWidth;
                canvas.height = canvas.offsetHeight;
            }

            ctx.save();
            ctx.clearRect(0, 0, canvas.width, canvas.height);
            
            // 只有 MediaPipe 绘图工具加载成功才绘制
            if (results.poseLandmarks && window.drawConnectors) {
                window.drawConnectors(ctx, results.poseLandmarks, POSE_CONNECTIONS, { color: '#00FF00', lineWidth: 2 });
                window.drawLandmarks(ctx, results.poseLandmarks, { color: '#FF0000', lineWidth: 1, radius: 3 });
            }
            ctx.restore();
        },

        clearCanvas(id) {
            const c = document.getElementById(id);
            if (c) c.getContext('2d').clearRect(0, 0, c.width, c.height);
        }
    };

    // 暴露全局对象
    window.AIContext = AIContext;

    // 自动绑定事件 (等待 DOM 加载)
    function bind() {
        const toggle = document.getElementById('aiToggle');
        if (toggle) {
            // 移除旧监听器 (通过克隆节点)
            const newToggle = toggle.cloneNode(true);
            toggle.parentNode.replaceChild(newToggle, toggle);
            
            newToggle.addEventListener('change', (e) => {
                if(e.target.checked) AIContext.start();
                else AIContext.stop();
            });
            console.log("✅ AI 开关绑定成功");
        }
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', bind);
    } else {
        bind();
    }
})();