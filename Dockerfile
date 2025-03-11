FROM python:3.11-slim

# Set work directory
WORKDIR /app

# Copy your app code
COPY . /app

# Install dependencies
RUN pip install -r requirements.txt

# Expose the app port
EXPOSE 5000

VOLUME ["/app/data/"]

# Run the app with Gunicorn and WebSocket support
CMD ["gunicorn", "-b", "0.0.0.0:5000", "--worker-class", "eventlet", "app:app"]



# Run the app with Gunicorn and WebSocket support, add the --log-level flag to change the log level
# CMD ["gunicorn", "-b", "0.0.0.0:5000", "--worker-class", "eventlet", "--log-level", "debug", "app:app"]

