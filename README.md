# HueLogs
Minimilistic Log Dashboard for personal projects. Setting up a dashboard for projects at initial stages will save you hastle to integrate and use log aggregators. HueLogs is minimal and quick log monitoring setup for projects at small scale.

![ Hue Logs Dashboard](https://github.com/user-attachments/assets/66eaf21c-3511-4518-981a-fc995ce3f547)


## Deploy with Docker Compose

**1. Set secrets in `docker-compose.yml`** — change all three values before starting:

| Variable | Description |
|---|---|
| `LOGGER_API_KEY` | Key used to authenticate log ingestion and dashboard login |
| `SECRET_KEY` | Random string used to sign session cookies |
| `POSTGRES_PASSWORD` | DB password — update both the `timescaledb` env and the `DATABASE_URL` in `hue-logs` to match |

If you're serving over HTTPS, also set `SESSION_COOKIE_SECURE=true` in the `hue-logs` service.

**2. Start the stack:**

```bash
docker compose up --build -d
```

TimescaleDB and Redis must pass their health checks before the app starts — this is handled automatically.

**3. Open the dashboard:**

```
http://<your-host>:5000/
```

Log in with the `LOGGER_API_KEY` you set above.

---

## How to run locally
```
1. docker compose up --build

2. open this in browser (default): http://127.0.0.1:5000/
```

## How to send logs
You may directly call an API to send logs, or use a package:

### 1. CURL

```
curl -X POST http://127.0.0.1:5000/api/add-log \
-H "Content-Type: application/json" \
-d '{"message": "This is a test log"}'
```

### 2. Python package
install https://github.com/singh47/hue_logger_py_client


