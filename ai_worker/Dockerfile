FROM python:3.9-slim

# 安装系统依赖（OpenCV 需要这些）
RUN apt-get update && apt-get install -y \
    libgl1 \
    libglib2.0-0 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 安装 Python 包
COPY requirements.txt .
RUN pip install --no-cache-dir \
    --default-timeout=120 \
    --retries 5 \
    -i https://mirrors.aliyun.com/pypi/simple/ \
    -r requirements.txt

# 拷贝代码
COPY woker.py .

CMD ["python", "-u", "woker.py"]