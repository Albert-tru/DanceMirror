#!/bin/bash

# PWA 图标生成脚本
echo "🎨 DanceMirror PWA 图标生成器"
echo "================================"

# 检查 ImageMagick
if ! command -v convert &> /dev/null; then
    echo "❌ 未找到 ImageMagick，正在安装..."
    sudo apt update && sudo apt install -y imagemagick
fi

# 创建 SVG 图标
cat > /tmp/dancemirror-icon.svg << 'EOF'
<svg width="512" height="512" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="grad1" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" style="stop-color:#667eea;stop-opacity:1" />
      <stop offset="100%" style="stop-color:#764ba2;stop-opacity:1" />
    </linearGradient>
  </defs>
  <rect width="512" height="512" rx="100" fill="url(#grad1)"/>
  <g transform="translate(256,256)">
    <circle cx="0" cy="-80" r="35" fill="white"/>
    <path d="M 0,-45 L 0,50" stroke="white" stroke-width="20" stroke-linecap="round"/>
    <path d="M 0,0 L -60,-40" stroke="white" stroke-width="18" stroke-linecap="round"/>
    <path d="M 0,0 L 60,40" stroke="white" stroke-width="18" stroke-linecap="round"/>
    <path d="M 0,50 L -40,120" stroke="white" stroke-width="18" stroke-linecap="round"/>
    <path d="M 0,50 L 40,120" stroke="white" stroke-width="18" stroke-linecap="round"/>
  </g>
</svg>
EOF

echo "✅ SVG 图标已创建"

# 转换为 PNG
convert -background none /tmp/dancemirror-icon.svg -resize 512x512 /tmp/icon-base.png
echo "✅ 基础图标已生成"

# 生成各种尺寸
SIZES=(72 96 128 144 152 192 384 512)
OUTPUT_DIR="$HOME/go/DanceMirror/static/icons"

for size in "${SIZES[@]}"; do
    convert /tmp/icon-base.png -resize ${size}x${size} "$OUTPUT_DIR/icon-${size}x${size}.png"
    echo "✅ icon-${size}x${size}.png"
done

# favicon 和 Apple icon
convert /tmp/icon-base.png -resize 32x32 $HOME/go/DanceMirror/static/favicon.ico
convert /tmp/icon-base.png -resize 180x180 $HOME/go/DanceMirror/static/apple-touch-icon.png

echo "🎉 所有图标已生成！"
