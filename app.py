import json
import logging
import os
import socket
import struct
import urllib.error
import urllib.request
import threading
from flask import Flask, jsonify, request, Response

app = Flask(__name__)
NODE_NAME = os.environ.get("SLAVE_NODE_NAME", "Slave2")
PORT = int(os.environ.get("SLAVE2_PORT", "5000"))
VALID_OPERATIONS = {"insert", "update", "delete"}

# Configurable master URLs from environment
MAIN_MASTER_URL = os.environ.get("MASTER_URL", "http://localhost:8080")
FAILOVER_MASTER_URL = os.environ.get("SLAVE1_URL", "http://localhost:8081")

SLAVE1_URL = os.environ.get("SLAVE1_URL", "http://localhost:8081")
SLAVE2_URL = os.environ.get("SLAVE2_URL", "http://localhost:5000")
SLAVE3_URL = os.environ.get("SLAVE3_URL", "http://localhost:8082")
SLAVE4_URL = os.environ.get("SLAVE4_URL", "http://localhost:5001")

ALL_NODES = [
    {"name": "Master", "url": MAIN_MASTER_URL},
    {"name": "Slave1", "url": SLAVE1_URL},
    {"name": "Slave2", "url": SLAVE2_URL},
    {"name": "Slave3", "url": SLAVE3_URL},
    {"name": "Slave4", "url": SLAVE4_URL},
]


def _get_self_url():
    for node in ALL_NODES:
        if node["name"] == NODE_NAME:
            return node["url"]
    return f"http://localhost:{PORT}"

SELF_URL = _get_self_url()

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(message)s",
)
logger = logging.getLogger(NODE_NAME.lower())

# In-memory database storage protected by a lock
clients_db = {}
db_lock = threading.Lock()

next_id = 1
next_id_lock = threading.Lock()


def initialize_database():
    logger.info("ensured in-memory storage initialized for %s", NODE_NAME)


def _wants_html():
    """Return True when the caller is a regular browser expecting HTML."""
    accept = request.headers.get("Accept", "")
    return "text/html" in accept


def _render_html(clients):
    """Render the professional dark-themed HTML page for browser clients."""
    if clients:
        rows = ""
        for cl in clients:
            rows += (
                f'<tr>'
                f'<td>{cl.get("id", "")}</td>'
                f'<td>{cl.get("name", "")}</td>'
                f'<td>{cl.get("national_id", "")}</td>'
                f'<td>{cl.get("phone", "")}</td>'
                f'<td>{cl.get("email", "")}</td>'
                f'<td class="balance">{cl.get("balance", 0):.2f} EGP</td>'
                f'</tr>'
            )
    else:
        rows = '<tr><td colspan="6" style="text-align:center;color:#94a3b8;padding:40px 0;font-size:15px;">No clients found in RAM store.</td></tr>'

    return HTML_TEMPLATE.format(
        node_name=NODE_NAME,
        count=len(clients),
        rows=rows,
    )


