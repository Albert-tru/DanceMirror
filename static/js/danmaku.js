window.DanmakuManager = {
    suggestions: [],
    prevTime: 0,

    send(text, type = "normal", containerId) {
        const container = document.getElementById(containerId);
        if (!container) {
            console.warn(`❌ 弹幕容器未找到: ${containerId}`);
            return;
        }

        const item = document.createElement("div");
        item.className = `danmaku-item danmaku-${type}`;
        item.textContent = text;

        if (type !== 'info') {
            item.style.top = (Math.floor(Math.random() * 70) + 10) + "%";
        }

        container.appendChild(item);
        item.addEventListener("animationend", () => item.remove());
    },

    update(videoCurrentTime, containerId) {
        if (videoCurrentTime < this.prevTime) {
            this.suggestions.forEach(s => s.isSent = false);
        }
        this.prevTime = videoCurrentTime;

        this.suggestions.forEach(item => {
            if (!item.isSent &&
                videoCurrentTime >= item.time &&
                videoCurrentTime < item.time + 1.0) {
                this.send(item.text, item.type, containerId);
                item.isSent = true;
            }
        });
    }
};

console.log("✅ DanmakuManager 已加载:", window.DanmakuManager);