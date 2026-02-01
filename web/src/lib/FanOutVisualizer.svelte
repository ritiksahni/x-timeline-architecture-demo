<script>
  import { onMount } from 'svelte';
  
  export let author = null;
  export let followers = [];
  export let fanOutCount = 0;
  export let isAnimating = false;
  export let tweetContent = '';
  export let onFollowerClick = (followerId) => {};
  
  // Animation state
  let animationPhase = 'idle'; // 'idle', 'tweet-appear', 'lines-draw', 'nodes-pulse', 'complete'
  let displayedCount = 0;
  let activeLines = [];
  let pulsedNodes = [];
  
  // Follower positions (calculated based on count)
  $: visibleFollowers = followers.slice(0, 5);
  $: extraCount = Math.max(0, fanOutCount - visibleFollowers.length);
  
  // Reset animation state when not animating
  $: if (!isAnimating) {
    animationPhase = 'idle';
    displayedCount = 0;
    activeLines = [];
    pulsedNodes = [];
  }
  
  export async function startAnimation() {
    if (!author || fanOutCount === 0) return;
    
    animationPhase = 'tweet-appear';
    displayedCount = 0;
    activeLines = [];
    pulsedNodes = [];
    
    // Phase 1: Tweet appears (300ms)
    await delay(300);
    
    // Phase 2: Lines draw to followers (staggered 150ms each)
    animationPhase = 'lines-draw';
    for (let i = 0; i < visibleFollowers.length; i++) {
      activeLines = [...activeLines, i];
      await delay(150);
    }
    
    // Phase 3: Nodes pulse (staggered 100ms each)
    animationPhase = 'nodes-pulse';
    for (let i = 0; i < visibleFollowers.length; i++) {
      pulsedNodes = [...pulsedNodes, i];
      await delay(100);
    }
    
    // Phase 4: Counter increments
    const increment = Math.max(1, Math.floor(fanOutCount / 20));
    while (displayedCount < fanOutCount) {
      displayedCount = Math.min(displayedCount + increment, fanOutCount);
      await delay(30);
    }
    
    animationPhase = 'complete';
  }
  
  function delay(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }
  
  // Calculate Y position for each follower node
  function getFollowerY(index, total) {
    const spacing = 280 / Math.max(total, 1);
    const startY = 60 + (280 - (total * spacing)) / 2;
    return startY + index * spacing + spacing / 2;
  }
</script>

