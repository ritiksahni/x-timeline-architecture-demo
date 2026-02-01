<script>
  import { onMount, onDestroy } from 'svelte';
  import { postTweet, getTimeline, getConfig, updateConfig, getMetrics, getRecentMetrics, clearMetrics, healthCheck, getSampleUsers, getUserFollowers, getUserFollowing } from './lib/api.js';
  import BarChart from './lib/BarChart.svelte';
  import UserPicker from './lib/UserPicker.svelte';
  import FanOutVisualizer from './lib/FanOutVisualizer.svelte';

  // State
  let connected = false;
  let config = {};
  let metrics = null;
  let recentMetrics = [];
  let timeline = [];
  let lastOperationResult = null;
  
  // Form state
  let selectedStrategy = 'fanout_write';
  let userId = 1;
  let tweetContent = '';
  let timelineUserId = 1;
  
  // Config form
  let newThreshold = 10000;
  
  // Polling
  let pollInterval;
  
  let sampleUsers = [];
  let regularUser = null;
  let celebrityUser = null;
  let selectedPersona = null; // 'regular' or 'celebrity'
  let authorFollowers = [];
  let demoPhase = 'select'; // 'select', 'posting', 'result'
  let isAnimating = false;
  let visualizerRef;
  let autoLoadedTimeline = false;

  // Strategy info - copy optimized for clarity and engagement
  const strategies = [
    {
      id: 'fanout_write',
      name: 'Fan-Out on Write',
      subtitle: 'Push Model',
      tagline: 'Tweet once, deliver everywhere instantly',
      description: 'When you post, the tweet is pushed to every follower\'s timeline cache immediately.',
      writeLabel: 'Slow',
      readLabel: 'Fast',
      writeComplexity: 'O(followers)',
      readComplexity: 'O(1)',
      color: 'indigo',
      regularBehavior: 'Updates all follower caches immediately',
      celebrityBehavior: 'Updates ALL follower caches - expensive!',
      regularIcon: 'fast',
      celebrityIcon: 'slow',
    },
    {
      id: 'fanout_read',
      name: 'Fan-Out on Read',
      subtitle: 'Pull Model',
      tagline: 'Store once, build timelines on-demand',
      description: 'Tweets are stored once. Timelines are assembled fresh when someone opens their feed.',
      writeLabel: 'Fast',
      readLabel: 'Slow',
      writeComplexity: 'O(1)',
      readComplexity: 'O(following)',
      color: 'emerald',
      regularBehavior: 'Stores tweet once, readers query later',
      celebrityBehavior: 'Same as regular - just stores once',
      regularIcon: 'fast',
      celebrityIcon: 'fast',
    },
    {
      id: 'hybrid',
      name: 'Hybrid',
      subtitle: 'What Twitter Uses',
      tagline: 'Push for most, pull for celebrities',
      description: 'Regular users get push delivery. Celebrity tweets are fetched on-demand and merged.',
      writeLabel: 'Balanced',
      readLabel: 'Balanced',
      writeComplexity: 'O(1) to O(n)',
      readComplexity: 'O(1) + O(celebs)',
      color: 'amber',
      badge: 'Production Strategy',
      regularBehavior: 'Push to all followers (like fan-out write)',
      celebrityBehavior: 'Stores once, merged on-demand (avoids explosion)',
      regularIcon: 'fast',
      celebrityIcon: 'smart',
    }
  ];

  // Computed: selected author based on persona
  $: selectedAuthor = selectedPersona === 'celebrity' ? celebrityUser : regularUser;

  onMount(async () => {
    await checkConnection();
    if (connected) {
      await loadConfig();
      await loadMetrics();
      await loadSampleUsers();
      startPolling();
    }
  });
  
  async function loadSampleUsers() {
    try {
      const result = await getSampleUsers();
      sampleUsers = result.users || [];
      
      // Find a regular user and a celebrity
      celebrityUser = sampleUsers.find(u => u.is_celebrity);
      regularUser = sampleUsers.find(u => !u.is_celebrity && u.follower_count > 0) 
                   || sampleUsers.find(u => !u.is_celebrity);
      
      // If no celebrity exists, show a warning
      if (!celebrityUser && sampleUsers.length > 0) {
        // Find user with most followers as "celebrity" stand-in
        const sorted = [...sampleUsers].sort((a, b) => b.follower_count - a.follower_count);
        if (sorted[0]?.follower_count >= 100) {
          celebrityUser = sorted[0];
          regularUser = sorted[sorted.length - 1];
        }
      }
    } catch (e) {
      console.error('Failed to load sample users:', e);
    }
  }
  
  async function loadAuthorFollowers() {
    if (!selectedAuthor) return;
    try {
      const result = await getUserFollowers(selectedAuthor.id);
      authorFollowers = result.sample_followers || [];
      // Update follower count from server
      if (selectedPersona === 'celebrity' && celebrityUser) {
        celebrityUser.follower_count = result.follower_count ?? celebrityUser.follower_count;
      } else if (selectedPersona === 'regular' && regularUser) {
        regularUser.follower_count = result.follower_count ?? regularUser.follower_count;
      }
    } catch (e) {
      console.error('Failed to load followers:', e);
    }
  }
  
  async function selectPersona(persona) {
    selectedPersona = persona;
    autoLoadedTimeline = false;
    timeline = [];
    lastOperationResult = null;
    
    const author = persona === 'celebrity' ? celebrityUser : regularUser;
    if (author) {
      userId = author.id;
      await loadAuthorFollowers();
      tweetContent = `Hello from ${author.username}! Testing ${strategies.find(s => s.id === selectedStrategy)?.name || 'fan-out'}.`;
      demoPhase = 'posting';
    }
  }

  onDestroy(() => {
    if (pollInterval) {
      clearInterval(pollInterval);
    }
  });

  async function checkConnection() {
    connected = await healthCheck();
  }

  async function loadConfig() {
    try {
      config = await getConfig();
      newThreshold = config.celebrity_threshold;
    } catch (e) {
      console.error('Failed to load config:', e);
    }
  }

  async function loadMetrics() {
    try {
      metrics = await getMetrics();
      recentMetrics = await getRecentMetrics(100);
    } catch (e) {
      console.error('Failed to load metrics:', e);
    }
  }

  function startPolling() {
    pollInterval = setInterval(async () => {
      await loadMetrics();
    }, 2000);
  }

  async function handlePostTweet() {
    if (!tweetContent.trim() || !selectedAuthor) return;
    
    try {
      isAnimating = true;
      
      const result = await postTweet(selectedAuthor.id, tweetContent, selectedStrategy);
      const opMetrics = result.metrics || {};
      lastOperationResult = {
        type: 'write',
        strategy: selectedStrategy,
        latency: opMetrics.duration || (opMetrics.duration_ms ? opMetrics.duration_ms + 'ms' : '-'),
        fanOutCount: opMetrics.fan_out_count || 0
      };
      
      // Trigger visualization animation
      if (visualizerRef && lastOperationResult.fanOutCount >= 0) {
        await visualizerRef.startAnimation();
      }
      
      await loadMetrics();
      
      // Auto-load a follower's timeline after animation
      setTimeout(async () => {
        demoPhase = 'result';
        isAnimating = false;
        
        // Automatically load first follower's timeline to show the tweet arrived
        if (authorFollowers.length > 0 && !autoLoadedTimeline) {
          autoLoadedTimeline = true;
          timelineUserId = authorFollowers[0];
          await handleGetTimeline();
        }
      }, 2000);
      
    } catch (e) {
      console.error('Failed to post tweet:', e);
      isAnimating = false;
    }
  }

  async function handleGetTimeline() {
    try {
      const result = await getTimeline(timelineUserId, selectedStrategy);
      timeline = result.tweets || [];
      const opMetrics = result.metrics || {};
      // Keep write result if we're just viewing timeline
      if (lastOperationResult?.type !== 'write') {
        lastOperationResult = {
          type: 'read',
          strategy: selectedStrategy,
          latency: opMetrics.duration || (opMetrics.duration_ms ? opMetrics.duration_ms + 'ms' : '-'),
          cacheHit: opMetrics.cache_hit
        };
      }
      await loadMetrics();
    } catch (e) {
      console.error('Failed to get timeline:', e);
    }
  }
  
  function handleFollowerClick(followerId) {
    timelineUserId = followerId;
    handleGetTimeline();
  }

  async function handleUpdateConfig() {
    try {
      await updateConfig('celebrity-threshold', newThreshold);
      await loadConfig();
      // Reload users as celebrity status might have changed
      await loadSampleUsers();
    } catch (e) {
      console.error('Failed to update config:', e);
    }
  }

  async function handleClearMetrics() {
    try {
      await clearMetrics();
      await loadMetrics();
      lastOperationResult = null;
    } catch (e) {
      console.error('Failed to clear metrics:', e);
    }
  }
  
  function resetDemo() {
    demoPhase = 'select';
    selectedPersona = null;
    timeline = [];
    lastOperationResult = null;
    isAnimating = false;
    autoLoadedTimeline = false;
    tweetContent = '';
  }
  
  function changeStrategy(strategyId) {
    selectedStrategy = strategyId;
    // Update tweet content to reflect strategy
    if (selectedAuthor) {
      tweetContent = `Hello from ${selectedAuthor.username}! Testing ${strategies.find(s => s.id === strategyId)?.name || 'fan-out'}.`;
    }
  }

  function formatDuration(str) {
    if (!str) return '-';
    return str;
  }

  function getStrategyData(strategyId) {
    return metrics?.by_strategy?.[strategyId] || {};
  }
  
  // Get expected behavior text for current strategy and persona
  function getExpectedBehavior() {
    const strategy = strategies.find(s => s.id === selectedStrategy);
    if (!strategy) return '';
    return selectedPersona === 'celebrity' ? strategy.celebrityBehavior : strategy.regularBehavior;
  }
