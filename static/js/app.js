const { createApp, ref, computed, onMounted } = Vue

// Chart.js instance lives outside Vue reactivity — no need for it to be a ref.
let chartInstance = null
let chartTimer    = null

createApp({
  setup() {
    const logs          = ref([])
    const search        = ref('')
    const levelFilter   = ref('ALL')
    const serviceFilter = ref('')
    const liveTail      = ref(true)
    const connected     = ref(false)
    const expandedId    = ref(null)
    const darkMode      = ref(localStorage.getItem('hl-dark') !== '0') // dark by default
    const now           = ref(Date.now())
    const chartCanvas   = ref(null)

    const LEVELS = ['ALL', 'DEBUG', 'INFO', 'WARNING', 'ERROR', 'CRITICAL']
    const LABEL  = { ALL:'ALL', DEBUG:'DEBUG', INFO:'INFO', WARNING:'WARN', ERROR:'ERROR', CRITICAL:'CRIT' }

    const LEGEND = [
      { key: 'DEBUG',    label: 'DEBUG', color: 'rgba(110,118,129,.55)' },
      { key: 'INFO',     label: 'INFO',  color: 'rgba(88,166,255,.65)'  },
      { key: 'WARNING',  label: 'WARN',  color: 'rgba(210,153,34,.75)'  },
      { key: 'ERROR',    label: 'ERROR', color: 'rgba(248,81,73,.8)'    },
      { key: 'CRITICAL', label: 'CRIT',  color: 'rgba(255,123,114,.85)' },
    ]

    // ── computed ────────────────────────────────────────────────────────────

    const services = computed(() =>
      [...new Set(logs.value.map(l => l.service_name).filter(Boolean))].sort()
    )

    const displayLogs = computed(() => {
      let r = logs.value
      if (levelFilter.value !== 'ALL') r = r.filter(l => l.level === levelFilter.value)
      if (serviceFilter.value)         r = r.filter(l => l.service_name === serviceFilter.value)
      return r
    })

    const levelCounts = computed(() => {
      const c = { DEBUG: 0, INFO: 0, WARNING: 0, ERROR: 0, CRITICAL: 0 }
      for (const l of logs.value) if (c[l.level] !== undefined) c[l.level]++
      return c
    })

    // ── helpers ─────────────────────────────────────────────────────────────

    function reltime(ts) {
      const s = Math.floor((now.value - new Date(ts).getTime()) / 1000)
      if (s < 5)  return 'just now'
      if (s < 60) return `${s}s ago`
      const m = Math.floor(s / 60)
      if (m < 60) return `${m}m ago`
      const h = Math.floor(m / 60)
      if (h < 24) return `${h}h ago`
      return new Date(ts).toLocaleDateString()
    }

    // ── chart ────────────────────────────────────────────────────────────────

    function buildChartData() {
      const BUCKETS = 30
      const STEP_MS = 60_000
      const cutoff  = Date.now() - BUCKETS * STEP_MS

      const buckets = {
        DEBUG: new Array(BUCKETS).fill(0),
        INFO:  new Array(BUCKETS).fill(0),
        WARNING: new Array(BUCKETS).fill(0),
        ERROR: new Array(BUCKETS).fill(0),
        CRITICAL: new Array(BUCKETS).fill(0),
      }

      for (const log of logs.value) {
        const ts  = new Date(log.timestamp).getTime()
        const idx = Math.floor((ts - cutoff) / STEP_MS)
        if (idx >= 0 && idx < BUCKETS && buckets[log.level]) buckets[log.level][idx]++
      }

      const labels = Array.from({ length: BUCKETS }, (_, i) =>
        new Date(cutoff + (i + 0.5) * STEP_MS)
          .toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
      )

      return { labels, ...buckets }
    }

    function initChart() {
      if (!chartCanvas.value || typeof Chart === 'undefined') return
      const d = buildChartData()

      chartInstance = new Chart(chartCanvas.value, {
        type: 'bar',
        data: {
          labels: d.labels,
          datasets: LEGEND.map(({ key, label, color }) => ({
            label,
            data: d[key],
            backgroundColor: color,
            borderWidth: 0,
          })),
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          animation: false,
          plugins: {
            legend: { display: false },
            tooltip: {
              backgroundColor: '#161b22',
              borderColor: '#30363d',
              borderWidth: 1,
              titleColor: '#e6edf3',
              bodyColor: '#8b949e',
              titleFont: { family: '"SFMono-Regular",Consolas,Menlo,monospace', size: 11 },
              bodyFont:  { family: '"SFMono-Regular",Consolas,Menlo,monospace', size: 11 },
              padding: 8,
              callbacks: {
                label: ctx => ctx.raw > 0 ? `  ${ctx.dataset.label}  ${ctx.raw}` : null,
              },
            },
          },
          scales: {
            x: {
              stacked: true,
              grid:   { color: 'rgba(48,54,61,.6)', tickLength: 0 },
              border: { display: false },
              ticks:  {
                color: '#6e7781',
                font:  { family: '"SFMono-Regular",Consolas,Menlo,monospace', size: 10 },
                maxTicksLimit: 8,
                maxRotation: 0,
              },
            },
            y: {
              stacked: true,
              grid:   { color: 'rgba(48,54,61,.6)', tickLength: 0 },
              border: { display: false },
              ticks:  {
                color: '#6e7781',
                font:  { family: '"SFMono-Regular",Consolas,Menlo,monospace', size: 10 },
                maxTicksLimit: 4,
                precision: 0,
              },
            },
          },
        },
      })
    }

    function updateChart() {
      if (!chartInstance) return
      const d = buildChartData()
      chartInstance.data.labels = d.labels
      LEGEND.forEach(({ key }, i) => { chartInstance.data.datasets[i].data = d[key] })
      chartInstance.update('none')
    }

    function scheduleChartUpdate() {
      clearTimeout(chartTimer)
      chartTimer = setTimeout(updateChart, 600)
    }

    // ── data loading ─────────────────────────────────────────────────────────

    async function loadLogs() {
      const url = search.value
        ? `/api/search-logs?q=${encodeURIComponent(search.value)}&limit=200`
        : '/api/logs?limit=200'
      try {
        const res  = await fetch(url)
        const data = await res.json()
        logs.value       = data.logs || []
        expandedId.value = null
        updateChart()
      } catch (e) { console.error('fetch failed', e) }
    }

    function onSearchInput() { if (!search.value) loadLogs() }
    function clearSearch()   { search.value = ''; loadLogs() }

    // ── ui actions ───────────────────────────────────────────────────────────

    function toggleExpand(id) {
      expandedId.value = expandedId.value === id ? null : id
    }

    function toggleDark() {
      darkMode.value = !darkMode.value
      localStorage.setItem('hl-dark', darkMode.value ? '1' : '0')
    }

    async function exportLogs() {
      try {
        const res  = await fetch('/api/logs?limit=500')
        const data = await res.json()
        const text = (data.logs || []).map(l =>
          `[${l.level}] [${l.service_name || '-'}] [${l.timestamp}] ${l.message}`
        ).join('\n')
        const a = Object.assign(document.createElement('a'), {
          href:     URL.createObjectURL(new Blob([text], { type: 'text/plain' })),
          download: 'huelogs-export.txt',
        })
        a.click()
        URL.revokeObjectURL(a.href)
      } catch (e) { console.error('export failed', e) }
    }

    // ── websocket ────────────────────────────────────────────────────────────

    let ws = null, delay = 1000

    function connectWS() {
      const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
      ws = new WebSocket(`${proto}//${location.host}/ws`)

      ws.onopen  = () => { connected.value = true; delay = 1000 }
      ws.onerror = () => { connected.value = false }
      ws.onclose = () => {
        connected.value = false
        setTimeout(connectWS, delay)
        delay = Math.min(delay * 2, 30_000)
      }
      ws.onmessage = ({ data }) => {
        if (!liveTail.value || search.value) return
        try {
          logs.value.unshift(JSON.parse(data))
          if (logs.value.length > 500) logs.value.length = 500
          scheduleChartUpdate()
        } catch {}
      }
    }

    onMounted(() => {
      connectWS()
      initChart()
      loadLogs()
      setInterval(() => { now.value = Date.now() }, 10_000)
    })

    return {
      logs, search, levelFilter, serviceFilter,
      liveTail, connected, expandedId, darkMode,
      chartCanvas, LEVELS, LABEL, LEGEND,
      services, displayLogs, levelCounts,
      reltime, loadLogs, onSearchInput, clearSearch,
      toggleExpand, toggleDark, exportLogs,
    }
  },
}).mount('#app')
