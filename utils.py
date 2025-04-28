from functools import wraps
from flask import request, abort, session

def require_api_key(api_key):
    def decorator(f):
        @wraps(f)
        def decorated_function(*args, **kwargs):
            # session first
            if session.get("api_key") == api_key:
                return f(*args, **kwargs)
            # fallback to headers
            provided_key = request.headers.get("X-API-Key")
            if not provided_key or provided_key != api_key:
                abort(401, description="Unauthorized: Invalid or missing API key")
            return f(*args, **kwargs)
        return decorated_function
    return decorator