</script>

<div class="min-h-screen bg-brand-cream">
  <!-- Hero Section -->
  <header class="section pb-12 md:pb-16">
    <div class="max-w-4xl mx-auto px-6 text-center">
      <p class="text-sm uppercase tracking-widest text-gray-500 mb-4">A System Design Deep Dive</p>
      <h1 class="text-5xl md:text-7xl mb-6 text-brand-blue">The Twitter Timeline Problem</h1>
      <p class="text-xl md:text-2xl text-gray-600 max-w-2xl mx-auto leading-relaxed">
        Elon tweets. 100 million followers need to see it. You have 5 seconds. What do you do?
      </p>
      <div class="mt-8 flex items-center justify-center gap-2 text-sm text-gray-500">
        <span>Built by</span>
        <span class="font-medium text-brand-blue">Ritik Sahni</span>
        <span class="mx-2">·</span>
        <span>Inspired by</span>
        <a href="https://dataintensive.net/" target="_blank" rel="noopener" class="font-medium underline underline-offset-2">
          DDIA
        </a>
      </div>
    </div>
  </header>

  <!-- Connection Status Banner -->
  {#if !connected}
    <div class="bg-amber-50 border-y border-amber-200 py-4">
      <div class="max-w-4xl mx-auto px-6 flex items-center justify-center gap-4">
        <div class="flex items-center gap-2">
          <span class="h-2 w-2 rounded-full bg-amber-500 animate-pulse"></span>
          <span class="text-amber-800">Server not connected</span>
        </div>
        <code class="bg-amber-100 px-3 py-1 rounded text-sm text-amber-900">make run</code>
        <button class="btn btn-secondary text-sm" on:click={checkConnection}>Retry</button>
      </div>
    </div>
  {/if}

  <!-- The Problem Section -->
  <section class="section bg-white border-y border-gray-100">
    <div class="max-w-4xl mx-auto px-6">
      <h2 class="text-3xl md:text-4xl mb-8 text-center">Why This Is Hard</h2>
      
      <div class="prose max-w-none">
        <p class="text-lg text-gray-600 text-center max-w-3xl mx-auto mb-12">
          Your timeline isn't just a database query. It's a <strong>distributed cache</strong> that needs to feel instant 
          while handling 500 million tweets per day.
        </p>

        <div class="grid md:grid-cols-2 gap-8 mb-12">
          <div class="bg-gray-50 rounded-2xl p-6">
            <div class="flex items-center gap-3 mb-4">
              <div class="w-10 h-10 rounded-xl bg-indigo-100 flex items-center justify-center">
                <svg class="w-5 h-5 text-indigo-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
              </div>
              <h3 class="text-xl mb-0">The Scale</h3>
            </div>
            <ul class="space-y-2 text-gray-600">
              <li><strong>500M tweets/day</strong> at Twitter's peak</li>
              <li>Average user follows <strong>~200 accounts</strong></li>
              <li>Top accounts: <strong>100M+ followers</strong> each</li>
              <li>Timeline must load in <strong>&lt;200ms</strong></li>
            </ul>
          </div>
          <div class="bg-gray-50 rounded-2xl p-6">
            <div class="flex items-center gap-3 mb-4">
              <div class="w-10 h-10 rounded-xl bg-amber-100 flex items-center justify-center">
                <svg class="w-5 h-5 text-amber-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
              </div>
              <h3 class="text-xl mb-0">The Trade-off</h3>
            </div>
            <p class="text-gray-600">
              Fast writes <em>or</em> fast reads. Pick one. Optimizing for posting speed 
              slows down timeline loads. Optimizing for reads makes posting expensive. 
              <strong>Users read 100x more than they write</strong> — that's your hint.
            </p>
          </div>
        </div>

        <blockquote class="text-xl text-gray-600">
          "Twitter's home timeline is a cache of tweets... The system has to do a lot of work to maintain this cache."
          <footer class="text-base text-gray-500 mt-2 not-italic">— Martin Kleppmann, Designing Data-Intensive Applications</footer>
        </blockquote>
      </div>
    </div>
  </section>

  <!-- Three Strategies Section -->
  <section class="section">
    <div class="max-w-6xl mx-auto px-6">
      <h2 class="text-3xl md:text-4xl mb-4 text-center">Three Solutions</h2>
      <p class="text-gray-600 text-center mb-12 max-w-2xl mx-auto">
        Same problem, different trade-offs. Select a strategy to test it in the demo below.
      </p>

      <div class="grid md:grid-cols-3 gap-6">
        {#each strategies as strategy}
          <button
            class="strategy-card text-left cursor-pointer {selectedStrategy === strategy.id ? 'ring-2 ring-brand-blue ring-offset-2' : ''}"
            on:click={() => selectedStrategy = strategy.id}
          >
            <div class="flex items-start justify-between mb-3">
              <div>
                <h3 class="text-xl mb-0">{strategy.name}</h3>
                <span class="text-sm text-gray-500">{strategy.subtitle}</span>
              </div>
              {#if strategy.badge}
                <span class="bg-amber-100 text-amber-700 text-xs px-2 py-1 rounded-full font-medium">{strategy.badge}</span>
              {:else if selectedStrategy === strategy.id}
                <span class="bg-brand-blue text-white text-xs px-2 py-1 rounded-full">Selected</span>
              {/if}
            </div>
            
            <p class="text-brand-blue font-medium text-sm mb-3">{strategy.tagline}</p>
            <p class="text-gray-600 text-sm mb-4">{strategy.description}</p>
            
            <div class="flex gap-3">
              <div class="flex-1 bg-gray-50 rounded-lg p-3 text-center">
                <span class="text-xs text-gray-500 block mb-1">Write</span>
                <span class="text-sm font-medium {strategy.writeLabel === 'Fast' ? 'text-emerald-600' : strategy.writeLabel === 'Slow' ? 'text-red-500' : 'text-amber-600'}">{strategy.writeLabel}</span>
                <code class="block text-xs text-gray-400 mt-1">{strategy.writeComplexity}</code>
              </div>
              <div class="flex-1 bg-gray-50 rounded-lg p-3 text-center">
                <span class="text-xs text-gray-500 block mb-1">Read</span>
                <span class="text-sm font-medium {strategy.readLabel === 'Fast' ? 'text-emerald-600' : strategy.readLabel === 'Slow' ? 'text-red-500' : 'text-amber-600'}">{strategy.readLabel}</span>
                <code class="block text-xs text-gray-400 mt-1">{strategy.readComplexity}</code>
              </div>
            </div>
          </button>
        {/each}
      </div>
    </div>
  </section>

  <!-- Interactive Demo Section -->
  <section class="section bg-white border-y border-gray-100">
    <div class="max-w-6xl mx-auto px-6">
      <h2 class="text-3xl md:text-4xl mb-4 text-center">See It In Action</h2>
      
      {#if !connected}
        <!-- Disconnected State -->
        <div class="card max-w-2xl mx-auto text-center py-12">
          <div class="w-16 h-16 rounded-2xl bg-amber-100 flex items-center justify-center mx-auto mb-6">
            <svg class="w-8 h-8 text-amber-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
            </svg>
          </div>
          <h3 class="text-2xl mb-4">Server Required for Live Demo</h3>
          <p class="text-gray-600 mb-6">
            The interactive demo needs the Go backend running to post tweets and see real fan-out happen.
          </p>
          
          <div class="bg-gray-50 rounded-xl p-6 mb-6 text-left">
            <p class="text-sm text-gray-500 mb-4 text-center">Here's what happens when you post a tweet:</p>
            <div class="flex flex-wrap items-center justify-center gap-2 sm:gap-4 text-sm">
              <div class="bg-indigo-100 text-indigo-700 px-3 sm:px-4 py-2 rounded-lg font-medium">You tweet</div>
              <svg class="w-5 h-5 text-gray-400 hidden sm:block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7l5 5m0 0l-5 5m5-5H6" />
              </svg>
              <div class="bg-gray-200 text-gray-700 px-3 sm:px-4 py-2 rounded-lg">Find followers</div>
              <svg class="w-5 h-5 text-gray-400 hidden sm:block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7l5 5m0 0l-5 5m5-5H6" />
              </svg>
              <div class="bg-emerald-100 text-emerald-700 px-3 sm:px-4 py-2 rounded-lg font-medium">Push to caches</div>
            </div>
          </div>
          
          <div class="space-y-3">
            <p class="text-sm text-gray-500">Start the server:</p>
            <code class="bg-gray-900 text-emerald-400 px-6 py-3 rounded-lg text-sm block font-mono">make run</code>
            <button class="btn btn-primary mt-4" on:click={checkConnection}>
              <svg class="w-4 h-4 mr-2 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
              Retry Connection
            </button>
          </div>
        </div>
      {:else if sampleUsers.length === 0}
        <!-- No Data State -->
        <div class="card max-w-2xl mx-auto text-center py-12">
          <div class="w-16 h-16 rounded-2xl bg-indigo-100 flex items-center justify-center mx-auto mb-6">
            <svg class="w-8 h-8 text-indigo-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
            </svg>
          </div>
          <h3 class="text-2xl mb-4">Seed the Database</h3>
          <p class="text-gray-600 mb-6">
            Create sample users with followers to see the fan-out demo in action.
          </p>
          <code class="bg-gray-900 text-emerald-400 px-6 py-3 rounded-lg text-sm block font-mono mb-4">./bin/fanout seed</code>
          <button class="btn btn-secondary" on:click={loadSampleUsers}>Refresh Users</button>
        </div>
      {:else if !celebrityUser}
        <!-- No Celebrity Warning -->
        <div class="card max-w-2xl mx-auto text-center py-8 mb-8 border-amber-200 bg-amber-50">
          <div class="flex items-start gap-4 text-left">
            <div class="w-10 h-10 rounded-xl bg-amber-100 flex items-center justify-center flex-shrink-0">
              <svg class="w-5 h-5 text-amber-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            </div>
            <div>
              <h3 class="text-lg font-medium text-amber-800 mb-2">No Celebrity Users Found</h3>
              <p class="text-sm text-amber-700 mb-3">
                All users have fewer than <strong>{config.celebrity_threshold?.toLocaleString() || '10,000'} followers</strong> (your celebrity threshold). 
                The Hybrid strategy won't behave differently from Fan-Out Write.
              </p>
              <p class="text-sm text-amber-600">
                <strong>Fix:</strong> Either lower the celebrity threshold below, or re-seed with more followers:
              </p>
              <code class="bg-amber-100 text-amber-800 px-3 py-1 rounded text-xs mt-2 inline-block font-mono">./bin/fanout seed --followers 50000</code>
            </div>
          </div>
        </div>
      {:else}
        <!-- Main Demo -->
        <p class="text-gray-600 text-center mb-8 max-w-2xl mx-auto">
          Compare how strategies behave differently for <strong>regular users</strong> vs <strong>celebrities</strong>.
        </p>
        
        {#if demoPhase === 'select'}
          <!-- Step 1: Choose Persona -->
          <div class="max-w-3xl mx-auto">
            <h3 class="text-xl text-center mb-6 text-gray-700">Who's tweeting?</h3>
            
            <div class="grid sm:grid-cols-2 gap-6 mb-8">
              <!-- Regular User Card -->
              <button
                class="card p-6 text-left cursor-pointer transition-all hover:shadow-lg
                  {selectedPersona === 'regular' ? 'ring-2 ring-brand-blue ring-offset-2' : ''}"
                on:click={() => selectPersona('regular')}
              >
                <div class="flex items-center gap-4 mb-4">
                  <div class="w-14 h-14 rounded-full bg-indigo-100 flex items-center justify-center">
                    <svg class="w-7 h-7 text-indigo-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                    </svg>
                  </div>
                  <div>
                    <h4 class="text-xl mb-0">Regular User</h4>
                    <p class="text-gray-500 text-sm">{regularUser?.username || 'user'}</p>
                  </div>
                </div>
                
                <div class="bg-gray-50 rounded-xl p-4 mb-4">
                  <div class="text-3xl font-bold text-brand-blue">{regularUser?.follower_count?.toLocaleString() || 0}</div>
                  <div class="text-sm text-gray-500">followers</div>
                </div>
                
                <p class="text-sm text-gray-600">
                  Most users fall here. Fan-out is manageable.
                </p>
              </button>
              
              <!-- Celebrity Card -->
              <button
                class="card p-6 text-left cursor-pointer transition-all hover:shadow-lg
                  {selectedPersona === 'celebrity' ? 'ring-2 ring-brand-blue ring-offset-2' : ''}"
                on:click={() => selectPersona('celebrity')}
              >
                <div class="flex items-center gap-4 mb-4">
                  <div class="w-14 h-14 rounded-full bg-amber-100 flex items-center justify-center">
                    <svg class="w-7 h-7 text-amber-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
                    </svg>
                  </div>
                  <div>
                    <h4 class="text-xl mb-0">Celebrity</h4>
                    <p class="text-gray-500 text-sm">{celebrityUser?.username || 'celebrity'}</p>
                  </div>
                </div>
                
                <div class="bg-amber-50 rounded-xl p-4 mb-4">
                  <div class="text-3xl font-bold text-amber-600">{celebrityUser?.follower_count?.toLocaleString() || 0}</div>
                  <div class="text-sm text-amber-700">followers</div>
                </div>
                
                <p class="text-sm text-gray-600">
                  Above {config.celebrity_threshold?.toLocaleString() || '10,000'} threshold. This is where strategies diverge.
                </p>
              </button>
            </div>
            
            <!-- Strategy Quick Reference -->
            <div class="bg-gray-50 rounded-2xl p-6">
              <h4 class="text-lg mb-4 text-center">What to expect with each strategy:</h4>
              <div class="overflow-x-auto">
                <table class="w-full text-sm">
                  <thead>
                    <tr class="text-left">
                      <th class="pb-3 pr-4 font-medium text-gray-500">Strategy</th>
                      <th class="pb-3 pr-4 font-medium text-gray-500">Regular User</th>
                      <th class="pb-3 font-medium text-gray-500">Celebrity</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-200">
                    {#each strategies as s}
                      <tr>
                        <td class="py-3 pr-4 font-medium">{s.name}</td>
                        <td class="py-3 pr-4 text-gray-600">{s.regularBehavior}</td>
                        <td class="py-3 text-gray-600">{s.celebrityBehavior}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        {:else}
          <!-- Step 2+: Posting and Results -->
          <div class="grid lg:grid-cols-2 gap-6 lg:gap-8">
            <!-- Left: Controls -->
            <div class="space-y-4 lg:space-y-6">
              <!-- Author Info -->
              <div class="card">
                <div class="flex items-center justify-between mb-4">
                  <div class="flex items-center gap-3">
                    <div class="w-12 h-12 rounded-full flex items-center justify-center
                      {selectedPersona === 'celebrity' ? 'bg-amber-100' : 'bg-indigo-100'}">
                      {#if selectedPersona === 'celebrity'}
                        <svg class="w-6 h-6 text-amber-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
                        </svg>
                      {:else}
                        <svg class="w-6 h-6 text-indigo-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                        </svg>
                      {/if}
                    </div>
                    <div>
                      <p class="font-medium text-gray-900">{selectedAuthor?.username}</p>
                      <p class="text-sm text-gray-500">{selectedAuthor?.follower_count?.toLocaleString()} followers</p>
                    </div>
                  </div>
                  <button class="text-sm text-gray-500 hover:text-gray-700" on:click={resetDemo}>
                    Change
                  </button>
                </div>
                
                <!-- Strategy Selector -->
                <div class="mb-4">
                  <p class="block text-sm font-medium text-gray-700 mb-2">Strategy</p>
                  <div class="grid grid-cols-3 gap-2" role="group" aria-label="Select strategy">
                    {#each strategies as s}
                      <button
                        class="p-3 rounded-xl border-2 text-center transition-all text-sm
                          {selectedStrategy === s.id ? 'border-brand-blue bg-indigo-50' : 'border-gray-100 hover:border-gray-200'}"
                        on:click={() => changeStrategy(s.id)}
                        disabled={demoPhase === 'result'}
                      >
                        <div class="font-medium truncate">{s.name.split(' ')[0]}</div>
                        <div class="text-xs text-gray-500 truncate">{s.subtitle}</div>
                      </button>
                    {/each}
                  </div>
                </div>
                
                <!-- Expected Behavior -->
                <div class="bg-gray-50 rounded-xl p-4 mb-4">
                  <p class="text-xs text-gray-500 uppercase tracking-wide mb-1">Expected Behavior</p>
                  <p class="text-sm text-gray-700">{getExpectedBehavior()}</p>
                </div>
                
                <!-- Tweet Composer -->
                {#if demoPhase === 'posting'}
                  <div class="space-y-4">
                    <div>
                      <label for="tweet-content" class="block text-sm font-medium text-gray-700 mb-2">Tweet</label>
                      <textarea 
                        id="tweet-content"
                        bind:value={tweetContent} 
                        class="input" 
                        rows="2" 
                        placeholder="What's happening?"
                      ></textarea>
                    </div>
                    
                    <button 
                      class="btn btn-cta w-full" 
                      on:click={handlePostTweet}
                      disabled={!tweetContent.trim() || isAnimating}
                    >
                      {#if isAnimating}
                        <svg class="w-4 h-4 mr-2 inline animate-spin" fill="none" viewBox="0 0 24 24">
                          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        Posting...
                      {:else}
                        <svg class="w-4 h-4 mr-2 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
                        </svg>
                        Post Tweet
                      {/if}
                    </button>
                  </div>
                {/if}
              </div>
              
              <!-- Result: Follower Timeline -->
              {#if demoPhase === 'result'}
                <div class="card">
                  <div class="flex items-center justify-between mb-4">
                    <h3 class="text-lg mb-0">Follower's Timeline</h3>
                    <span class="text-xs px-2 py-1 rounded-full
                      {lastOperationResult?.cacheHit ? 'bg-emerald-100 text-emerald-700' : 'bg-amber-100 text-amber-700'}">
                      Cache {lastOperationResult?.cacheHit ? 'Hit' : 'Miss'}
                    </span>
                  </div>
                  
                  <p class="text-sm text-gray-600 mb-4">
                    {#if selectedStrategy === 'fanout_write'}
                      Tweet was pre-delivered to this timeline cache.
                    {:else if selectedStrategy === 'fanout_read'}
                      Timeline built by querying followed accounts.
                    {:else}
                      {selectedPersona === 'celebrity' ? 'Celebrity tweet fetched on-demand.' : 'Tweet was pre-delivered.'}
                    {/if}
                  </p>
                  
                  {#if timeline.length > 0}
                    <div class="border border-gray-200 rounded-xl overflow-hidden">
                      <div class="max-h-48 overflow-y-auto">
                        {#each timeline as tweet}
                          <div class="p-3 border-b border-gray-100 last:border-b-0">
                            <div class="flex justify-between text-sm mb-1">
                              <span class="font-medium text-brand-blue">@{tweet.username || `user_${tweet.user_id}`}</span>
                              <span class="text-gray-400 text-xs">{new Date(tweet.created_at).toLocaleTimeString()}</span>
                            </div>
                            <p class="text-sm text-gray-700">{tweet.content}</p>
                            {#if tweet.user_id === selectedAuthor?.id}
                              <span class="inline-block mt-1 text-xs bg-emerald-100 text-emerald-700 px-2 py-0.5 rounded">Just delivered!</span>
                            {/if}
                          </div>
                        {/each}
                      </div>
                    </div>
                  {:else}
                    <div class="text-center py-6 text-gray-500 text-sm">
                      No tweets in timeline
                    </div>
                  {/if}
                  
                  <div class="flex gap-3 mt-4">
                    <button class="btn btn-secondary flex-1" on:click={resetDemo}>
                      Try Different Persona
                    </button>
                    <button class="btn btn-primary flex-1" on:click={() => { demoPhase = 'posting'; autoLoadedTimeline = false; }}>
                      Post Another
                    </button>
                  </div>
                </div>
              {/if}
            </div>
            
            <!-- Right: Visualization -->
            <div class="space-y-4 lg:space-y-6">
              <div class="card p-4">
                <div class="flex items-center justify-between mb-4">
                  <h3 class="text-lg mb-0">Fan-Out Visualization</h3>
                  {#if lastOperationResult?.type === 'write'}
                    <span class="text-sm text-gray-500">
                      <span class="font-mono text-brand-blue">{lastOperationResult.latency}</span>
                    </span>
                  {/if}
                </div>
                
                <FanOutVisualizer
                  bind:this={visualizerRef}
                  author={selectedAuthor}
                  followers={authorFollowers}
                  fanOutCount={lastOperationResult?.fanOutCount || 0}
                  {isAnimating}
                  {tweetContent}
                  onFollowerClick={handleFollowerClick}
                />
              </div>
              
              <!-- Write Operation Result -->
              {#if lastOperationResult?.type === 'write'}
                <div class="bg-gray-50 rounded-2xl p-6">
                  <div class="grid grid-cols-3 gap-4 text-center">
                    <div>
                      <p class="text-xs text-gray-500 mb-1">Strategy</p>
                      <p class="font-medium text-sm">{strategies.find(s => s.id === selectedStrategy)?.name}</p>
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 mb-1">Write Latency</p>
                      <p class="font-mono text-brand-blue font-medium">{lastOperationResult.latency}</p>
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 mb-1">Fan-Out</p>
                      <p class="font-medium">{lastOperationResult.fanOutCount?.toLocaleString() || 0}</p>
                    </div>
                  </div>
                </div>
              {/if}
            </div>
          </div>
        {/if}
      {/if}
    </div>
  </section>

  <!-- Metrics Section -->
  <section class="section">
    <div class="max-w-6xl mx-auto px-6">
      <div class="flex items-center justify-between mb-8">
        <div>
          <h2 class="text-3xl md:text-4xl mb-2">Performance Metrics</h2>
          <p class="text-gray-600">Live metrics from your test operations</p>
        </div>
        <button class="btn btn-secondary text-sm" on:click={handleClearMetrics}>
          <svg class="w-4 h-4 mr-1 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
          Clear
        </button>
      </div>

      <!-- Summary Stats -->
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
        <div class="bg-white rounded-2xl p-6 border border-gray-100">
          <p class="text-sm text-gray-500 mb-1">Total Operations</p>
          <p class="text-3xl font-serif text-brand-blue">
            {(metrics?.total_writes || 0) + (metrics?.total_reads || 0)}
          </p>
        </div>
        <div class="bg-white rounded-2xl p-6 border border-gray-100">
          <p class="text-sm text-gray-500 mb-1">Tweets Posted</p>
          <p class="text-3xl font-serif text-brand-blue">{metrics?.total_writes || 0}</p>
        </div>
        <div class="bg-white rounded-2xl p-6 border border-gray-100">
          <p class="text-sm text-gray-500 mb-1">Timelines Loaded</p>
          <p class="text-3xl font-serif text-brand-blue">{metrics?.total_reads || 0}</p>
        </div>
        <div class="bg-white rounded-2xl p-6 border border-gray-100">
          <p class="text-sm text-gray-500 mb-1">Celebrity Threshold</p>
          <p class="text-3xl font-serif text-brand-blue">{config.celebrity_threshold?.toLocaleString() || '-'}</p>
        </div>
      </div>

      <!-- Chart -->
      <div class="card mb-8">
        <h3 class="text-2xl mb-6">P95 Latency Comparison</h3>
        <p class="text-gray-600 text-sm mb-6">
          P95 = 95th percentile. Shows how the slowest 5% of requests perform — a better metric than averages because it exposes tail latency.
        </p>
        <BarChart {metrics} title="" />
      </div>

      <!-- Strategy Breakdown -->
      <div class="grid md:grid-cols-3 gap-6">
        {#each strategies as strategy}
          {@const data = getStrategyData(strategy.id)}
          <div class="bg-white rounded-2xl p-6 border border-gray-100 {selectedStrategy === strategy.id ? 'ring-2 ring-brand-blue' : ''}">
            <div class="flex items-center justify-between mb-4">
              <h4 class="text-xl">{strategy.name}</h4>
              {#if selectedStrategy === strategy.id}
                <span class="h-2 w-2 rounded-full bg-brand-blue"></span>
              {/if}
            </div>
            <div class="space-y-3 text-sm">
              <div class="flex justify-between py-2 border-b border-gray-50">
                <span class="text-gray-500">Writes</span>
                <span class="font-medium">{data?.write_count || 0}</span>
              </div>
              <div class="flex justify-between py-2 border-b border-gray-50">
                <span class="text-gray-500">Reads</span>
                <span class="font-medium">{data?.read_count || 0}</span>
              </div>
              <div class="flex justify-between py-2 border-b border-gray-50">
                <span class="text-gray-500">Write P95</span>
                <span class="font-medium text-brand-blue">{formatDuration(data?.write_latency_p95)}</span>
              </div>
              <div class="flex justify-between py-2 border-b border-gray-50">
                <span class="text-gray-500">Read P95</span>
                <span class="font-medium text-brand-blue">{formatDuration(data?.read_latency_p95)}</span>
              </div>
              <div class="flex justify-between py-2 border-b border-gray-50">
                <span class="text-gray-500">Avg Fan-Out</span>
                <span class="font-medium">{data?.avg_fan_out_count?.toFixed(1) || '0'}</span>
              </div>
              <div class="flex justify-between py-2">
                <span class="text-gray-500">Cache Hit Rate</span>
                <span class="font-medium">{((data?.cache_hit_rate || 0) * 100).toFixed(0)}%</span>
              </div>
            </div>
          </div>
        {/each}
      </div>
    </div>
  </section>

  <!-- Configuration Section -->
  <section class="section bg-white border-t border-gray-100">
    <div class="max-w-4xl mx-auto px-6">
      <h2 class="text-3xl md:text-4xl mb-4 text-center">Tune the Algorithm</h2>
      <p class="text-gray-600 text-center mb-12 max-w-2xl mx-auto">
        At what follower count does someone become a "celebrity"? Change it and watch the hybrid strategy adapt.
      </p>

      <div class="card max-w-lg mx-auto">
        <div class="mb-6">
          <label for="celebrity-threshold" class="block text-sm font-medium text-gray-700 mb-2">Celebrity Threshold</label>
          <p class="text-sm text-gray-500 mb-4">
            Users with this many followers skip the push model. Their tweets get merged on-demand instead 
            of updating millions of caches. Twitter uses ~10,000 as their cutoff.
          </p>
          <div class="flex gap-3">
            <input id="celebrity-threshold" type="number" bind:value={newThreshold} class="input" min="1" />
            <button class="btn btn-primary" on:click={handleUpdateConfig}>Apply</button>
          </div>
        </div>
        
        <div class="grid grid-cols-2 gap-4 pt-6 border-t border-gray-100">
          <div>
            <p class="text-sm text-gray-500">Max Cached Tweets</p>
            <p class="text-lg font-medium">{config.timeline_cache_size?.toLocaleString() || '-'}</p>
          </div>
          <div>
            <p class="text-sm text-gray-500">Tweets Per Page</p>
            <p class="text-lg font-medium">{config.timeline_page_size || '-'}</p>
          </div>
        </div>
      </div>
    </div>
  </section>

  <!-- Footer -->
  <footer class="py-16 border-t border-gray-200">
    <div class="max-w-4xl mx-auto px-6 text-center">
      <p class="text-sm uppercase tracking-widest text-gray-400 mb-4">An Engineering Project by</p>
      <h2 class="text-3xl md:text-4xl mb-4">Ritik Sahni</h2>
      <p class="text-gray-600 mb-8 max-w-lg mx-auto">
        I built this to understand — and explain — one of the most elegant trade-offs in distributed systems. 
        The concepts come from 
        <a href="https://dataintensive.net/" target="_blank" rel="noopener" class="underline underline-offset-2">
          Designing Data-Intensive Applications
        </a>.
      </p>
      
      <div class="flex items-center justify-center gap-6 text-sm text-gray-500">
        <a href="https://github.com/ritiksahni" target="_blank" rel="noopener" class="hover:text-brand-blue transition-colors flex items-center gap-1">
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24"><path d="M12 0C5.374 0 0 5.373 0 12c0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23A11.509 11.509 0 0112 5.803c1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576C20.566 21.797 24 17.3 24 12c0-6.627-5.373-12-12-12z"/></svg>
          GitHub
        </a>
        <span class="text-gray-300">·</span>
        <a href="https://x.com/ritiksahni22" target="_blank" rel="noopener" class="hover:text-brand-blue transition-colors flex items-center gap-1">
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z"/></svg>
          X
        </a>
        <span class="text-gray-300">·</span>
        <a href="mailto:ritik@ritiksahni.com" class="hover:text-brand-blue transition-colors flex items-center gap-1">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" /></svg>
          Email
        </a>
      </div>
      
      <p class="mt-8 text-xs text-gray-400">
        Built with Go, PostgreSQL, Redis, and Svelte
      </p>
    </div>
  </footer>
</div>
