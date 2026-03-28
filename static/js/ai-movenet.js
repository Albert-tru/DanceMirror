(function () {
    console.log("🚀 正在加载 MoveNet...");

    const CONNECTIONS = [
        [0, 1], [0, 2], [1, 3], [2, 4],
        [5, 6], [5, 7], [7, 9], [6, 8], [8, 10],
        [5, 11], [6, 12], [11, 12],
        [11, 13], [13, 15], [12, 14], [14, 16]
    ];

    window.AIContext = {
        isEnabled: false,
        detector: null,
        isInitialized: false,
        lastPoses: { teach: null, user: null },

        async init() {
            if (this.isInitialized) return;

            const hint = document.getElementById('aiStatusHint');
            if (hint) {
                hint.textContent = "⏳ 加载 MoveNet 模型中...";
                hint.className = "ai-status-hint loading";
            }

            await window.tf.ready();
            console.log("✅ TFJS Backend:", window.tf.getBackend());

            this.detector = await window.poseDetection.createDetector(
                window.poseDetection.SupportedModels.MoveNet,
                { modelType: window.poseDetection.movenet.modelType.SINGLEPOSE_THUNDER }
            );

            this.isInitialized = true;
            console.log("✅ MoveNet 加载完成");
        },

        async start() {
            const toggle = document.getElementById('aiToggle');
            const hint = document.getElementById('aiStatusHint');
            if (toggle) toggle.disabled = true;

            try {
                await this.init();
                this.isEnabled = true;
                this.loop();
                if (hint) {
                    hint.textContent = "✅ AI 运行中";
                    hint.className = "ai-status-hint ready";
                }
            } catch (e) {
                console.error("AI 启动失败:", e);
                alert("启动失败: " + e.message);
                if (toggle) toggle.checked = false;
            } finally {
                if (toggle) toggle.disabled = false;
            }
        },

        pushRealtimeSuggestion(score, userKeypoints) {
    if (!window.DanmakuManager || !Array.isArray(userKeypoints)) return;

    const now = Date.now();
    this._lastRealtimeTs = this._lastRealtimeTs || 0;
    this._lastRealtimeType = this._lastRealtimeType || '';

    // 1.8s 冷却，防止刷屏
    if (now - this._lastRealtimeTs < 1800) return;

    const kp = userKeypoints;
    const ls = kp[5], rs = kp[6], lw = kp[9], rw = kp[10];
    if (!ls || !rs || !lw || !rw) return;
    if ((ls.score ?? 1) < 0.3 || (rs.score ?? 1) < 0.3 || (lw.score ?? 1) < 0.3 || (rw.score ?? 1) < 0.3) return;

    let type = 'pace';
    let text = '节奏稳住，继续保持 👏';

    // 手腕低于肩太多
    const rightLow = (rw.y - rs.y) > 0.08;
    const leftLow  = (lw.y - ls.y) > 0.08;
    const shoulderTilt = Math.abs(ls.y - rs.y);

    if (rightLow && leftLow) {
        type = 'arms_both_low';
        text = '双臂再抬高一点，动作会更打开 💪';
    } else if (rightLow) {
        type = 'right_arm_low';
        text = '右臂偏低，右手再提一点 ↗';
    } else if (leftLow) {
        type = 'left_arm_low';
        text = '左臂偏低，左手再提一点 ↖';
    } else if (shoulderTilt > 0.06) {
        type = 'torso_tilt';
        text = '身体有点歪，核心收紧站稳';
    } else if (score > 85) {
        type = 'good';
        text = '这一段很稳！继续保持 🔥';
    }

    // 避免连续重复同类提示
    if (type === this._lastRealtimeType) return;

    const tVideo = document.body.classList.contains('mode-single')
        ? document.getElementById('singleVideo')
        : document.getElementById('userVideo');

    const t = tVideo && Number.isFinite(tVideo.currentTime) ? tVideo.currentTime : 0;

    window.DanmakuManager.suggestions.push({
        time: t + 0.2,
        text,
        type: (type === 'good' ? 'good' : 'warn'),
        isSent: false
    });

    this._lastRealtimeTs = now;
    this._lastRealtimeType = type;
},

        stop() {
            this.isEnabled = false;
            this.lastPoses = { teach: null, user: null };
            ['singleSkeletonCanvas', 'teachSkeletonCanvas', 'userSkeletonCanvas'].forEach(id => {
                const c = document.getElementById(id);
                if (c) c.getContext('2d').clearRect(0, 0, c.width, c.height);
            });
            const hint = document.getElementById('aiStatusHint');
            if (hint) hint.textContent = "已关闭";
            const board = document.getElementById('aiScoreBoard');
            if (board) board.classList.remove('active');
        },

        async loop() {
            if (!this.isEnabled) return;

            const isSingle = document.body.classList.contains('mode-single');
            const tasks = [
                {
                    type: 'teach',
                    vid: isSingle ? 'singleVideo' : 'originalVideo',
                    cid: isSingle ? 'singleSkeletonCanvas' : 'teachSkeletonCanvas'
                },
                { type: 'user', vid: 'userVideo', cid: 'userSkeletonCanvas' }
            ];

            for (const task of tasks) {
                if (isSingle && task.vid === 'userVideo') continue;

                const video = document.getElementById(task.vid);
                const canvas = document.getElementById(task.cid);

                if (video && canvas && !video.paused && video.readyState >= 2) {
                    if (canvas.width !== video.videoWidth || canvas.height !== video.videoHeight) {
                        canvas.width = video.videoWidth || video.offsetWidth;
                        canvas.height = video.videoHeight || video.offsetHeight;
                    }

                    try {
                        const poses = await this.detector.estimatePoses(video);
                        const ctx = canvas.getContext('2d');
                        ctx.clearRect(0, 0, canvas.width, canvas.height);

                        if (poses.length > 0) {
                            this.drawSkeleton(ctx, poses[0].keypoints);
                            this.lastPoses[task.type] = poses[0].keypoints;
                        } else {
                            this.lastPoses[task.type] = null;
                        }
                    } catch (e) {
                        console.warn("检测帧失败:", e);
                    }
                }
            }

            this.updateScore();
            requestAnimationFrame(() => this.loop());
        },

        updateScore() {
            const board = document.getElementById('aiScoreBoard');
            const scoreEl = document.getElementById('liveScore');
            const commentEl = document.getElementById('scoreComment');

            if (!this.lastPoses.teach || !this.lastPoses.user) {
                if (commentEl) commentEl.textContent = "准备中...";
                return;
            }

            if (board) board.classList.add('active');

            const score = Math.floor(this.calculateScore(this.lastPoses.teach, this.lastPoses.user) * 100);
            if (scoreEl) scoreEl.textContent = score;
            if (commentEl) {
                if (score > 85) commentEl.textContent = "太完美了! 🔥";
                else if (score > 70) commentEl.textContent = "动作很准 👍";
                else if (score > 50) commentEl.textContent = "继续加油 💪";
                else commentEl.textContent = "跟上节奏 🎵";
            }
            this.pushRealtimeSuggestion(score, this.lastPoses.user);
        },

        calculateScore(kp1, kp2) {
            const coreIndices = [5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16];
            let totalSim = 0, count = 0;

            coreIndices.forEach(idx => {
                const p1 = kp1[idx], p2 = kp2[idx];
                if (p1?.score > 0.3 && p2?.score > 0.3) {
                    const dx = Math.abs(p1.x - p2.x) / 300;
                    const dy = Math.abs(p1.y - p2.y) / 300;
                    totalSim += 1 - Math.min(1, (dx + dy) / 2);
                    count++;
                }
            });

            return count > 0 ? totalSim / count : 0;
        },

        drawSkeleton(ctx, keypoints) {
            ctx.strokeStyle = '#00FF00';
            ctx.lineWidth = 3;

            CONNECTIONS.forEach(([i, j]) => {
                const kp1 = keypoints[i], kp2 = keypoints[j];
                if (kp1?.score > 0.3 && kp2?.score > 0.3) {
                    ctx.beginPath();
                    ctx.moveTo(kp1.x, kp1.y);
                    ctx.lineTo(kp2.x, kp2.y);
                    ctx.stroke();
                }
            });

            keypoints.forEach(kp => {
                if (kp?.score > 0.3) {
                    ctx.fillStyle = '#FF0000';
                    ctx.beginPath();
                    ctx.arc(kp.x, kp.y, 5, 0, 2 * Math.PI);
                    ctx.fill();
                }
            });
        }
    };

    // 绑定开关
    function initToggle() {
        const toggle = document.getElementById('aiToggle');
        if (!toggle) return;

        toggle.checked = false;
        toggle.onclick = async (e) => {
            if (e.target.checked) {
                await window.AIContext.start();
            } else {
                window.AIContext.stop();
            }
        };

        console.log("✅ AI 开关已绑定");
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => setTimeout(initToggle, 500));
    } else {
        setTimeout(initToggle, 500);
    }
})();