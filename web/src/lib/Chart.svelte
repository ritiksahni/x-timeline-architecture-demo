<script>
  import { onMount, onDestroy } from 'svelte';
  import Chart from 'chart.js/auto';

  export let data = [];
  export let title = '';
  export let type = 'line';

  let canvas;
  let chart;

  $: if (chart && data) {
    updateChart();
  }

  function updateChart() {
    if (!chart) return;

    const strategies = ['fanout_write', 'fanout_read', 'hybrid'];
    const colors = {
      fanout_write: 'rgb(239, 68, 68)',
      fanout_read: 'rgb(34, 197, 94)',
      hybrid: 'rgb(59, 130, 246)'
    };

    const datasets = strategies.map(strategy => {
      const strategyData = data.filter(d => d.strategy === strategy);
      return {
        label: strategy.replace('_', ' '),
        data: strategyData.map(d => ({
          x: new Date(d.timestamp),
          y: d.duration_ms
        })),
        borderColor: colors[strategy],
        backgroundColor: colors[strategy] + '20',
        tension: 0.1,
        pointRadius: 2
      };
    });

    chart.data.datasets = datasets;
    chart.update('none');
  }

  onMount(() => {
    chart = new Chart(canvas, {
      type,
      data: {
        datasets: []
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          title: {
            display: true,
            text: title
          },
          legend: {
            position: 'bottom'
          }
        },
        scales: {
          x: {
            type: 'time',
            time: {
              unit: 'second'
            },
            title: {
              display: true,
              text: 'Time'
            }
          },
          y: {
            title: {
              display: true,
              text: 'Latency (ms)'
            },
            beginAtZero: true
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

<div class="h-64">
  <canvas bind:this={canvas}></canvas>
</div>