HTML_TEMPLATE = """<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Distributed Banking System — {node_name}</title>
<meta name="description" content="In-memory distributed banking client database with real-time replication across five nodes.">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800;900&display=swap" rel="stylesheet">
<style>
*,*::before,*::after{{box-sizing:border-box;margin:0;padding:0}}
:root{{
  --bg-primary:#07111f;
  --bg-card:rgba(15,23,42,.92);
  --bg-row-hover:rgba(56,189,248,.06);
  --border:rgba(148,163,184,.14);
  --accent:#38bdf8;
  --accent-glow:rgba(56,189,248,.18);
  --text-primary:#f1f5f9;
  --text-secondary:#94a3b8;
  --text-muted:#64748b;
  --green:#34d399;
  --green-bg:rgba(52,211,153,.12);
  --purple:#a78bfa;
}}
body{{
  font-family:'Inter',system-ui,-apple-system,sans-serif;
  background:
    radial-gradient(ellipse at 20% 0%,rgba(56,189,248,.10),transparent 50%),
    radial-gradient(ellipse at 80% 0%,rgba(167,139,250,.08),transparent 50%),
    var(--bg-primary);
  color:var(--text-primary);
  min-height:100vh;
  line-height:1.5;
}}
.wrapper{{
  max-width:1280px;
  margin:0 auto;
  padding:32px 24px 64px;
}}
.header{{
  display:flex;
  align-items:center;
  justify-content:space-between;
  flex-wrap:wrap;
  gap:16px;
  margin-bottom:32px;
}}
.logo{{
  display:flex;align-items:center;gap:14px;
}}
.logo-icon{{
  width:48px;height:48px;
  background:linear-gradient(135deg,var(--accent),var(--purple));
  border-radius:14px;
  display:flex;align-items:center;justify-content:center;
  font-size:22px;font-weight:900;color:#fff;
  box-shadow:0 8px 24px var(--accent-glow);
}}
.logo-text h1{{
  font-size:22px;font-weight:800;letter-spacing:-.3px;
  background:linear-gradient(135deg,#fff,var(--accent));
  -webkit-background-clip:text;-webkit-text-fill-color:transparent;
}}
.logo-text p{{
  font-size:13px;color:var(--text-muted);margin-top:2px;
}}
.header-badges{{display:flex;gap:10px;flex-wrap:wrap;}}
.badge{{
  padding:7px 16px;
  border-radius:999px;
  font-size:12px;font-weight:700;
  letter-spacing:.3px;
  border:1px solid;
}}
.badge-node{{
  background:rgba(56,189,248,.08);
  color:var(--accent);
  border-color:rgba(56,189,248,.22);
}}
.badge-ram{{
  background:var(--green-bg);
  color:var(--green);
  border-color:rgba(52,211,153,.25);
}}
.badge-count{{
  background:rgba(167,139,250,.10);
  color:var(--purple);
  border-color:rgba(167,139,250,.25);
}}
.stats{{
  display:grid;
  grid-template-columns:repeat(auto-fit,minmax(200px,1fr));
  gap:16px;
  margin-bottom:32px;
}}
.stat-card{{
  background:var(--bg-card);
  border:1px solid var(--border);
  border-radius:16px;
  padding:20px 22px;
  position:relative;
  overflow:hidden;
  transition:border-color .25s,box-shadow .25s;
}}
.stat-card:hover{{
  border-color:rgba(56,189,248,.28);
  box-shadow:0 12px 40px rgba(0,0,0,.3);
}}
.stat-card::before{{
  content:'';position:absolute;top:0;left:0;right:0;height:3px;
  background:linear-gradient(90deg,var(--accent),var(--purple));
  opacity:.6;border-radius:16px 16px 0 0;
}}
.stat-label{{font-size:12px;color:var(--text-muted);text-transform:uppercase;letter-spacing:.8px;font-weight:600;}}
.stat-value{{font-size:28px;font-weight:800;margin-top:6px;color:var(--text-primary);}}
.stat-sub{{font-size:12px;color:var(--text-secondary);margin-top:4px;}}
.table-card{{
  background:var(--bg-card);
  border:1px solid var(--border);
  border-radius:20px;
  overflow:hidden;
  box-shadow:0 20px 60px rgba(0,0,0,.28);
}}
.table-header{{
  padding:22px 28px;
  border-bottom:1px solid var(--border);
  display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:12px;
}}
.table-header h2{{
  font-size:18px;font-weight:700;
}}
.table-header .hint{{
  font-size:12px;color:var(--text-muted);
  background:rgba(100,116,139,.12);
  padding:5px 12px;border-radius:8px;
}}
table{{
  width:100%;border-collapse:collapse;
}}
thead{{
  background:rgba(15,23,42,.5);
}}
th{{
  padding:14px 20px;
  text-align:left;
  font-size:11px;
  text-transform:uppercase;
  letter-spacing:.8px;
  color:var(--text-muted);
  font-weight:700;
  border-bottom:1px solid var(--border);
  white-space:nowrap;
}}
td{{
  padding:16px 20px;
  font-size:14px;
  border-bottom:1px solid var(--border);
  color:var(--text-secondary);
  transition:background .15s;
}}
tr:hover td{{
  background:var(--bg-row-hover);
  color:var(--text-primary);
}}
tr:last-child td{{border-bottom:none;}}
td.balance{{
  font-weight:700;
  color:var(--green);
  font-variant-numeric:tabular-nums;
}}
.footer{{
  text-align:center;
  margin-top:40px;
  padding:18px;
  font-size:12px;
  color:var(--text-muted);
}}
@media(max-width:768px){{
  .wrapper{{padding:20px 12px 40px;}}
  .header{{flex-direction:column;align-items:flex-start;}}
  td,th{{padding:10px 12px;font-size:13px;}}
  .stat-value{{font-size:22px;}}
}}
</style>
</head>
<body>
<div class="wrapper">
  <header class="header">
    <div class="logo">
      <div class="logo-icon">DB</div>
      <div class="logo-text">
        <h1>Distributed Banking System</h1>
        <p>In-Memory RAM Store &middot; Real-Time Replication</p>
      </div>
    </div>
    <div class="header-badges">
      <span class="badge badge-node">Node: {node_name}</span>
      <span class="badge badge-ram">RAM Store</span>
      <span class="badge badge-count">{count} Clients</span>
    </div>
  </header>

  <section class="stats">
    <div class="stat-card">
      <div class="stat-label">Total Clients</div>
      <div class="stat-value">{count}</div>
      <div class="stat-sub">Stored in RAM</div>
    </div>
    <div class="stat-card">
      <div class="stat-label">Active Node</div>
      <div class="stat-value">{node_name}</div>
      <div class="stat-sub">In-Memory Store</div>
    </div>
    <div class="stat-card">
      <div class="stat-label">Storage Engine</div>
      <div class="stat-value">RAM</div>
      <div class="stat-sub">In-Memory &middot; No Disk</div>
    </div>
    <div class="stat-card">
      <div class="stat-label">Architecture</div>
      <div class="stat-value">5 Nodes</div>
      <div class="stat-sub">Master-Slave Replication</div>
    </div>
  </section>

  <div class="table-card">
    <div class="table-header">
      <h2>Client Records</h2>
      <span class="hint">GET /clients &middot; JSON available via Accept: application/json</span>
    </div>
    <table>
      <thead>
        <tr>
          <th>ID</th>
          <th>Name</th>
          <th>National ID</th>
          <th>Phone</th>
          <th>Email</th>
          <th>Balance</th>
        </tr>
      </thead>
      <tbody>
        {rows}
      </tbody>
    </table>
  </div>

  <footer class="footer">
    Distributed Database Project &middot; University Demo &middot; In-Memory RAM Storage &middot; 5-Node Architecture
  </footer>
</div>
</body>
</html>"""


