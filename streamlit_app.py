import base64
import os
import socket
import struct
import time
from pathlib import Path

import pandas as pd
import requests
import streamlit as st


NODES = {
    "Master": {
        "url": os.environ.get("MASTER_URL", "http://localhost:8080"),
        "port": "8080",
        "database": "RAM Store",
        "role": "Master",
    },
    "Slave1": {
        "url": os.environ.get("SLAVE1_URL", "http://localhost:8081"),
        "port": "8081",
        "database": "RAM Store",
        "role": "Slave / Failover Master",
    },
    "Slave2": {
        "url": os.environ.get("SLAVE2_URL", "http://localhost:5000"),
        "port": "5000",
        "database": "RAM Store",
        "role": "Slave",
    },
    "Slave3": {
        "url": os.environ.get("SLAVE3_URL", "http://localhost:8082"),
        "port": "8082",
        "database": "RAM Store",
        "role": "Slave",
    },
    "Slave4": {
        "url": os.environ.get("SLAVE4_URL", "http://localhost:5001"),
        "port": "5001",
        "database": "RAM Store",
        "role": "Slave",
    },
}

MAIN_MASTER = "Master"
REQUEST_TIMEOUT = 3
HEADER_IMAGE = Path("assets/fares.jpeg")


def get_image_base64(image_path):
    with open(image_path, "rb") as img_file:
        return base64.b64encode(img_file.read()).decode()


def _ip_to_numeric(ip_str):
    try:
        packed = socket.inet_aton(ip_str)
        return struct.unpack("!I", packed)[0]
    except (socket.error, OSError):
        return 0


def _extract_ip(raw_url):
    from urllib.parse import urlparse

    parsed = urlparse(raw_url)
    host = parsed.hostname or ""
    if host == "" or host == "localhost":
        return "127.0.0.1"
    return host


def _node_name_order(name):
    order = {"Master": 0, "Slave1": 1, "Slave2": 2, "Slave3": 3, "Slave4": 4}
    return order.get(name, 99)


st.set_page_config(
    page_title="Distributed Database Dashboard",
    page_icon="🏦",
    layout="wide",
)


