<script>
  import { onMount } from 'svelte';
  import { HardDrive, Activity, Moon, List, Search, BarChart3, ShieldCheck, ShieldAlert, ShieldX, Timer, Gauge } from 'lucide-svelte';

  let config = { drives: [] };
  let status = { 
    states: {}, 
    iops: {},
    idle_timers: {}
  };
  let events = [];
  let stats = {};

  async function fetchConfig() {
    try {
      const res = await fetch('/api/config');
      config = await res.json();
    } catch (e) {
      console.error('Failed to fetch config', e);
    }
  }

  async function fetchData() {
    try {
      const [statusRes, eventsRes, statsRes] = await Promise.all([
        fetch('/api/status'),
        fetch('/api/events'),
        fetch('/api/stats')
      ]);
      
      status = await statusRes.json();
      events = await eventsRes.json();
      stats = await statsRes.json();
    } catch (e) {
      console.error('Failed to fetch data', e);
    }
  }

  onMount(async () => {
    await fetchConfig();
    fetchData();
    const interval = setInterval(fetchData, 5000);
    return () => clearInterval(interval);
  });

  function formatDate(ts) {
    return new Date(ts).toLocaleString();
  }

  function formatTimer(seconds) {
    if (seconds <= 0) return "00:00";
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
  }

  function getRecommendation(driveStats) {
    if (!driveStats) return 'Loading health data...';
    if (driveStats.health_score === 'Critical') {
      return 'CRITICAL: High wear detected. Increase spindown timeout (hdparm -S) immediately to prevent premature failure.';
    } else if (driveStats.health_score === 'Warning') {
      return 'Warning: Frequent cycling. Consider checking for background apps or increasing sleep timeout.';
    } else {
      return 'Drive health is good. Current sleep patterns are within safe limits.';
    }
  }
</script>