@app.get("/clients")
@app.get("/users")
def list_clients():
    with db_lock:
        clients = list(clients_db.values())
        # Sort clients by ID to simulate ORDER BY Id
        clients.sort(key=lambda c: c.get("id", 0))

    if _wants_html():
        html = _render_html(clients)
        return Response(html, status=200, content_type="text/html; charset=utf-8")

    return jsonify(clients), 200


# ---------------------------------------------------------------------------
# Election helpers
# ---------------------------------------------------------------------------

def _ip_to_numeric(ip_str):
    """Convert an IPv4 address string to a numeric value."""
    try:
        packed = socket.inet_aton(ip_str)
        return struct.unpack("!I", packed)[0]
    except (socket.error, OSError):
        return 0


def _extract_ip(raw_url):
    """Extract the hostname/IP from a URL."""
    from urllib.parse import urlparse
    parsed = urlparse(raw_url)
    host = parsed.hostname or ""
    if host == "" or host == "localhost":
        return "127.0.0.1"
    return host


def _node_name_order(name):
    """Deterministic priority for tie-breaking."""
    order = {"Master": 0, "Slave1": 1, "Slave2": 2, "Slave3": 3, "Slave4": 4}
    return order.get(name, 99)


def _check_node_health(node_url):
    """Check if a node is online by hitting GET /health."""
    try:
        req = urllib.request.Request(node_url + "/health", method="GET")
        with urllib.request.urlopen(req, timeout=2) as resp:
            return resp.status == 200
    except Exception:
        return False


