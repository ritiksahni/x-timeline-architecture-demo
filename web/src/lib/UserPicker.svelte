<script>
  export let users = [];
  export let selectedUserId = null;
  export let onSelect = () => {};
  
  $: selectedUser = users.find(u => u.id === selectedUserId);
</script>

<div class="space-y-2">
  <label class="block text-sm font-medium text-gray-700">Select User</label>
  <select 
    bind:value={selectedUserId} 
    on:change={() => onSelect(selectedUserId)}
    class="select"
  >
    <option value={null}>Choose a user...</option>
    {#each users as user}
      <option value={user.id}>
        {user.username} 
        {user.is_celebrity ? '[Celebrity]' : ''} 
        ({user.follower_count.toLocaleString()} followers)
      </option>
    {/each}
  </select>
  
  {#if selectedUser}
    <div class="text-sm text-gray-600 bg-gray-50 rounded-lg p-3">
      <div class="flex items-center gap-2 mb-1">
        <span class="font-medium">{selectedUser.username}</span>
        {#if selectedUser.is_celebrity}
          <span class="text-xs bg-brand-blue/10 text-brand-blue px-2 py-0.5 rounded">Celebrity</span>
        {/if}
      </div>
      <p class="text-xs text-gray-500">
        {selectedUser.follower_count.toLocaleString()} followers
      </p>
    </div>
  {/if}
</div>
