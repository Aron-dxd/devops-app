from flask import Flask
import os

app = Flask(__name__)

@app.route("/")
def hello():
    # We use an environment variable for the version so it's easy to change later
    version = os.getenv("APP_VERSION", "v1")
    return f"<h1>Hello World - {version}</h1>\n"

if __name__ == "__main__":
    # Standard web port 80. '0.0.0.0' allows external access.
    app.run(host="0.0.0.0", port=80)