st.markdown(
    """
    <style>
        .stApp {
            background:
                radial-gradient(circle at top left, rgba(37, 99, 235, 0.18), transparent 34%),
                radial-gradient(circle at top right, rgba(20, 184, 166, 0.12), transparent 28%),
                #07111f;
            color: #e5edf7;
        }

        .block-container {
            padding-top: 1.4rem;
            padding-bottom: 2.2rem;
        }

        section[data-testid="stSidebar"] {
            background: #0b1628;
            border-right: 1px solid rgba(148, 163, 184, 0.16);
        }

        div[data-testid="stMetric"] {
            background: linear-gradient(180deg, rgba(15, 23, 42, 0.96), rgba(15, 23, 42, 0.72));
            border: 1px solid rgba(148, 163, 184, 0.18);
            border-radius: 16px;
            padding: 18px;
            box-shadow: 0 16px 38px rgba(0, 0, 0, 0.26);
        }

        div[data-testid="stMetricLabel"] {
            color: #93a4b8;
        }

        div[data-testid="stMetricValue"] {
            color: #f8fafc;
        }

       .cover-box {
    width: 100%;
    height: 260px;
    max-height: 260px;
    overflow: hidden;
    border-radius: 24px;
    margin: 0 0 24px 0;
    border: 1px solid rgba(125, 211, 252, 0.18);
    box-shadow: 0 18px 45px rgba(0, 0, 0, 0.35);
    background: #0f172a;
}

.cover-box img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    object-position: center 35%;
    display: block;
}

        .hero {
            background: linear-gradient(135deg, rgba(15, 23, 42, 0.98), rgba(30, 64, 175, 0.82));
            border: 1px solid rgba(125, 211, 252, 0.18);
            border-radius: 22px;
            padding: 30px 34px;
            margin-bottom: 22px;
            box-shadow: 0 22px 55px rgba(0, 0, 0, 0.35);
        }

        .hero h1 {
            margin: 0;
            font-size: 36px;
            letter-spacing: 0;
            color: #ffffff;
        }

        .hero p {
            margin: 10px 0 0 0;
            max-width: 850px;
            color: #cbd5e1;
            font-size: 16px;
            line-height: 1.6;
        }

        .section-title {
            margin: 24px 0 12px 0;
            color: #f8fafc;
            font-size: 22px;
            font-weight: 800;
        }

        .card {
            background: rgba(15, 23, 42, 0.84);
            border: 1px solid rgba(148, 163, 184, 0.18);
            border-radius: 18px;
            padding: 18px;
            box-shadow: 0 18px 44px rgba(0, 0, 0, 0.28);
        }

        .node-card {
            min-height: 150px;
        }

        .node-name {
            color: #f8fafc;
            font-size: 20px;
            font-weight: 800;
            margin-bottom: 8px;
        }

        .muted {
            color: #94a3b8;
            font-size: 13px;
            line-height: 1.7;
        }

        .badge {
            display: inline-block;
            padding: 5px 10px;
            border-radius: 999px;
            font-size: 12px;
            font-weight: 800;
            margin-bottom: 10px;
        }

        .badge-online {
            background: rgba(34, 197, 94, 0.16);
            color: #86efac;
            border: 1px solid rgba(34, 197, 94, 0.38);
        }

        .badge-offline {
            background: rgba(239, 68, 68, 0.16);
            color: #fca5a5;
            border: 1px solid rgba(239, 68, 68, 0.38);
        }

        .badge-master {
            background: rgba(59, 130, 246, 0.16);
            color: #93c5fd;
            border: 1px solid rgba(59, 130, 246, 0.38);
        }

        .topology {
            text-align: center;
            padding: 22px 8px;
        }

        .top-node {
            display: inline-block;
            min-width: 170px;
            padding: 16px 24px;
            border-radius: 16px;
            margin-bottom: 12px;
            font-weight: 900;
        }

        .branch {
            color: #64748b;
            font-family: Consolas, monospace;
            font-size: 26px;
            line-height: 1.1;
            margin-bottom: 8px;
        }

        .slave-row {
            display: flex;
            gap: 12px;
            justify-content: center;
            flex-wrap: wrap;
        }

        .slave-node {
            min-width: 125px;
            padding: 13px 16px;
            border-radius: 14px;
            font-weight: 800;
        }

        .node-online {
            background: rgba(21, 128, 61, 0.18);
            color: #bbf7d0;
            border: 1px solid rgba(34, 197, 94, 0.42);
        }

        .node-offline {
            background: rgba(185, 28, 28, 0.2);
            color: #fecaca;
            border: 1px solid rgba(248, 113, 113, 0.44);
        }

        .activity-item {
            padding: 11px 13px;
            border-radius: 12px;
            margin-bottom: 9px;
            background: rgba(30, 41, 59, 0.82);
            border: 1px solid rgba(148, 163, 184, 0.14);
            color: #dbeafe;
            font-size: 14px;
        }

        .flow-step {
            padding: 14px 16px;
            border-radius: 14px;
            background: rgba(15, 23, 42, 0.72);
            border: 1px solid rgba(148, 163, 184, 0.16);
            color: #cbd5e1;
            margin-bottom: 10px;
        }
    </style>
    """,
    unsafe_allow_html=True,
)


def node_url(node_name):
    return NODES[node_name]["url"]


def request_json(method, url, **kwargs):
    try:
        headers = kwargs.pop("headers", {})
        headers.setdefault("Accept", "application/json")
        response = requests.request(method, url, timeout=REQUEST_TIMEOUT, headers=headers, **kwargs)

        try:
            data = response.json() if response.content else None
        except ValueError:
            data = response.text

        return response.ok, response.status_code, data, None

    except requests.RequestException as exc:
        return False, None, None, str(exc)


def get_clients(node_name):
    ok, status, data, error = request_json("GET", f"{node_url(node_name)}/clients")

    if ok and isinstance(data, list):
        return data, None

    if error:
        return [], error

    return [], f"HTTP {status}: {data}"


def get_pending_writes():
    ok, status, data, error = request_json("GET", f"{node_url('Master')}/pending-writes")

    if ok and isinstance(data, list):
        return data, None

    if error:
        return [], error

    return [], f"HTTP {status}: {data}"


def node_is_online(node_name):
    ok, _, _, _ = request_json("GET", f"{node_url(node_name)}/clients")
    return ok


def get_statuses():
    return {name: node_is_online(name) for name in NODES}