def _elect_master(failed_url):
    """Elect the closest online node based on IP distance to the failed master."""
    failed_ip = _extract_ip(failed_url)
    failed_numeric = _ip_to_numeric(failed_ip)

    candidates = []
    for node in ALL_NODES:
        if node["url"] == failed_url:
            continue
        if node["name"] == NODE_NAME:
            online = True  # self is always online
        else:
            online = _check_node_health(node["url"])
        if not online:
            continue

        node_ip = _extract_ip(node["url"])
        node_numeric = _ip_to_numeric(node_ip)
        distance = abs(int(node_numeric) - int(failed_numeric))
        order = _node_name_order(node["name"])
        candidates.append((distance, order, node))

    if not candidates:
        return None

    candidates.sort(key=lambda x: (x[0], x[1]))
    return candidates[0][2]


# ---------------------------------------------------------------------------
# Health endpoint
# ---------------------------------------------------------------------------

@app.get("/health")
def health_check():
    return jsonify({
        "node": NODE_NAME,
        "role": "Slave",
        "status": "online",
        "url": SELF_URL,
    }), 200


# ---------------------------------------------------------------------------
# Replication to all (when acting as temp master)
# ---------------------------------------------------------------------------

def _replicate_to_all(operation, client):
    """Replicate an operation to all other online slave nodes."""
    payload = json.dumps({"operation": operation, "client": client}).encode("utf-8")

    for node in ALL_NODES:
        if node["name"] == NODE_NAME:
            continue  # don't replicate to self
        if node["name"] == "Master":
            continue  # Master has no /replicate endpoint

        target_url = node["url"] + "/replicate"
        target_name = node["name"]

        def send(url=target_url, name=target_name):
            try:
                req = urllib.request.Request(
                    url,
                    data=payload,
                    headers={"Content-Type": "application/json"},
                    method="POST",
                )
                with urllib.request.urlopen(req, timeout=3) as resp:
                    logger.info("replication succeeded to %s", name)
            except Exception as exc:
                logger.warning("replication failed to %s: %s", name, exc)

        threading.Thread(target=send, daemon=True).start()


# ---------------------------------------------------------------------------
# Local write handling (temp master mode)
# ---------------------------------------------------------------------------

def _handle_local_write(effective_id):
    """Handle a write locally when this node is elected as temporary master."""
    if request.method == "POST":
        return _local_create()
    elif request.method == "PUT":
        if effective_id is None:
            return jsonify({"error": "id required for update"}), 400
        return _local_update(effective_id)
    elif request.method == "DELETE":
        if effective_id is None:
            return jsonify({"error": "id required for delete"}), 400
        return _local_delete(effective_id)
    else:
        return jsonify({"error": "method not allowed"}), 405


def _validate_write_payload(operation, payload, client_id=None):
    if operation in {"create", "update"}:
        if not isinstance(payload, dict):
            return "request body must be valid JSON"

        name = payload.get("name")
        national_id = payload.get("national_id")
        phone = payload.get("phone")
        email = payload.get("email")
        balance = payload.get("balance", 0)

        if not isinstance(name, str) or not name or len(name) > 100:
            return "name is required"
        if not isinstance(national_id, str) or not national_id or len(national_id) > 14:
            return "national_id is required"
        if not isinstance(phone, str) or not phone or len(phone) > 15:
            return "phone is required"
        if not isinstance(email, str) or not email or len(email) > 150:
            return "email is required"
        if not isinstance(balance, (int, float)) or balance < 0:
            return "balance must be a non-negative number"

    if operation in {"update", "delete"}:
        if not isinstance(client_id, int) or client_id <= 0:
            return "client_id must be a positive integer"

    return None


def _build_approval_payload(effective_id):
    method_map = {
        "POST": "create",
        "PUT": "update",
        "DELETE": "delete",
    }
    operation = method_map.get(request.method)
    if operation is None:
        return None, "method not allowed"

    payload = request.get_json(silent=True) if request.method in {"POST", "PUT"} else None
    validation_error = _validate_write_payload(operation, payload, effective_id)
    if validation_error:
        return None, validation_error

    return {
        "operation": operation,
        "source_node": NODE_NAME,
        "client_id": effective_id,
        "client": payload,
    }, None


