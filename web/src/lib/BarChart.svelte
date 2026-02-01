<script>
  import { onMount, onDestroy } from 'svelte';
  import Chart from 'chart.js/auto';

  export let metrics = null;
  export let title = '';

  // Brand colors (updated to indigo theme)
  const BRAND_BLUE = '#4F46E5';
  const BRAND_BLUE_LIGHT = 'rgba(79, 70, 229, 0.15)';
  const BRAND_BLUE_MEDIUM = 'rgba(79, 70, 229, 0.6)';

  let canvas;
  let chart;

  $: if (chart && metrics) {
    updateChart();
  }

  function updateChart() {
    if (!chart || !metrics?.by_strategy) return;

    const strategies = ['fanout_write', 'fanout_read', 'hybrid'];
    const labels = ['Fan-Out Write', 'Fan-Out Read', 'Hybrid'];
    
    const writeData = strategies.map(s => {
      const data = metrics.by_strategy[s];
      if (!data?.write_latency_p95) return 0;
      return parseLatency(data.write_latency_p95);
    });

    const readData = strategies.map(s => {
      const data = metrics.by_strategy[s];
      if (!data?.read_latency_p95) return 0;
      return parseLatency(data.read_latency_p95);
    });

    chart.data.labels = labels;
    chart.data.datasets = [
      {
        label: 'Write P95',
        data: writeData,
        backgroundColor: BRAND_BLUE,
        borderRadius: 6,
        borderSkipped: false,
      },
      {
        label: 'Read P95',
        data: readData,
        backgroundColor: BRAND_BLUE_MEDIUM,
        borderRadius: 6,
        borderSkipped: false,
      }
    ];
    chart.update('none');
  }

  function parseLatency(str) {
    if (!str) return 0;
    // Parse Go duration strings like "1.234ms", "123µs", "1.5s"
    const match = str.match(/^([\d.]+)(µs|ms|s)$/);
    if (!match) return 0;
    const value = parseFloat(match[1]);
    const unit = match[2];
    switch (unit) {
      case 'µs': return value / 1000;
      case 'ms': return value;
      case 's': return value * 1000;
      default: return 0;
    }
  }

  onMount(() => {
    chart = new Chart(canvas, {
      type: 'bar',
      data: {
        labels: [],
        datasets: []
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          title: {
            display: !!title,
            text: title,
            font: {
              family: "'Instrument Serif', Georgia, serif",
              size: 18,
              weight: 'normal'
            },
            color: BRAND_BLUE,
            padding: { bottom: 20 }
          },
          legend: {
            position: 'bottom',
            labels: {
              usePointStyle: true,
              pointStyle: 'circle',
              padding: 20,
              font: {
                family: "'Inter', system-ui, sans-serif",
                size: 12
              }
            }
          },
          tooltip: {
            backgroundColor: 'white',
            titleColor: BRAND_BLUE,
            bodyColor: '#374151',
            borderColor: '#e5e7eb',
            borderWidth: 1,
            cornerRadius: 8,
            padding: 12,
            titleFont: {
              family: "'Inter', system-ui, sans-serif",
              weight: '600'
            },
            bodyFont: {
              family: "'Inter', system-ui, sans-serif"
            },
            callbacks: {
              label: function(context) {
                return `${context.dataset.label}: ${context.raw.toFixed(2)}ms`;
              }
            }
          }
        },
        scales: {
          y: {
            title: {
              display: true,
              text: 'Latency (ms)',
              font: {
                family: "'Inter', system-ui, sans-serif",
                size: 12
              },
              color: '#6b7280'
            },
            beginAtZero: true,
            grid: {
              color: '#f3f4f6',
              drawBorder: false
            },
            ticks: {
              font: {
                family: "'Inter', system-ui, sans-serif",
                size: 11
              },
              color: '#9ca3af'
            }
          },
          x: {
            grid: {
              display: false
            },
            ticks: {
              font: {
                family: "'Inter', system-ui, sans-serif",
                size: 12
              },
              color: '#374151'
            }
          }
        }
      }
    });

    updateChart();
  });

  onDestroy(() => {
    if (chart) {
      chart.destroy();
    }
  });
</script>

<div class="h-72">
  <canvas bind:this={canvas}></canvas>
</div>