def get_active_master(statuses):
    if statuses.get(MAIN_MASTER):
        return MAIN_MASTER

    master_url = NODES[MAIN_MASTER]["url"]
    master_ip = _extract_ip(master_url)
    master_numeric = _ip_to_numeric(master_ip)

    candidates = []

    for name, info in NODES.items():
        if name == MAIN_MASTER:
            continue

        if not statuses.get(name):
            continue

        node_ip = _extract_ip(info["url"])
        node_numeric = _ip_to_numeric(node_ip)
        distance = abs(int(node_numeric) - int(master_numeric))
        order = _node_name_order(name)

        candidates.append((distance, order, name))

    if not candidates:
        return "Unavailable"

    candidates.sort(key=lambda x: (x[0], x[1]))
    return candidates[0][2]


def add_activity(message):
    st.session_state.activity.insert(0, message)
    st.session_state.activity = st.session_state.activity[:8]


def replication_messages(operation, selected_node, active_master):
    messages = []

    if active_master == "Unavailable":
        messages.append("No active master available")
        return messages

    if selected_node == active_master:
        messages.append(f"{operation} executed locally on {active_master}")
    else:
        messages.append(f"Forwarded write from {selected_node} to {active_master}")

    for node in NODES:
        if node != active_master:
            messages.append(f"Replicated to {node}")

    return messages


def show_result(ok, status, data, selected_node, operation, active_master):
    if isinstance(data, dict) and data.get("pending"):
        st.info("Pending master approval")
        add_activity(f"{operation} pending approval from Master")
        st.write(data)
        return

    if ok:
        st.success(f"{operation} completed through {selected_node}.")

        for message in replication_messages(operation, selected_node, active_master):
            add_activity(message)

        if isinstance(data, dict) and data.get("message"):
            st.info(data["message"])

        if data:
            st.json(data)

    else:
        st.error(f"{operation} failed. Status: {status or 'offline'}")
        add_activity(f"{operation} failed on {selected_node}")

        if isinstance(data, dict) and data.get("message"):
            st.warning(data["message"])

        if data:
            st.write(data)


if "activity" not in st.session_state:
    st.session_state.activity = [
        "Dashboard ready",
        "Waiting for node activity",
    ]


if "pending_action_message" not in st.session_state:
    st.session_state.pending_action_message = None


with st.sidebar:
    st.markdown("## Control Panel")
    selected_node = st.selectbox("Selected node", list(NODES.keys()))
    st.caption(node_url(selected_node))
    refresh_clicked = st.button("Refresh cluster", use_container_width=True)
    st.divider()


statuses = get_statuses()
active_master = get_active_master(statuses)
selected_clients, selected_error = get_clients(selected_node)
pending_writes, pending_error = get_pending_writes() if selected_node == "Master" else ([], None)
total_clients = len(selected_clients)
online_count = sum(statuses.values())


if refresh_clicked:
    add_activity("Cluster status refreshed")
    st.rerun()


with st.sidebar:
    st.markdown("### System Overview")
    st.write(f"Nodes: **{len(NODES)}**")
    st.write(f"Online: **{online_count}/{len(NODES)}**")
    st.write(f"Active master: **{active_master}**")
    st.write(f"Selected DB: **{NODES[selected_node]['database']}**")

    if active_master == MAIN_MASTER:
        st.success("Main master is active")
    elif active_master != "Unavailable":
        st.warning(f"Failover mode is active — {active_master} elected")
    else:
        st.error("No active master")


if HEADER_IMAGE.exists():
    img_base64 = get_image_base64(HEADER_IMAGE)
    st.markdown(
        f"""
        <div class="cover-box">
            <img src="data:image/jpeg;base64,{img_base64}">
        </div>
        """,
        unsafe_allow_html=True,
    )


st.markdown(
    """
    <div class="hero">
        <h1>Distributed Database Command Center</h1>
        <p>
            A five-node master-slave dashboard with in-memory RAM storage, real-time replication,
            write forwarding, and simple failover. This frontend only uses the existing HTTP APIs.
        </p>
    </div>
    """,
    unsafe_allow_html=True,
)


st.markdown('<div class="section-title">Cluster Overview</div>', unsafe_allow_html=True)

overview_cols = st.columns(4)

overview_cols[0].metric("Total Nodes", len(NODES))
overview_cols[1].metric("Online Nodes", f"{online_count}/{len(NODES)}")
overview_cols[2].metric("Active Master", active_master)
overview_cols[3].metric("Clients on Selected Node", total_clients)


