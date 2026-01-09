const API_BASE = "http://localhost:8080";

export async function fetchHistory(room) {
  const res = await fetch(`${API_BASE}/history?room=${room}`);
  return res.json();
}

export async function sendMessage(room, username, message) {
  await fetch(`${API_BASE}/send`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ room, username, message })
  });
}
