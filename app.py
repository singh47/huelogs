from flask import Flask, jsonify, render_template, request, abort, session, redirect, url_for
from flask_socketio import SocketIO, emit
import sqlite3
import os, secrets
from utils import require_api_key
from datetime import timedelta

# API Key Configuration
API_KEY = os.environ.get("LOGGER_API_KEY", secrets.token_hex(32))  # Use env var in production
print(f"API Key (save this securely): {API_KEY}") # could remove this in prod

app = Flask(__name__)
app.config["SECRET_KEY"] = "your-secret-key"
app.config["SESSION_COOKIE_SECURE"] = True  # Disable for http - in development
app.config["SESSION_COOKIE_HTTPONLY"] = True
app.config["SESSION_COOKIE_SAMESITE"] = "Lax"
app.config["PERMANENT_SESSION_LIFETIME"] = timedelta(days=7)

socketio = SocketIO(app)

# Path to volume mapping in docker-compose.yml
volume_path = "/app/data"
DB_PATH = os.path.join(volume_path, "logs.db")

# Initialize the database
def init_db():
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute('''
        CREATE TABLE IF NOT EXISTS logs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            timestamp TEXT NOT NULL,
            message TEXT NOT NULL,
            service_name TEXT
        )
    ''')
    # Check if service_name column exists and add it if not - we added this feature later
    cursor.execute("PRAGMA table_info(logs)")
    columns = [col[1] for col in cursor.fetchall()]
    if "service_name" not in columns:
        cursor.execute("ALTER TABLE logs ADD COLUMN service_name TEXT")
    conn.commit()
    conn.close()

if not os.path.exists(DB_PATH):
    init_db()
else:
    init_db()

# Login route
@app.route("/login", methods=["GET", "POST"])
def login():
    if request.method == "POST":
        api_key = request.form.get("api_key")
        if api_key == API_KEY:
            session.permanent = True
            session["api_key"] = api_key
            return redirect(url_for("dashboard"))
        return render_template("login.html", error="Invalid API key")
    return render_template("login.html", error=None)

# Logout route
@app.route("/logout")
def logout():
    session.pop("api_key", None)
    return redirect(url_for("login"))

# Dashboard route
@app.route("/")
@require_api_key(API_KEY)
def dashboard():
    return render_template("index.html")

@app.route("/api/logs")
@require_api_key(API_KEY)
def get_logs():
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute("SELECT timestamp, message, service_name FROM logs ORDER BY id DESC LIMIT 100")
    logs = cursor.fetchall()
    conn.close()

    log_list = [{"timestamp": log[0], "message": log[1], "service_name": log[2]} for log in logs]
    return jsonify({"logs": log_list})

@app.route("/api/add-log", methods=["POST"])
@require_api_key(API_KEY)
def add_log():
    data = request.get_json()
    message = data.get("message")
    service_name = data.get("service_name")

    if not message:
        return jsonify({"error": "Message is required"}), 400

    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute(
        "INSERT INTO logs (timestamp, message, service_name) VALUES (datetime('now'), ?, ?)",
        (message, service_name)
    )
    conn.commit()

    # Emit the new log to connected clients
    new_log = {
        "timestamp": cursor.execute("SELECT datetime('now')").fetchone()[0],
        "message": message,
        "service_name": service_name
    }
    socketio.emit("new_log", new_log)

    conn.close()
    return jsonify({"success": True, "message": "Log added successfully"})

@app.route("/api/search-logs", methods=["GET"])
@require_api_key(API_KEY)
def search_logs():
    query = request.args.get("q", "")

    if not query:
        return jsonify({"error": "Search query is required"}), 400

    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute(
        "SELECT timestamp, message, service_name FROM logs WHERE message LIKE ? OR service_name LIKE ? ORDER BY id DESC",
        (f"%{query}%", f"%{query}%")
    )
    logs = cursor.fetchall()
    conn.close()

    log_list = [{"timestamp": log[0], "message": log[1], "service_name": log[2]} for log in logs]
    return jsonify({"logs": log_list})

# Error Handling for Unauthorized Access
@app.errorhandler(401)
def unauthorized(e):
    # For HTML requests, redirect to login
    if request.accept_mimetypes.accept_html:
        return redirect(url_for("login"))
    # For API requests, return JSON error
    return jsonify({"error": str(e)}), 401

@socketio.on("connect")
def handle_connect():
    # Check session for SocketIO connection
    if session.get("api_key") != API_KEY:
        print("Unauthorized SocketIO connection attempt")
        return False  # Reject connection
    print("Client connected")

@socketio.on("disconnect")
def handle_disconnect():
    print("Client disconnected")

if __name__ == "__main__":
    socketio.run(app, debug=True, host="0.0.0.0", port=5000)