(function () {
    console.log("🚀 AI 骨架评分系统已加载 (V4.1-Scoring)");

    // 1. 定义骨架连线
    const POSE_CONNECTIONS = [
        [11, 12], [11, 13], [13, 15], [12, 14], [14, 16],
        [11, 23], [12, 24], [23, 24], [23, 25], [24, 26], [25, 27], [26, 28]
    ];

    // 余弦相似度计算
    function cosineSimilarity(v1, v2) {
        if (!v1 || !v2 || v1.length === 0 || v1.length !== v2.length) return 0;
        let dot = 0, mag1 = 0, mag2 = 0;
        for (let i = 0; i < v1.length; i++) {
            dot += v1[i] * v2[i];
            mag1 += v1[i] * v1[i];
            mag2 += v2[i] * v2[i];
        }
        return dot / (Math.sqrt(mag1) * Math.sqrt(mag2) || 1e-6);
    }

    // 获取关键部位向量
    function getLimbVectors(landmarks) {
        if (!landmarks) return [];
        const connections = [
            [11, 13], [13, 15], // 左臂
            [12, 14], [14, 16], // 右臂
            [11, 23], [12, 24], // 躯干纵向
            [11, 12], [23, 24], // 躯干横向
            [23, 25], [25, 27], // 左腿
            [24, 26], [26, 28]  // 右腿
        ];
        let vectors = [];
        connections.forEach(([s, e]) => {
            const p1 = landmarks[s], p2 = landmarks[e];
            if (p1 && p2) {
                let dx = p2.x - p1.x, dy = p2.y - p1.y;
                let mag = Math.sqrt(dx * dx + dy * dy) || 1e-6;
                vectors.push(dx / mag, dy / mag);
            }
        });
        return vectors;
    }

    // 2. AI 核心对象
    const AIContext = {
        isEnabled: false,
        teachPose: null,
        userPose: null,
        lastTeachLandmarks: null,
        lastUserLandmarks: null,
        
        async init() {
            if (this.teachPose) return;
            if (!window.Pose) {
                alert("❌ MediaPipe 库未加载，请检查网络！");
                return;
            }

            console.log("⚙️ 初始化 AI 模型...");
            if(typeof showMessage === "function") showMessage("⏳ 正在初始化 AI 模型...", "info", 5000);
            
            //2.1 加载MediaPipe Pose模型
            this.teachPose = new window.Pose({ locateFile: (f) => `https://cdn.jsdelivr.net/npm/@mediapipe/pose/${f}` });
            
            //2.2 设置模型参数
            const options = { 
                modelComplexity: 1,             //模型复杂度，0-2，数值越大精度越高但性能要求更高
                smoothLandmarks: true,          //平滑关键点
                minDetectionConfidence: 0.5,    //最小检测置信度，0-1，数值越大要求越高但可能漏检
                minTrackingConfidence: 0.5      //最小跟踪置信度，0-1，数值越大要求越高但可能丢失跟踪
            };
            this.teachPose.setOptions(options);

            //2.3 绑定结果回调，保存关键点并绘制骨架
            this.teachPose.onResults(results => {
                this.lastTeachLandmarks = results.poseLandmarks;
                const isSingle = document.body.classList.contains('mode-single');
                const canvasId = isSingle ? 'singleSkeletonCanvas' : 'teachSkeletonCanvas';
                this.draw(results, canvasId);
            });

            //2.4 用户骨架+评分
            this.userPose = new window.Pose({ locateFile: (f) => `https://cdn.jsdelivr.net/npm/@mediapipe/pose/${f}` });
            this.userPose.setOptions(options);
            this.userPose.onResults(results => {
                this.lastUserLandmarks = results.poseLandmarks;
                this.draw(results, 'userSkeletonCanvas');      //绘制用户骨架
                this.computeScore();        //触发评分
            });
        },

        async start() {
            try {
                await this.init();
                this.isEnabled = true;
                this.loop();
                const board = document.getElementById('aiScoreBoard');
                if (board) {
                    board.classList.add('active');
                    board.style.opacity = '1';
                    board.style.display = 'flex';
                }
                console.log("✅ AI 启动成功");
                if(typeof showMessage === "function") showMessage("✅ AI 已启动！评分系统运行中", "success", 3000);
            } catch(e) { console.error("❌ 启动失败:", e); }
        },

        stop() {
            this.isEnabled = false;
            this.clearCanvas('singleSkeletonCanvas');
            this.clearCanvas('teachSkeletonCanvas');
            this.clearCanvas('userSkeletonCanvas');
            const board = document.getElementById('aiScoreBoard');
            if (board) {
                board.classList.remove('active');
                board.style.opacity = '0';
            }
        },

        async loop() {
            if (!this.isEnabled) return;
            const isSingle = document.body.classList.contains('mode-single');
            const teachVideo = isSingle ? document.getElementById('singleVideo') : document.getElementById('originalVideo');
            const userVideo = document.getElementById('userVideo');

            try {
                //单视频模式分析教学视频
                if (teachVideo && !teachVideo.paused && teachVideo.readyState >= 2) {
                    await this.teachPose.send({ image: teachVideo });   
                    //         ↓
                    // MediaPipe 输出 33 个关键点 (x, y, z, visibility)
                }
                //对比模式分析用户视频
                if (!isSingle && userVideo && !userVideo.paused && userVideo.readyState >= 2) {
                    await this.userPose.send({ image: userVideo });
                }
            } catch (e) {}
            
            // 循环调用自己，保持实时分析
            if (this.isEnabled) requestAnimationFrame(this.loop.bind(this));
        },

        //计算相似度并更新评分显示
        computeScore() {
            if (!this.lastUserLandmarks || !this.lastTeachLandmarks) return;
            
            // 获取骨架向量
            const vLeft = getLimbVectors(this.lastTeachLandmarks);
            const vRight = getLimbVectors(this.lastUserLandmarks);
            
            // 计算余弦相似度并转换为0-100分
            const sim = cosineSimilarity(vLeft, vRight);
            const score = Math.round(Math.max(0, sim * 100));
            
            // 更新评分显示
            const scoreEl = document.getElementById('liveScore');
            const commentEl = document.getElementById('scoreComment');
            const board = document.getElementById('aiScoreBoard');

            // 根据分数设置评论和颜色
            if (scoreEl) scoreEl.innerText = score;
            if (commentEl) {
                let text = "加油", color = "#ef4444";
                if (score > 85) { text = "完美!"; color = "#10b981"; }
                else if (score > 75) { text = "很棒!"; color = "#3b82f6"; }
                else if (score > 60) { text = "不错"; color = "#f59e0b"; }
                commentEl.innerText = text;
                commentEl.style.color = color;
                if (board) board.style.borderColor = color;
                const circle = board ? board.querySelector('.score-circle') : null;
                if (circle) circle.style.borderColor = color;
            }
        },

        // 绘制骨架
        draw(results, canvasId) {
            const canvas = document.getElementById(canvasId);
            if (!canvas) return;
            const ctx = canvas.getContext('2d');

            if (canvas.width !== canvas.offsetWidth || canvas.height !== canvas.offsetHeight) {
                canvas.width = canvas.offsetWidth;
                canvas.height = canvas.offsetHeight;
            }
            ctx.save();
            ctx.clearRect(0, 0, canvas.width, canvas.height);
            if (results.poseLandmarks && window.drawConnectors) {
                //绘制骨架连线（绿色）
                window.drawConnectors(ctx, results.poseLandmarks, POSE_CONNECTIONS, { color: '#00FF00', lineWidth: 2 });
                //绘制关键点（红色）
                window.drawLandmarks(ctx, results.poseLandmarks, { color: '#FF0000', lineWidth: 1, radius: 3 });
            }
            ctx.restore();
        },

        clearCanvas(id) {
            const c = document.getElementById(id);
            if (c) c.getContext('2d').clearRect(0, 0, c.width, c.height);
        }
    };

    window.AIContext = AIContext;

    // 3. 绑定开关事件
    function bind() {
        const toggle = document.getElementById('aiToggle');
        if (toggle) {
            const newToggle = toggle.cloneNode(true);
            toggle.parentNode.replaceChild(newToggle, toggle);
            newToggle.addEventListener('change', (e) => {
                if(e.target.checked) AIContext.start();
                else AIContext.stop();
            });
        }
    }
    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', bind);
    else bind();
})();