st.markdown('<div class="section-title">Node Status Grid</div>', unsafe_allow_html=True)

node_cols = st.columns(5)

for col, (name, details) in zip(node_cols, NODES.items()):
    online = statuses[name]
    badge = "badge-online" if online else "badge-offline"
    label = "ONLINE" if online else "OFFLINE"
    role_badge = "badge-master" if name == active_master else ""

    col.markdown(
        f"""
        <div class="card node-card">
            <div class="node-name">{name}</div>
            <span class="badge {badge}">{label}</span>
            <span class="badge {role_badge}">{details["role"]}</span>
            <div class="muted">Port: {details["port"]}</div>
            <div class="muted">Database: {details["database"]}</div>
        </div>
        """,
        unsafe_allow_html=True,
    )


topology_col, activity_col = st.columns([1.35, 1])


with topology_col:
    st.markdown(
        '<div class="section-title">Distributed Network Visualization</div>',
        unsafe_allow_html=True,
    )

    master_class = "node-online" if statuses["Master"] else "node-offline"
    slave_html = ""

    for slave in ["Slave1", "Slave2", "Slave3", "Slave4"]:
        node_class = "node-online" if statuses[slave] else "node-offline"

        slave_html += (
            f'<div class="slave-node {node_class}">'
            f'{slave}<br><span class="muted">:{NODES[slave]["port"]}</span>'
            f'</div>'
        )

    st.markdown(
        f"""
        <div class="card topology">
            <div class="top-node {master_class}">
                Master<br><span class="muted">:8080</span>
            </div>
            <div class="branch">/ &nbsp; / &nbsp; | &nbsp; \\ &nbsp; \\</div>
            <div class="slave-row">{slave_html}</div>
        </div>
        """,
        unsafe_allow_html=True,
    )


with activity_col:
    st.markdown('<div class="section-title">Live Replication Activity</div>', unsafe_allow_html=True)

    activity_html = "".join(
        f'<div class="activity-item">{message}</div>'
        for message in st.session_state.activity
    )

    st.markdown(f'<div class="card">{activity_html}</div>', unsafe_allow_html=True)


st.markdown('<div class="section-title">CRUD Operations Panel</div>', unsafe_allow_html=True)

operation_tabs = st.tabs(["Get Clients", "Add Client", "Update Client", "Delete Client"])


with operation_tabs[0]:
    st.markdown(f"Reading locally from **{selected_node}**.")

    if st.button("Get clients from selected node", use_container_width=True):
        clients, error = get_clients(selected_node)

        if error:
            st.error(error)
            add_activity(f"Client read failed from {selected_node}")
        else:
            st.dataframe(pd.DataFrame(clients), use_container_width=True, hide_index=True)
            add_activity(f"Read {len(clients)} clients from {selected_node}")


with operation_tabs[1]:
    with st.form("add_client_form"):
        name = st.text_input("Name", placeholder="Ahmed")
        national_id = st.text_input("National ID", placeholder="12345678901234")
        phone = st.text_input("Phone", placeholder="01012345678")
        email = st.text_input("Email", placeholder="ahmed@example.com")
        balance = st.number_input("Balance", min_value=0.0, value=5000.0, step=100.0)

        submitted = st.form_submit_button("Add Client", use_container_width=True)

    if submitted:
        payload = {
            "name": name,
            "national_id": national_id,
            "phone": phone,
            "email": email,
            "balance": float(balance),
        }

        ok, status, data, _ = request_json(
            "POST",
            f"{node_url(selected_node)}/clients",
            json=payload,
        )

        show_result(ok, status, data, selected_node, "Add client", active_master)
        time.sleep(0.2)


with operation_tabs[2]:
    with st.form("update_client_form"):
        client_id = st.number_input("Client ID", min_value=1, value=1, key="update_id")
        name = st.text_input("New name", placeholder="Ahmed Ali")
        national_id = st.text_input("New National ID", placeholder="12345678901234")
        phone = st.text_input("New phone", placeholder="01012345678")
        email = st.text_input("New email", placeholder="ahmed.ali@example.com")
        balance = st.number_input("New balance", min_value=0.0, value=5000.0, step=100.0)

        submitted = st.form_submit_button("Update Client", use_container_width=True)

    if submitted:
        payload = {
            "name": name,
            "national_id": national_id,
            "phone": phone,
            "email": email,
            "balance": float(balance),
        }

        ok, status, data, _ = request_json(
            "PUT",
            f"{node_url(selected_node)}/clients/{int(client_id)}",
            json=payload,
        )

        show_result(ok, status, data, selected_node, "Update client", active_master)
        time.sleep(0.2)


