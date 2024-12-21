from flask import Flask, jsonify, render_template, request
from flask_socketio import SocketIO, emit
import sqlite3
import os

app = Flask(__name__)
app.config["SECRET_KEY"] = "your-secret-key"
socketio = SocketIO(app)

# Path to the SQLite database
DB_PATH = "logs.db"

# Initialize the database
if not os.path.exists(DB_PATH):
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute('''
    CREATE TABLE logs (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        timestamp TEXT NOT NULL,
        message TEXT NOT NULL
    )
    ''')
    conn.commit()
    conn.close()

@app.route("/")
def dashboard():
    return render_template("index.html")

@app.route("/api/logs")
def get_logs():
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute("SELECT timestamp, message FROM logs ORDER BY id DESC LIMIT 100")
    logs = cursor.fetchall()
    conn.close()

    log_list = [{"timestamp": log[0], "message": log[1]} for log in logs]
    return jsonify({"logs": log_list})

@app.route("/api/add-log", methods=["POST"])
def add_log():
    data = request.get_json()
    message = data.get("message")

    if not message:
        return jsonify({"error": "Message is required"}), 400

    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute(
        "INSERT INTO logs (timestamp, message) VALUES (datetime('now'), ?)", (message,)
    )
    conn.commit()

    # Emit the new log to connected clients
    new_log = {"timestamp": cursor.execute("SELECT datetime('now')").fetchone()[0], "message": message}
    socketio.emit("new_log", new_log)

    conn.close()
    return jsonify({"success": True, "message": "Log added successfully"})


@app.route("/api/search-logs", methods=["GET"])
def search_logs():
    query = request.args.get("q", "")

    if not query:
        return jsonify({"error": "Search query is required"}), 400

    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute("SELECT timestamp, message FROM logs WHERE message LIKE ? ORDER BY id DESC", (f"%{query}%",))
    logs = cursor.fetchall()
    conn.close()

    log_list = [{"timestamp": log[0], "message": log[1]} for log in logs]
    return jsonify({"logs": log_list})



@socketio.on("connect")
def handle_connect():
    print("Client connected")

@socketio.on("disconnect")
def handle_disconnect():
    print("Client disconnected")

if __name__ == "__main__":
    socketio.run(app, debug=True, host="0.0.0.0", port=5000)