<main class="container">
  <header>
    <h1>Bob HDD Spindown Monitor</h1>
    <p>Monitoring drive power states</p>
  </header>

  <div class="status-grid">
    {#each config.drives as drive}
      {@const driveStatus = status.states[drive.name] || 'unknown'}
      {@const driveStats = stats[drive.name] || { total_spin_ups: 0, avg_cycles_per_day: 0, health_score: 'Good' }}
      {@const driveIOPS = status.iops[drive.name] || 0}
      {@const driveTimer = status.idle_timers[drive.name] || 0}

      <div class="card" class:active={driveStatus === 'active'}>
        <div class="card-header">
          <HardDrive size={24} />
          <h2>{drive.name}</h2>
          <div class="health-badge" class:warning={driveStats.health_score === 'Warning'} class:critical={driveStats.health_score === 'Critical'}>
            {#if driveStats.health_score === 'Good'}<ShieldCheck size={16}/>{:else if driveStats.health_score === 'Warning'}<ShieldAlert size={16}/>{:else}<ShieldX size={16}/>{/if}
            <span>{driveStats.health_score}</span>
          </div>
        </div>
        
        <div class="state-container">
          <div class="state">
            {#if driveStatus === 'active'}
              <Activity color="#22c55e" /> <span>Active / Idle</span>
            {:else if driveStatus === 'standby'}
              <Moon color="#3b82f6" /> <span>Standby</span>
            {:else}
              <span>Loading...</span>
            {/if}
          </div>

          <div class="live-metrics">
            <div class="metric">
              <Gauge size={16} />
              <span>{driveIOPS.toFixed(1)} IOPS</span>
            </div>
            {#if driveStatus === 'active'}
              <div class="metric timer">
                <Timer size={16} />
                <span>Idle in {formatTimer(driveTimer)}</span>
              </div>
            {/if}
          </div>
        </div>

        <div class="stats-box">
          <div class="stat-item">
            <span class="label">Total Spin-ups:</span>
            <span class="val">{driveStats.total_spin_ups}</span>
          </div>
          <div class="stat-item">
            <span class="label">Avg Cycles/Day:</span>
            <span class="val">{driveStats.avg_cycles_per_day.toFixed(1)}</span>
          </div>
        </div>

        <p class="recommendation">{getRecommendation(driveStats)}</p>
        <p class="description">{drive.description} ({drive.device})</p>
      </div>
    {/each}
  </div>

  <section class="events-section">
    <div class="section-header">
      <List size={20} />
      <h3>Recent Events (Last 7 Days)</h3>
    </div>
    <div class="table-container">
      <table>
        <thead>
          <tr>
            <th>Time</th>
            <th>Drive</th>
            <th>Event</th>
            <th>Culprit</th>
          </tr>
        </thead>
        <tbody>
          {#each events as event}
            <tr>
              <td>{formatDate(event.timestamp)}</td>
              <td><strong>{event.drive}</strong></td>
              <td>
                <span class="badge" class:spin-up={event.event === 'Spin-up'}>
                  {event.event}
                </span>
              </td>
              <td>
                {#if event.culprit && event.culprit !== 'unknown'}
                  <div class="culprit-tag">
                    <Search size={12} />
                    <span>{event.culprit}</span>
                  </div>
                {:else if event.event === 'Spin-up'}
                  <span class="unknown-culprit">Unknown process</span>
                {:else}
                  <span class="muted">-</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </section>
</main>

<style>
  :global(body) {
    background-color: #0f172a;
    color: #f1f5f9;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    margin: 0;
    padding: 0;
  }

  .container {
    max-width: 1000px;
    margin: 2rem auto;
    padding: 0 1rem;
  }

  header {
    margin-bottom: 2rem;
    text-align: center;
  }

  header h1 {
    margin: 0;
    font-size: 2.5rem;
    background: linear-gradient(to right, #3b82f6, #2dd4bf);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }

  header p {
    color: #94a3b8;
    margin-top: 0.5rem;
  }

  .status-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
    gap: 1.5rem;
    margin-bottom: 3rem;
  }

  .card {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 1rem;
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    transition: all 0.3s ease;
  }

  .card.active {
    border-color: #22c55e;
    box-shadow: 0 0 15px rgba(34, 197, 94, 0.1);
  }

  .card-header {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-bottom: 1rem;
  }

  .card-header h2 {
    margin: 0;
    font-size: 1.5rem;
    flex-grow: 1;
  }

  .health-badge {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    background: #22c55e33;
    color: #4ade80;
    padding: 0.25rem 0.6rem;
    border-radius: 9999px;
    font-size: 0.75rem;
    font-weight: 700;
  }

  .health-badge.warning { background: #f59e0b33; color: #fbbf24; }
  .health-badge.critical { background: #ef444433; color: #f87171; }

  .state-container {
    margin-bottom: 1.5rem;
  }

  .state {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 1.25rem;
    font-weight: 600;
    margin-bottom: 0.5rem;
  }

  .live-metrics {
    display: flex;
    gap: 1rem;
    color: #94a3b8;
    font-size: 0.9rem;
  }

  .metric {
    display: flex;
    align-items: center;
    gap: 0.3rem;
  }

  .metric.timer {
    color: #3b82f6;
    font-weight: 600;
  }

  .stats-box {
    display: grid;
    grid-template-columns: 1fr 1fr;
    background: #0f172a;
    border-radius: 0.5rem;
    padding: 1rem;
    margin-bottom: 1rem;
  }

  .stat-item {
    display: flex;
    flex-direction: column;
  }

  .stat-item .label { color: #64748b; font-size: 0.75rem; }
  .stat-item .val { font-size: 1.25rem; font-weight: 700; }

  .recommendation {
    font-size: 0.875rem;
    background: #334155;
    padding: 0.75rem;
    border-radius: 0.5rem;
    margin-bottom: 1rem;
    border-left: 4px solid #3b82f6;
  }

  .description {
    color: #64748b;
    font-size: 0.8rem;
    margin: 0;
    margin-top: auto;
  }

  .events-section {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 1rem;
    overflow: hidden;
  }

  .section-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 1rem 1.5rem;
    border-bottom: 1px solid #334155;
  }

  .table-container { overflow-x: auto; }

  table { width: 100%; border-collapse: collapse; text-align: left; }

  th { padding: 0.75rem 1.5rem; background: #0f172a; color: #94a3b8; font-size: 0.875rem; }

  td { padding: 0.8rem 1.5rem; border-bottom: 1px solid #334155; font-size: 0.9rem; }

  .badge { padding: 0.2rem 0.5rem; border-radius: 9999px; font-size: 0.7rem; font-weight: 700; background: #3b82f633; color: #60a5fa; }
  .badge.spin-up { background: #22c55e33; color: #4ade80; }

  .culprit-tag { display: inline-flex; align-items: center; gap: 0.4rem; background: #475569; padding: 0.15rem 0.5rem; border-radius: 4px; font-family: monospace; font-size: 0.8rem; }
  .unknown-culprit { color: #64748b; font-style: italic; font-size: 0.8rem; }
  .muted { color: #475569; }
</style>