with operation_tabs[3]:
    with st.form("delete_client_form"):
        client_id = st.number_input("Client ID", min_value=1, value=1, key="delete_id")

        submitted = st.form_submit_button("Delete Client", use_container_width=True)

    if submitted:
        ok, status, data, _ = request_json(
            "DELETE",
            f"{node_url(selected_node)}/clients/{int(client_id)}",
        )

        show_result(ok, status, data, selected_node, "Delete client", active_master)
        time.sleep(0.2)


st.markdown('<div class="section-title">Client Table</div>', unsafe_allow_html=True)

if selected_error:
    st.warning(f"Could not load clients from {selected_node}: {selected_error}")
else:
    table = pd.DataFrame(selected_clients)

    if table.empty:
        st.info("No clients found on the selected node.")
    else:
        st.dataframe(table, use_container_width=True, hide_index=True)


if selected_node == "Master":
    st.markdown('<div class="section-title">Pending Write Requests</div>', unsafe_allow_html=True)

    if st.session_state.pending_action_message:
        level, message = st.session_state.pending_action_message
        if level == "success":
            st.success(message)
        else:
            st.error(message)
        st.session_state.pending_action_message = None

    if pending_error:
        st.warning(f"Could not load pending write requests: {pending_error}")
    else:
        pending_table = pd.DataFrame(
            [
                {
                    "Request ID": item.get("request_id"),
                    "Operation": item.get("operation"),
                    "Source Node": item.get("source_node"),
                    "Client ID": item.get("client_id"),
                    "Status": item.get("status"),
                }
                for item in pending_writes
            ]
        )

        if pending_table.empty:
            st.info("No pending write requests.")
        else:
            st.dataframe(pending_table, use_container_width=True, hide_index=True)

            for item in pending_writes:
                request_id = item.get("request_id")
                status = item.get("status", "pending")
                summary = (
                    f"Request #{request_id} · {item.get('operation')} · "
                    f"{item.get('source_node')} · status: {status}"
                )
                st.markdown(summary)

                if status != "pending":
                    continue

                action_cols = st.columns(2)
                if action_cols[0].button("Approve", key=f"approve_{request_id}", use_container_width=True):
                    ok, _, data, error = request_json(
                        "POST",
                        f"{node_url('Master')}/pending-writes/{request_id}/approve",
                    )
                    if ok:
                        message = "Pending write approved."
                        if isinstance(data, dict) and data.get("message"):
                            message = data["message"]
                        st.session_state.pending_action_message = ("success", message)
                        add_activity(f"Approved pending request #{request_id}")
                    else:
                        message = error or data
                        st.session_state.pending_action_message = ("error", f"Approve failed: {message}")
                    st.rerun()

                if action_cols[1].button("Reject", key=f"reject_{request_id}", use_container_width=True):
                    ok, _, data, error = request_json(
                        "POST",
                        f"{node_url('Master')}/pending-writes/{request_id}/reject",
                    )
                    if ok:
                        message = "Pending write rejected."
                        if isinstance(data, dict) and data.get("message"):
                            message = data["message"]
                        st.session_state.pending_action_message = ("success", message)
                        add_activity(f"Rejected pending request #{request_id}")
                    else:
                        message = error or data
                        st.session_state.pending_action_message = ("error", f"Reject failed: {message}")
                    st.rerun()


st.markdown('<div class="section-title">Failover Section</div>', unsafe_allow_html=True)

failover_cols = st.columns(3)

failover_cols[0].markdown(
    '<div class="flow-step"><strong>1. Normal</strong><br>Writes go to Master on port 8080.</div>',
    unsafe_allow_html=True,
)

failover_cols[1].markdown(
    '<div class="flow-step"><strong>2. Failure</strong><br>If Master is offline, slaves elect the closest online IP as temporary master.</div>',
    unsafe_allow_html=True,
)

failover_cols[2].markdown(
    '<div class="flow-step"><strong>3. Dynamic Election</strong><br>The elected node writes locally and replicates to all other online slaves.</div>',
    unsafe_allow_html=True,
)
