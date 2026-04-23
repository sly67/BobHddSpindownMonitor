# HDD Spin State Monitor

A lightweight, Dockerized tool for monitoring the power states of hard drives and identifying the processes that wake them up. Designed for Debian-based systems.

## Features
*   **Real-time Monitoring:** Tracks spin-up and spin-down events every 30 seconds.
*   **Process Identification:** Uses kernel `block_dump` to identify the specific process or script that triggers a drive wake-up.
*   **Statistics & Health:** Calculates daily spin cycles and provides health recommendations (Good, Warning, Critical).
*   **Data Retention:** Automatically purges event logs older than 7 days to keep the database lean.
*   **Persistence:** Data survives container rebuilds via Docker volumes.

## Setup

### 1. Host Configuration (Debian)
Ensure `hdparm` is installed and configured for persistent sleep routines on the host:

```bash
sudo apt install hdparm
```

Add the following to `/etc/hdparm.conf`:
```conf
/dev/sda {
    spindown_time = 241  # 30 minutes
}
/dev/sdb {
    spindown_time = 241  # 30 minutes
}
```

### 2. Deployment
Run the monitor using Docker Compose:

```bash
docker compose up -d --build
```

Access the dashboard at `http://localhost:48070`.

## Architecture
*   **Backend:** Go (Golang) serving a REST API and a hardware polling loop.
*   **Frontend:** Svelte (Vite) for a responsive dashboard.
*   **Database:** SQLite (persisted in a Docker volume).
*   **Kernel Integration:** Requires `privileged` mode to read `/dev/kmsg` and manage `vm.block_dump`.

## Health Metrics
The monitor calculates the **Average Cycles Per Day**.
*   **Good (< 12 cycles/day):** Healthy sleep patterns.
*   **Warning (12-24 cycles/day):** High cycling frequency. Check for periodic background tasks.
*   **Critical (> 24 cycles/day):** Excessive wear. Consider increasing the `spindown_time` or disabling sleep for the drive.

## Maintenance
Logs are automatically purged every hour for entries older than 1 week. The database is located at `/var/lib/docker/volumes/hdd-monitor_hdd-monitor-data/` on the host.