def _request_master_approval(effective_id):
    payload, error = _build_approval_payload(effective_id)
    if error:
        return {"approved": False, "message": error}, None

    logger.info("Slave requested approval from Master")

    request_data = json.dumps(payload).encode("utf-8")
    approval_request = urllib.request.Request(
        MAIN_MASTER_URL + "/approve-write",
        data=request_data,
        headers={"Content-Type": "application/json", "Accept": "application/json"},
        method="POST",
    )

    try:
        with urllib.request.urlopen(approval_request, timeout=3) as response:
            response_body = response.read()
            approval = json.loads(response_body.decode("utf-8")) if response_body else {}
    except urllib.error.HTTPError as exc:
        response_body = exc.read()
        approval = json.loads(response_body.decode("utf-8")) if response_body else {}
    except urllib.error.URLError as exc:
        return None, exc

    if approval.get("pending"):
        return approval, None
    if approval.get("approved"):
        logger.info("Master approved write")
    else:
        logger.info("Master rejected write")

    return approval, None


def _local_create():
    """Handle POST /clients locally as temp master."""
    global next_id
    payload = request.get_json(silent=True)
    if not payload:
        return jsonify({"error": "request body must be valid JSON"}), 400

    with next_id_lock:
        client_id = next_id
        next_id += 1

    client = {
        "id": client_id,
        "name": payload.get("name", ""),
        "national_id": payload.get("national_id", ""),
        "phone": payload.get("phone", ""),
        "email": payload.get("email", ""),
        "balance": float(payload.get("balance", 0)),
    }

    with db_lock:
        clients_db[client_id] = client

    _replicate_to_all("insert", client)
    logger.info("local create as temp master: client_id=%d", client_id)
    return jsonify(client), 201


def _local_update(client_id):
    """Handle PUT /clients/:id locally as temp master."""
    payload = request.get_json(silent=True)
    if not payload:
        return jsonify({"error": "request body must be valid JSON"}), 400

    with db_lock:
        if client_id not in clients_db:
            return jsonify({"error": "client not found"}), 404
        client = {
            "id": client_id,
            "name": payload.get("name", ""),
            "national_id": payload.get("national_id", ""),
            "phone": payload.get("phone", ""),
            "email": payload.get("email", ""),
            "balance": float(payload.get("balance", 0)),
        }
        clients_db[client_id] = client

    _replicate_to_all("update", client)
    logger.info("local update as temp master: client_id=%d", client_id)
    return jsonify(client), 200


def _local_delete(client_id):
    """Handle DELETE /clients/:id locally as temp master."""
    with db_lock:
        if client_id not in clients_db:
            return jsonify({"error": "client not found"}), 404
        client = clients_db.pop(client_id)

    _replicate_to_all("delete", client)
    logger.info("local delete as temp master: client_id=%d", client_id)
    return "", 204


# ---------------------------------------------------------------------------
# Write forwarding with dynamic failover
# ---------------------------------------------------------------------------

@app.post("/clients")
@app.put("/clients")
@app.put("/clients/<int:client_id>")
@app.delete("/clients")
@app.delete("/clients/<int:client_id>")
@app.post("/users")
@app.put("/users")
@app.put("/users/<int:user_id>")
@app.delete("/users")
@app.delete("/users/<int:user_id>")
def forward_write(client_id=None, user_id=None):
    effective_id = client_id or user_id

    approval, approval_error = _request_master_approval(effective_id)
    if approval_error is None and approval.get("pending"):
        return jsonify(approval), 202
    if approval_error is None and not approval.get("approved"):
        return jsonify({
            "approved": False,
            "message": approval.get("message", "Write rejected by Master"),
        }), 403

    # Try the main master first.
    logger.info(
        "forwarding direct %s request to %s from %s",
        request.method,
        request.path,
        request.remote_addr,
    )

    response, status, content_type, error = forward_to(MAIN_MASTER_URL)
    if error is None:
        logger.info("Write forwarded after approval")
        return response, status, {"Content-Type": content_type}

    if approval_error is not None:
        logger.warning("main master unavailable: %s, running failover election", approval_error)
    else:
        logger.warning("main master unavailable: %s, running failover election", error)

    # Dynamic election: find closest online node to the failed master.
    elected = _elect_master(MAIN_MASTER_URL)

    if elected is None:
        logger.error("no online node available for failover")
        return jsonify({"error": "no master node available"}), 503

    # If elected node is self, handle locally.
    if elected["name"] == NODE_NAME:
        logger.info("elected self (%s) as temporary master", NODE_NAME)
        return _handle_local_write(effective_id)

    # Forward to elected node.
    logger.info("forwarding write to elected master: %s at %s", elected["name"], elected["url"])
    response, status, content_type, error = forward_to(elected["url"])
    if error is None:
        return response, status, {"Content-Type": content_type}

    logger.error("elected master %s also unavailable: %s", elected["name"], error)
    return jsonify({"error": "no master node available"}), 503


