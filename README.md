# HueLogs
Minimilistic Log Dashboard for personal projects. Setting up a dashboard for projects at initial stages will save you hastle to integrate and use log aggregators. HueLogs is minimal and quick log monitoring setup for projects at small scale.

## How to run
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
