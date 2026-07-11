import os

os.environ.setdefault("SLAVE_NODE_NAME", "Slave4")
os.environ.setdefault("SLAVE2_PORT", os.environ.get("SLAVE4_PORT", "5001"))

from app import PORT, app, initialize_database


if __name__ == "__main__":
    initialize_database()
    app.run(host="0.0.0.0", port=PORT)
