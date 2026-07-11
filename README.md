# Distributed Banking System

A 5-node distributed banking clients database project built with Go (Gin), Python (Flask), and Streamlit. It uses **In-Memory RAM Storage** instead of a SQL database, featuring write forwarding, failover, and active replication.

## Key Features

- Five-node distributed architecture with one master and four replicas
- In-memory client data storage protected by synchronization locks
- Write forwarding from replicas to the active master
- Automatic failover to a backup master when the main master is unavailable
- Asynchronous replication across Go and Python nodes
- REST endpoints for creating, reading, updating, and deleting banking clients
- Health checks and node-status monitoring
- Streamlit dashboard for cluster visibility and client management
- Environment-based configuration for localhost or multi-laptop LAN deployment

## Technologies

- Go and Gin
- Python and Flask
- Streamlit
- REST APIs
- Concurrent replication and failover logic
- In-memory data structures


> [!WARNING]
> Since data is stored entirely in memory (RAM) within each node, **all data will be lost when a node restarts**.

## Structure

```text
cmd/api                 Master node (Go Gin)
cmd/slave               Slave1 / Backup Master node (Go Gin)
cmd/slave3              Slave3 node (Go Gin)
app.py                  Slave2 node (Flask Python)
app_slave4.py           Slave4 node (Flask Python)
streamlit_app.py        Streamlit dashboard GUI
requirements.txt        Python dependencies
internal/config         Environment-based configuration
internal/handlers       Gin HTTP handlers for clients
internal/models         Request and response models
internal/repositories   Master in-memory store
internal/replication    Asynchronous HTTP replication
internal/routes         Route registration
internal/slave          Slave handlers, in-memory repository, and routes
```

## Node Topology & Default Ports

```text
Master  (Go)     -> Port 8080 (Active Master)
Slave1  (Go)     -> Port 8081 (Slave / Temporary Master Failover)
Slave2  (Python) -> Port 5000 (Slave)
Slave3  (Go)     -> Port 8082 (Slave)
Slave4  (Python) -> Port 5001 (Slave)
```

## Configuration

The system is configured using environment variables. If no environment variables are set, the system defaults to running everything on `localhost`:

| Variable | Default | Purpose |
| --- | --- | --- |
| `SERVER_ADDRESS` | `:8080` | Bind address for Master node |
| `SLAVE_SERVER_ADDRESS` | `:8081` | Bind address for Slave1 node |
| `SLAVE2_PORT` | `5000` | Port for Slave2 node |
| `SLAVE3_SERVER_ADDRESS` | `:8082` | Bind address for Slave3 node |
| `SLAVE4_PORT` | `5001` | Port for Slave4 node (via `app_slave4.py`) |
| `MASTER_URL` | `http://localhost:8080` | URL to communicate with Master |
| `SLAVE1_URL` | `http://localhost:8081` | URL to communicate with Slave1 |
| `SLAVE2_URL` | `http://localhost:5000` | URL to communicate with Slave2 |
| `SLAVE3_URL` | `http://localhost:8082` | URL to communicate with Slave3 |
| `SLAVE4_URL` | `http://localhost:5001` | URL to communicate with Slave4 |

## Run Locally on a Single Machine

To start the cluster on a single machine, open 6 separate terminal windows and run the following commands.

### 1. Install Dependencies
```bash
# Go dependencies
go mod tidy

# Python dependencies
pip install -r requirements.txt
```

### 2. Run Nodes

*   **Master Node (Port 8080):**
    ```bash
    go run ./cmd/api
    ```
*   **Slave1 Node (Port 8081 - Failover Master):**
    ```bash
    go run ./cmd/slave
    ```
*   **Slave2 Node (Port 5000):**
    ```bash
    python app.py
    ```
*   **Slave3 Node (Port 8082):**
    ```bash
    go run ./cmd/slave3
    ```
*   **Slave4 Node (Port 5001):**
    ```bash
    python app_slave4.py
    ```
*   **Streamlit Dashboard:**
    ```bash
    streamlit run streamlit_app.py
    ```

---

## Run Across Multiple Laptops (LAN Setup)

To run the nodes on separate laptops connected to the same local Wi-Fi or Ethernet network:

### 1. Determine Local IP Addresses
On each laptop, open a terminal and find its local IP address (e.g. `192.168.1.X`):
*   **Windows:** `ipconfig`
*   **macOS / Linux:** `ifconfig` or `ip a`

Assume the laptops have the following IP addresses:
*   Master Laptop: `192.168.1.10`
*   Slave1 Laptop: `192.168.1.11`
*   Slave2 Laptop: `192.168.1.12`
*   Slave3 Laptop: `192.168.1.13`
*   Slave4 Laptop: `192.168.1.14`

### 2. Set environment variables on each machine before running
Set the environment variables corresponding to the network configuration:

#### Windows (PowerShell)
```powershell
$env:MASTER_URL="http://192.168.1.10:8080"
$env:SLAVE1_URL="http://192.168.1.11:8081"
$env:SLAVE2_URL="http://192.168.1.12:5000"
$env:SLAVE3_URL="http://192.168.1.13:8082"
$env:SLAVE4_URL="http://192.168.1.14:5001"
```

#### Windows (CMD)
```cmd
set MASTER_URL=http://192.168.1.10:8080
set SLAVE1_URL=http://192.168.1.11:8081
set SLAVE2_URL=http://192.168.1.12:5000
set SLAVE3_URL=http://192.168.1.13:8082
set SLAVE4_URL=http://192.168.1.14:5001
```

#### macOS / Linux
```bash
export MASTER_URL="http://192.168.1.10:8080"
export SLAVE1_URL="http://192.168.1.11:8081"
export SLAVE2_URL="http://192.168.1.12:5000"
export SLAVE3_URL="http://192.168.1.13:8082"
export SLAVE4_URL="http://192.168.1.14:5001"
```

### 3. Run the Nodes on Each Laptop
After setting the environment variables in step 2:
*   On **Master Laptop (`192.168.1.10`)**: Run `go run ./cmd/api`
*   On **Slave1 Laptop (`192.168.1.11`)**: Run `go run ./cmd/slave`
*   On **Slave2 Laptop (`192.168.1.12`)**: Run `python app.py`
*   On **Slave3 Laptop (`192.168.1.13`)**: Run `go run ./cmd/slave3`
*   On **Slave4 Laptop (`192.168.1.14`)**: Run `python app_slave4.py`
*   On **Any Laptop**: Run `streamlit run streamlit_app.py` to view the unified status dashboard.

---

## Replication and Write Flow

### 1. Write Forwarding
When a user submits a client operation (`POST`, `PUT`, `DELETE`) to any slave node (e.g. Slave2 on `:5000`), the slave forwards it to the active master:
*   Under normal operations, forwarded to **Master** (`:8080`).
*   If **Master** is offline, forwarded to **Slave1** (`:8081`) which temporarily acts as Master.

### 2. Multi-threaded Replication
The active master processes the write in its local RAM storage, then sends parallel HTTP `POST /replicate` requests to all other nodes concurrently using goroutines.
If any node is offline or slow, the master logs a warning but proceeds immediately without blocking client requests.