<div class="fan-out-visualizer" class:animating={isAnimating}>
  <!-- Desktop/Tablet view -->
  <svg viewBox="0 0 600 400" class="w-full h-auto hidden sm:block">
    <!-- Background -->
    <rect x="0" y="0" width="600" height="400" fill="transparent" />
    
    <!-- Connection lines (drawn during animation) -->
    {#each visibleFollowers as follower, i}
      <line
        x1="150"
        y1="200"
        x2="450"
        y2={getFollowerY(i, visibleFollowers.length)}
        class="connection-line"
        class:active={activeLines.includes(i)}
        style="--delay: {i * 150}ms"
      />
    {/each}
    
    <!-- Extra followers indicator line -->
    {#if extraCount > 0}
      <line
        x1="150"
        y1="200"
        x2="450"
        y2="360"
        class="connection-line dashed"
        class:active={activeLines.length === visibleFollowers.length}
      />
    {/if}
    
    <!-- Author node (left side) -->
    <g class="author-node" transform="translate(80, 160)">
      <circle
        cx="70"
        cy="40"
        r="45"
        class="node-circle author"
        class:tweet-sent={animationPhase !== 'idle'}
      />
      <!-- User icon -->
      <g transform="translate(50, 20)">
        <circle cx="20" cy="12" r="10" fill="currentColor" class="icon-fill" />
        <path d="M4 38c0-8.8 7.2-16 16-16s16 7.2 16 16" fill="currentColor" class="icon-fill" />
      </g>
      
      <!-- Author label -->
      <text x="70" y="105" text-anchor="middle" class="node-label">
        {author?.username || 'Author'}
      </text>
      <text x="70" y="122" text-anchor="middle" class="node-sublabel">
        {author?.follower_count?.toLocaleString() || 0} followers
      </text>
    </g>
    
    <!-- Tweet bubble (appears during animation) -->
    {#if animationPhase !== 'idle'}
      <g class="tweet-bubble" class:visible={animationPhase !== 'idle'}>
        <rect
          x="120"
          y="240"
          width="120"
          height="50"
          rx="8"
          class="tweet-rect"
        />
        <text x="180" y="262" text-anchor="middle" class="tweet-text">
          Tweet
        </text>
        <text x="180" y="278" text-anchor="middle" class="tweet-subtext">
          {tweetContent.slice(0, 15)}{tweetContent.length > 15 ? '...' : ''}
        </text>
      </g>
    {/if}
    
    <!-- Follower nodes (right side) -->
    {#each visibleFollowers as follower, i}
      {@const y = getFollowerY(i, visibleFollowers.length)}
      <g 
        class="follower-node cursor-pointer"
        transform="translate(420, {y - 25})"
        on:click={() => onFollowerClick(follower)}
        on:keydown={(e) => e.key === 'Enter' && onFollowerClick(follower)}
        role="button"
        tabindex="0"
      >
        <rect
          x="0"
          y="0"
          width="160"
          height="50"
          rx="8"
          class="node-rect"
          class:pulsed={pulsedNodes.includes(i)}
          style="--delay: {i * 100}ms"
        />
        <!-- Timeline icon -->
        <g transform="translate(12, 13)">
          <rect x="0" y="0" width="24" height="3" rx="1.5" fill="currentColor" class="icon-fill-muted" />
          <rect x="0" y="6" width="18" height="3" rx="1.5" fill="currentColor" class="icon-fill-muted" />
          <rect x="0" y="12" width="24" height="3" rx="1.5" fill="currentColor" class="icon-fill-muted" />
          <rect x="0" y="18" width="14" height="3" rx="1.5" fill="currentColor" class="icon-fill-muted" />
        </g>
        <text x="45" y="20" class="follower-label">User {follower}</text>
        <text x="45" y="36" class="follower-sublabel">Timeline Cache</text>
        
        <!-- Checkmark when pulsed -->
        {#if pulsedNodes.includes(i)}
          <g transform="translate(130, 12)" class="checkmark">
            <circle cx="12" cy="12" r="10" class="check-circle" />
            <path d="M8 12l3 3 5-6" stroke="white" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round" />
          </g>
        {/if}
      </g>
    {/each}
    
    <!-- Extra followers indicator -->
    {#if extraCount > 0}
      <g class="extra-indicator" transform="translate(420, 335)">
        <rect
          x="0"
          y="0"
          width="160"
          height="50"
          rx="8"
          class="node-rect extra"
          class:pulsed={animationPhase === 'complete'}
        />
        <text x="80" y="30" text-anchor="middle" class="extra-label">
          +{extraCount.toLocaleString()} more
        </text>
      </g>
    {/if}
    
    <!-- Fan-out counter -->
    {#if animationPhase !== 'idle'}
      <g class="counter" transform="translate(250, 20)">
        <text x="50" y="25" text-anchor="middle" class="counter-label">Timelines Updated</text>
        <text x="50" y="55" text-anchor="middle" class="counter-value">
          {displayedCount.toLocaleString()}
        </text>
      </g>
    {/if}
  </svg>
  
  <!-- Mobile view (vertical layout) -->
  <div class="sm:hidden">
    <!-- Mobile counter -->
    {#if animationPhase !== 'idle'}
      <div class="text-center mb-4 p-4 bg-indigo-50 rounded-xl">
        <p class="text-xs text-gray-500 uppercase tracking-wide mb-1">Timelines Updated</p>
        <p class="text-3xl font-bold text-indigo-600 font-mono">{displayedCount.toLocaleString()}</p>
      </div>
    {/if}
    
    <!-- Mobile author -->
    <div class="flex flex-col items-center mb-4">
      <div class="w-16 h-16 rounded-full flex items-center justify-center mb-2 transition-all duration-300
        {animationPhase !== 'idle' ? 'bg-indigo-600' : 'bg-indigo-100'}">
        <svg class="w-8 h-8 {animationPhase !== 'idle' ? 'text-white' : 'text-indigo-600'}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
        </svg>
      </div>
      <p class="font-medium text-gray-900">{author?.username || 'Author'}</p>
      <p class="text-sm text-gray-500">{author?.follower_count?.toLocaleString() || 0} followers</p>
    </div>
    
    <!-- Mobile tweet bubble -->
    {#if animationPhase !== 'idle'}
      <div class="bg-indigo-600 text-white rounded-xl p-3 mx-8 mb-4 text-center animate-tweet-appear">
        <p class="text-sm font-medium">Tweet sent!</p>
        <p class="text-xs opacity-80 truncate">{tweetContent.slice(0, 30)}{tweetContent.length > 30 ? '...' : ''}</p>
      </div>
    {/if}
    
    <!-- Mobile arrow -->
    <div class="flex justify-center mb-4">
      <svg class="w-6 h-6 text-indigo-400 {animationPhase !== 'idle' ? 'animate-bounce' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 14l-7 7m0 0l-7-7m7 7V3" />
      </svg>
    </div>
    
    <!-- Mobile followers list -->
    <div class="space-y-2">
      {#each visibleFollowers as follower, i}
        <button
          class="w-full flex items-center justify-between p-3 rounded-xl border-2 transition-all cursor-pointer
            {pulsedNodes.includes(i) ? 'border-indigo-500 bg-indigo-50' : 'border-gray-100 bg-white'}"
          on:click={() => onFollowerClick(follower)}
        >
          <div class="flex items-center gap-3">
            <div class="w-8 h-8 rounded bg-gray-100 flex items-center justify-center">
              <svg class="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h16" />
              </svg>
            </div>
            <div class="text-left">
              <p class="text-sm font-medium text-gray-900">User {follower}</p>
              <p class="text-xs text-gray-500">Timeline Cache</p>
            </div>
          </div>
          {#if pulsedNodes.includes(i)}
            <div class="w-6 h-6 rounded-full bg-emerald-500 flex items-center justify-center">
              <svg class="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
              </svg>
            </div>
          {/if}
        </button>
      {/each}
      
      {#if extraCount > 0}
        <div class="p-3 rounded-xl border-2 border-dashed border-gray-200 text-center
          {animationPhase === 'complete' ? 'border-indigo-300 bg-indigo-50' : ''}">
          <p class="text-sm text-gray-500">+{extraCount.toLocaleString()} more timelines</p>
        </div>
      {/if}
    </div>
  </div>
  
  <!-- Legend -->
  <div class="legend">
    <div class="legend-item">
      <span class="legend-dot author"></span>
      <span>Author</span>
    </div>
    <div class="legend-item">
      <span class="legend-dot follower"></span>
      <span>Timeline Cache</span>
    </div>
  </div>
</div>

<style>
  .fan-out-visualizer {
    @apply w-full bg-white rounded-2xl p-6 border border-gray-100;
  }
  
  .fan-out-visualizer.animating {
    @apply ring-2 ring-indigo-200;
  }
  
  /* Connection lines */
  .connection-line {
    stroke: #e5e7eb;
    stroke-width: 2;
    stroke-dasharray: 300;
    stroke-dashoffset: 300;
    transition: stroke-dashoffset 400ms ease-out, stroke 200ms ease;
  }
  
  .connection-line.active {
    stroke-dashoffset: 0;
    stroke: #4F46E5;
  }
  
  .connection-line.dashed {
    stroke-dasharray: 8 4;
  }
  
  .connection-line.dashed.active {
    stroke-dashoffset: 0;
    stroke: #94a3b8;
  }
  
  /* Author node */
  .node-circle.author {
    fill: #EEF2FF;
    stroke: #4F46E5;
    stroke-width: 3;
    transition: all 300ms ease;
  }
  
  .node-circle.author.tweet-sent {
    fill: #4F46E5;
    transform-origin: center;
    animation: pulse-author 600ms ease-out;
  }
  
  .icon-fill {
    fill: #4F46E5;
    transition: fill 300ms ease;
  }
  
  .tweet-sent .icon-fill {
    fill: white;
  }
  
  .icon-fill-muted {
    fill: #94a3b8;
  }
  
  .node-label {
    @apply text-sm font-medium;
    fill: #1e1b4b;
  }
  
  .node-sublabel {
    @apply text-xs;
    fill: #64748b;
  }
  
  /* Tweet bubble */
  .tweet-bubble {
    opacity: 0;
    transform: translateY(10px);
    transition: all 300ms ease;
  }
  
  .tweet-bubble.visible {
    opacity: 1;
    transform: translateY(0);
  }
  
  .tweet-rect {
    fill: #4F46E5;
  }
  
  .tweet-text {
    fill: white;
    @apply text-sm font-medium;
  }
  
  .tweet-subtext {
    fill: rgba(255, 255, 255, 0.8);
    @apply text-xs;
  }
  
  /* Follower nodes */
  .follower-node {
    transition: transform 150ms ease;
  }
  
  .follower-node:hover {
    transform: translateX(4px);
  }
  
  .node-rect {
    fill: #f8fafc;
    stroke: #e2e8f0;
    stroke-width: 2;
    transition: all 300ms ease;
  }
  
  .node-rect.pulsed {
    fill: #EEF2FF;
    stroke: #4F46E5;
    animation: node-receive 400ms ease-out;
  }
  
  .node-rect.extra {
    fill: #f1f5f9;
    stroke-dasharray: 4 2;
  }
  
  .node-rect.extra.pulsed {
    fill: #EEF2FF;
    stroke: #4F46E5;
    stroke-dasharray: none;
  }
  
  .follower-label {
    @apply text-sm font-medium;
    fill: #334155;
  }
  
  .follower-sublabel {
    @apply text-xs;
    fill: #94a3b8;
  }
  
  .extra-label {
    @apply text-sm font-medium;
    fill: #64748b;
  }
  
  /* Checkmark */
  .checkmark {
    animation: check-appear 200ms ease-out;
  }
  
  .check-circle {
    fill: #10b981;
  }
  
  /* Counter */
  .counter-label {
    @apply text-xs uppercase tracking-wide;
    fill: #64748b;
  }
  
  .counter-value {
    @apply text-3xl font-bold;
    fill: #4F46E5;
    font-family: 'Fira Code', monospace;
  }
  
  /* Legend */
  .legend {
    @apply flex items-center justify-center gap-6 mt-4 text-sm text-gray-600;
  }
  
  .legend-item {
    @apply flex items-center gap-2;
  }
  
  .legend-dot {
    @apply w-3 h-3 rounded-full;
  }
  
  .legend-dot.author {
    @apply bg-indigo-600;
  }
  
  .legend-dot.follower {
    @apply bg-slate-200 border-2 border-slate-300;
  }
  
  /* Animations */
  @keyframes pulse-author {
    0% { transform: scale(1); }
    50% { transform: scale(1.1); }
    100% { transform: scale(1); }
  }
  
  @keyframes node-receive {
    0% { transform: scale(1); }
    50% { transform: scale(1.05); fill: #c7d2fe; }
    100% { transform: scale(1); }
  }
  
  @keyframes check-appear {
    0% { opacity: 0; transform: scale(0); }
    100% { opacity: 1; transform: scale(1); }
  }
  
  /* Reduced motion */
  @media (prefers-reduced-motion: reduce) {
    .connection-line,
    .node-circle,
    .node-rect,
    .tweet-bubble,
    .follower-node,
    .checkmark {
      animation: none;
      transition: none;
    }
    
    .connection-line.active {
      stroke-dashoffset: 0;
    }
  }
</style>
