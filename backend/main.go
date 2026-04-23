package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Event struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Drive     string    `json:"drive"`
	Event     string    `json:"event"`
	Culprit   string    `json:"culprit"`
}

type DriveStats struct {
	TotalSpinUps    int     `json:"total_spin_ups"`
	TotalSpinDowns  int     `json:"total_spin_downs"`
	AvgCyclesPerDay float64 `json:"avg_cycles_per_day"`
	HealthScore     string  `json:"health_score"`
}

type StatsResponse struct {
	Pulsar DriveStats `json:"pulsar"`
	Quasar DriveStats `json:"quasar"`
}

type Status struct {
	Pulsar          string  `json:"pulsar"`
	Quasar          string  `json:"quasar"`
	PulsarIOPS      float64 `json:"pulsar_iops"`
	QuasarIOPS      float64 `json:"quasar_iops"`
	PulsarIdleTimer int     `json:"pulsar_idle_timer"` // Seconds remaining
	QuasarIdleTimer int     `json:"quasar_idle_timer"` // Seconds remaining
}

var (
	db              *sql.DB
	lastIO          = make(map[string]uint64)
	lastActivity    = make(map[string]time.Time)
	currentIOPS     = make(map[string]float64)
	mu              sync.Mutex
	spindownTimeout = 1800 // 30 minutes
)

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "/data/hdd_monitor.db")
	if err != nil {
		log.Fatal(err)
	}

	sqlStmt := `
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		drive TEXT,
		event TEXT,
		culprit TEXT
	);
	`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Fatalf("%q: %s\n", err, sqlStmt)
	}
}

func purgeOldData() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		db.Exec("DELETE FROM events WHERE timestamp < datetime('now', '-7 days')")
	}
}

func getDriveState(device string) string {
	out, err := exec.Command("hdparm", "-C", device).Output()
	if err != nil {
		return "unknown"
	}
	s := string(out)
	if strings.Contains(s, "standby") {
		return "standby"
	}
	if strings.Contains(s, "active/idle") {
		return "active"
	}
	return "unknown"
}

func setBlockDump(val string) {
	os.WriteFile("/proc/sys/vm/block_dump", []byte(val+"\n"), 0644)
}

func getCulprit(dev string) string {
	setBlockDump("1")
	time.Sleep(2 * time.Second)
	defer setBlockDump("0")

	devBase := strings.TrimPrefix(dev, "/dev/")
	out, err := exec.Command("dmesg").Output()
	if err != nil {
		return "unknown"
	}

	lines := strings.Split(string(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if strings.Contains(line, devBase) && (strings.Contains(line, "READ") || strings.Contains(line, "WRITE") || strings.Contains(line, "dirtied")) {
			if strings.Contains(line, "):") {
				parts := strings.Split(line, "):")
				if len(parts) > 0 {
					subParts := strings.Split(parts[0], " ")
					return subParts[len(subParts)-1]
				}
			}
		}
	}
	return "unknown"
}

func monitorIO() {
	ticker := time.NewTicker(2 * time.Second)
	for range ticker.C {
		data, err := os.ReadFile("/proc/diskstats")
		if err != nil {
			continue
		}

		mu.Lock()
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 14 {
				continue
			}
			devName := fields[2]
			if devName == "sda" || devName == "sdb" {
				reads, _ := strconv.ParseUint(fields[3], 10, 64)
				writes, _ := strconv.ParseUint(fields[7], 10, 64)
				total := reads + writes

				if prev, ok := lastIO[devName]; ok {
					diff := total - prev
					currentIOPS[devName] = float64(diff) / 2.0
					if diff > 0 {
						lastActivity[devName] = time.Now()
					}
				}
				lastIO[devName] = total
			}
		}
		mu.Unlock()
	}
}

func pollDrives() {
	ticker := time.NewTicker(15 * time.Second)
	states := make(map[string]string)
	states["/dev/sda"] = getDriveState("/dev/sda")
	states["/dev/sdb"] = getDriveState("/dev/sdb")

	for range ticker.C {
		for _, dev := range []string{"/dev/sda", "/dev/sdb"} {
			currentState := getDriveState(dev)
			if currentState != states[dev] && currentState != "unknown" {
				driveName := "Quasar"
				if dev == "/dev/sdb" {
					driveName = "Pulsar"
				}

				event := "Spin-up"
				culprit := ""
				if currentState == "standby" {
					event = "Spin-down"
				} else {
					culprit = getCulprit(dev)
					mu.Lock()
					lastActivity[strings.TrimPrefix(dev, "/dev/")] = time.Now()
					mu.Unlock()
				}

				db.Exec("INSERT INTO events (drive, event, culprit) VALUES (?, ?, ?)", driveName, event, culprit)
				states[dev] = currentState
			}
		}
	}
}

func calculateDriveStats(driveName string) DriveStats {
	var stats DriveStats
	db.QueryRow("SELECT COUNT(*) FROM events WHERE drive = ? AND event = 'Spin-up'", driveName).Scan(&stats.TotalSpinUps)
	db.QueryRow("SELECT COUNT(*) FROM events WHERE drive = ? AND event = 'Spin-down'", driveName).Scan(&stats.TotalSpinDowns)

	var days float64
	db.QueryRow("SELECT CASE WHEN julianday('now') - min(julianday(timestamp)) < 0.01 THEN 0.01 ELSE julianday('now') - min(julianday(timestamp)) END FROM events WHERE drive = ?", driveName).Scan(&days)

	if days > 0 {
		stats.AvgCyclesPerDay = float64(stats.TotalSpinUps) / days
	}

	if stats.AvgCyclesPerDay > 24 {
		stats.HealthScore = "Critical"
	} else if stats.AvgCyclesPerDay > 12 {
		stats.HealthScore = "Warning"
	} else {
		stats.HealthScore = "Good"
	}
	return stats
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	resp := StatsResponse{
		Quasar: calculateDriveStats("Quasar"),
		Pulsar: calculateDriveStats("Pulsar"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	qState := getDriveState("/dev/sda")
	pState := getDriveState("/dev/sdb")

	qTimer := 0
	if qState == "active" {
		elapsed := time.Since(lastActivity["sda"]).Seconds()
		qTimer = int(float64(spindownTimeout) - elapsed)
		if qTimer < 0 { qTimer = 0 }
	}

	pTimer := 0
	if pState == "active" {
		elapsed := time.Since(lastActivity["sdb"]).Seconds()
		pTimer = int(float64(spindownTimeout) - elapsed)
		if pTimer < 0 { pTimer = 0 }
	}

	status := Status{
		Quasar:          qState,
		Pulsar:          pState,
		QuasarIOPS:      currentIOPS["sda"],
		PulsarIOPS:      currentIOPS["sdb"],
		QuasarIdleTimer: qTimer,
		PulsarIdleTimer: pTimer,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func handleEvents(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, timestamp, drive, event, COALESCE(culprit, '') FROM events ORDER BY timestamp DESC LIMIT 100")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		rows.Scan(&e.ID, &e.Timestamp, &e.Drive, &e.Event, &e.Culprit)
		events = append(events, e)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

func main() {
	initDB()
	lastActivity["sda"] = time.Now()
	lastActivity["sdb"] = time.Now()
	
	go pollDrives()
	go purgeOldData()
	go monitorIO()

	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/events", handleEvents)
	http.HandleFunc("/api/stats", handleStats)

	fs := http.FileServer(http.Dir("./dist"))
	http.Handle("/", fs)

	port := os.Getenv("PORT")
	if port == "" {
		port = "48070"
	}

	fmt.Printf("Starting server on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
