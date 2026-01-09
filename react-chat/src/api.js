const API_BASE = "http://localhost:8080";

export async function fetchHistory(room) {
  const res = await fetch(`${API_BASE}/history?room=${room}`);
  if (!res.ok) throw new Error("Failed to fetch history");
  return res.json();
}

export async function sendMessage(room, username, content) {
  const res = await fetch(`${API_BASE}/send`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ room, username, content })
  });
  if (!res.ok) throw new Error("Failed to send message");
}
