# HDD Spin State Monitor

A lightweight, Dockerized tool for monitoring the power states of hard drives, tracking IOPS, and identifying the processes that wake them up. Designed for Debian-based systems.

## Features
*   **Real-time Monitoring:** Tracks spin-up and spin-down events, live IOPS, and estimated time to idle.
*   **Process Identification:** Uses kernel `block_dump` to identify the specific process or script that triggers a drive wake-up.
*   **Single Source of Truth:** All drives and timeouts are managed via a simple `config.json` file.
*   **Statistics & Health:** Calculates daily spin cycles and provides health recommendations (Good, Warning, Critical).
*   **Data Retention:** Automatically purges event logs older than 7 days to keep the database lean.

## Setup

### 1. Host Configuration (Debian)
It is recommended to use `hd-idle` for reliable spindown management on modern Seagate drives.

```bash
sudo apt install hd-idle
```

Configure `/etc/default/hd-idle` (Example for 30-minute spindown):
```conf
START_HD_IDLE=true
HD_IDLE_OPTS="-i 0 -a sda -i 1800 -a sdb -i 1800"
```

#### Optimization: Disable Access Time (`noatime`)
To prevent small read operations from resetting the idle timer, update your `/etc/fstab` to include the `noatime` option for your HDD mount points:
```conf
UUID=... /mnt/pulsar ext4 defaults,noatime,nofail 0 0
```

### 2. Configuration
Edit the `config.json` in the project root to define your drives:

```json
{
  "drives": [
    {
      "name": "Quasar",
      "device": "/dev/sda",
      "description": "Backup Storage"
    }
  ],
  "spindown_timeout_seconds": 1800,
  "polling_interval_seconds": 30
}
```

### 3. Deployment
Run the monitor using Docker Compose:

```bash
docker compose up -d --build
```

Access the dashboard at `http://localhost:48070`.

## Architecture
*   **Status Polling:** Uses `hdparm -C` inside the container to check power states without waking the disks.
*   **Backend:** Go (Golang) serving a REST API and a hardware polling loop.
*   **Frontend:** Svelte (Vite) for a responsive dashboard with live metrics.
*   **Database:** SQLite (persisted in a Docker volume).
*   **Kernel Integration:** Requires `privileged` mode to read `/dev/kmsg` and manage `vm.block_dump`.

## Health Metrics
The monitor calculates the **Average Cycles Per Day**.
*   **Good (< 12 cycles/day):** Healthy sleep patterns.
*   **Warning (12-24 cycles/day):** High cycling frequency. Check for periodic background tasks.
*   **Critical (> 24 cycles/day):** Excessive wear. Consider increasing the `spindown_timeout_seconds` or checking for aggressive background processes.

## Maintenance
Logs are automatically purged every hour for entries older than 1 week.
