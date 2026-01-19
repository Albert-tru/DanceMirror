import sys

TARGET_FILE = 'static/video-player.html'

with open(TARGET_FILE, 'r', encoding='utf-8') as f:
    content = f.read()

CAMERA_LOGIC = """
    <script>
        // Global state for camera (legacy support + manual toggle)
        let currentStream = null;

        async function toggleCamera() {
            const btn = document.getElementById('actionRecord'); 
            
            if (currentStream) {
                // Stop camera
                currentStream.getTracks().forEach(t => t.stop());
                currentStream = null;
                if (typeof userVideo !== 'undefined' && userVideo) {
                    userVideo.srcObject = null;
                }
                return;
            }

            try {
                if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
                    throw new Error("浏览器不支持摄像头");
                }
                const stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
                currentStream = stream;
                
                if (typeof userVideo !== 'undefined' && userVideo) {
                    userVideo.srcObject = stream;
                    userVideo.muted = true;
                    userVideo.play().catch(e => console.warn(e));
                }
            } catch (e) {
                console.error("Camera access failed:", e);
                alert("无法访问摄像头: " + e.message);
            }
        }
    </script>
"""

# Insert before the ai-scoring.js import
INSERT_MARKER = '<script src="/static/js/ai-scoring.js"></script>'

if 'function toggleCamera' not in content:
    if INSERT_MARKER in content:
        content = content.replace(INSERT_MARKER, CAMERA_LOGIC + '\n' + INSERT_MARKER)
        print("Injected toggleCamera logic")
    else:
        print("Marker not found, appending to body end")
        if '</body>' in content:
            content = content.replace('</body>', CAMERA_LOGIC + '\n' + INSERT_MARKER + '\n</body>')

with open(TARGET_FILE, 'w', encoding='utf-8') as f:
    f.write(content)