# ---------------------------------------------------------------------------
# Replication endpoint (slave receives from master)
# ---------------------------------------------------------------------------

@app.post("/replicate")
def replicate():
    # The master sends this endpoint an operation and the client row to apply.
    payload = request.get_json(silent=True)
    if not payload:
        return jsonify({"error": "request body must be valid JSON"}), 400

    operation = payload.get("operation")
    client = payload.get("client") or payload.get("user") or {}
    validation_error = validate_replication_payload(operation, client)
    if validation_error:
        return jsonify({"error": validation_error}), 400

    try:
        if operation == "insert":
            insert_client(client)
        elif operation == "update":
            update_client(client)
        elif operation == "delete":
            delete_client(client["id"])
        else:
            return jsonify({"error": "unsupported replication operation"}), 400
    except Exception as exc:
        logger.exception(
            "replication failed operation=%s client_id=%s: %s",
            operation,
            client.get("id"),
            exc,
        )
        return jsonify({"error": "replication failed"}), 500

    logger.info("replication applied operation=%s client_id=%s", operation, client["id"])
    return jsonify({"status": "replicated"}), 200


def validate_replication_payload(operation, client):
    if operation not in VALID_OPERATIONS:
        return "operation must be insert, update, or delete"

    if not isinstance(client, dict):
        return "client must be an object"

    client_id = client.get("id")
    if not isinstance(client_id, int) or client_id <= 0:
        return "client.id must be a positive integer"

    if operation in {"insert", "update"}:
        name = client.get("name")
        national_id = client.get("national_id")
        phone = client.get("phone")
        email = client.get("email")
        balance = client.get("balance")
        if not isinstance(name, str) or not name or len(name) > 100:
            return "client.name is required and must be at most 100 characters"
        if not isinstance(national_id, str) or not national_id or len(national_id) > 14:
            return "client.national_id is required and must be at most 14 characters"
        if not isinstance(phone, str) or not phone or len(phone) > 15:
            return "client.phone is required and must be at most 15 characters"
        if not isinstance(email, str) or not email or len(email) > 150:
            return "client.email is required and must be at most 150 characters"
        if not isinstance(balance, (int, float)) or balance < 0:
            return "client.balance must be a non-negative number"

    return None


def forward_to(base_url):
    url = base_url + request.path
    if request.query_string:
        url += "?" + request.query_string.decode("utf-8")

    data = request.get_data()
    headers = {}
    if request.content_type:
        headers["Content-Type"] = request.content_type

    forwarded_request = urllib.request.Request(
        url,
        data=data,
        headers=headers,
        method=request.method,
    )

    try:
        with urllib.request.urlopen(forwarded_request, timeout=3) as response:
            body = response.read()
            content_type = response.headers.get("Content-Type", "application/json")
            return body, response.status, content_type, None
    except urllib.error.HTTPError as exc:
        body = exc.read()
        content_type = exc.headers.get("Content-Type", "application/json")
        return body, exc.code, content_type, None
    except urllib.error.URLError as exc:
        return None, None, None, exc


def insert_client(client):
    global next_id
    with db_lock:
        clients_db[client["id"]] = client
    with next_id_lock:
        if client["id"] >= next_id:
            next_id = client["id"] + 1


def update_client(client):
    with db_lock:
        if client["id"] not in clients_db:
            raise KeyError("Client not found")
        clients_db[client["id"]] = client


def delete_client(client_id):
    with db_lock:
        if client_id not in clients_db:
            raise KeyError("Client not found")
        del clients_db[client_id]


if __name__ == "__main__":
    initialize_database()
    app.run(host="0.0.0.0", port=PORT)
