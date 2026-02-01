const API_BASE = '/api';

export async function postTweet(userId, content, strategy) {
  const response = await fetch(`${API_BASE}/tweet`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      user_id: userId,
      content,
      strategy
    })
  });
  return response.json();
}

export async function getTimeline(userId, strategy, limit = 50) {
  const response = await fetch(
    `${API_BASE}/timeline/${userId}?strategy=${strategy}&limit=${limit}`
  );
  return response.json();
}

export async function getConfig() {
  const response = await fetch(`${API_BASE}/config`);
  return response.json();
}

export async function updateConfig(key, value) {
  const response = await fetch(`${API_BASE}/config`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, value })
  });
  return response.json();
}

export async function getMetrics() {
  const response = await fetch(`${API_BASE}/metrics`);
  return response.json();
}

export async function getRecentMetrics(limit = 100) {
  const response = await fetch(`${API_BASE}/metrics/recent?limit=${limit}`);
  return response.json();
}

export async function clearMetrics() {
  const response = await fetch(`${API_BASE}/metrics`, { method: 'DELETE' });
  return response.json();
}

export async function healthCheck() {
  try {
    const response = await fetch('/health');
    return response.ok;
  } catch {
    return false;
  }
}

export async function getSampleUsers() {
  const response = await fetch(`${API_BASE}/users/sample`);
  return response.json();
}

export async function getUserFollowers(userId) {
  const response = await fetch(`${API_BASE}/users/${userId}/followers`);
  return response.json();
}

export async function getUserFollowing(userId) {
  const response = await fetch(`${API_BASE}/users/${userId}/following`);
  return response.json();
}
