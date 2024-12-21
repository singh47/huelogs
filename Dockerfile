FROM python:3.11-slim

# Set work directory
WORKDIR /app

# Copy your app code
COPY . /app

# Install dependencies
RUN pip install --no-cache-dir flask flask-socketio eventlet gunicorn

# Expose the app port
EXPOSE 5000

# Run the app with Gunicorn and WebSocket support
CMD ["gunicorn", "-b", "0.0.0.0:5000", "--worker-class", "eventlet", "app:app"]
