import { useEffect, useState, useRef } from "react";
import { fetchHistory, sendMessage } from "./api";

export default function Chat() {
  const [room, setRoom] = useState("");
  const [username, setUsername] = useState("");
  const [messages, setMessages] = useState([]);
  const [text, setText] = useState("");
  const [roomsList, setRoomsList] = useState([]);
  const [roomInput, setRoomInput] = useState("");
  const [eventSource, setEventSource] = useState(null);

  const messagesEndRef = useRef(null); // ref for auto-scroll

  // 1️⃣ Fetch available rooms on mount
  useEffect(() => {
    fetch("http://localhost:8080/rooms")
      .then(res => res.json())
      .then(data => {
        setRoomsList(data);
        if (data.length > 0) {
          setRoom(data[0]);
          setRoomInput(data[0]);
        }
      })
      .catch(console.error);
  }, []);

  // 2️⃣ Fetch message history + SSE for live updates
  useEffect(() => {
    if (!room) return;

    fetchHistory(room)
      .then(setMessages)
      .catch(console.error);

    if (eventSource) {
      eventSource.close();
    }

    const es = new EventSource(`http://localhost:8080/stream?room=${room}`);
  es.onmessage = (e) => {
  const msg = JSON.parse(e.data);
  if (msg.content.includes("is typing")) {
    setTypingUsers(prev => {
      if (!prev.includes(msg.username)) return [...prev, msg.username];
      return prev;
    });
    setTimeout(() => {
      setTypingUsers(prev => prev.filter(u => u !== msg.username));
    }, 2000); // show typing for 2s
    return;
  }
  setMessages(prev => [...prev, msg]);
};

    es.onerror = (err) => {
      console.error("SSE error:", err);
      es.close();
    };
    setEventSource(es);

    return () => es.close();
  }, [room]);

  // 3️⃣ Auto-scroll to latest message whenever messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  // Send typing notifications
useEffect(() => {
  if (!username || !room) return;
  const timeout = setTimeout(() => {
    if (eventSource) return; // only send when typing
    fetch(`http://localhost:8080/typing`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, room, typing: true })
    }).catch(console.error);
  }, 300); // 300ms after typing starts
  return () => clearTimeout(timeout);
}, [text]);

const [typingUsers, setTypingUsers] = useState([]);

  const handleSend = async () => {
    if (!text || !username) return;
    await sendMessage(room, username, text);
    setText("");
  };

  const switchRoom = (newRoom) => {
    setRoom(newRoom);
    setRoomInput(newRoom);
    setMessages([]);
  };

  return (
    <div className="chat-container" style={{ width: "500px", margin: "auto" }}>
      <h2>Room: {room}</h2>

      {/* Username */}
      <input
        placeholder="Username"
        value={username}
        onChange={e => setUsername(e.target.value)}
        style={{ width: "100%", marginBottom: "10px" }}
      />

      {/* Room selection */}
      <div style={{ display: "flex", gap: "10px", margin: "10px 0" }}>
        {roomsList.map(r => (
          <button
            key={r}
            onClick={() => switchRoom(r)}
            style={{ fontWeight: r === room ? "bold" : "normal" }}
          >
            {r}
          </button>
        ))}
        <input
          placeholder="Custom room"
          value={roomInput}
          onChange={e => setRoomInput(e.target.value)}
        />
        <button onClick={() => switchRoom(roomInput)}>Join</button>
      </div>

      {/* Messages */}
      <div className="messages" style={{ maxHeight: "400px", overflowY: "auto", border: "1px solid #ccc", padding: "10px" }}>
        {messages.map((m, i) => (
          <div key={i}>
            <strong>{m.username}</strong> ({new Date(m.created_at).toLocaleTimeString()}): {m.content}
          </div>
        ))}
        <div ref={messagesEndRef} /> {/* Scroll target */}
      </div>
      {typingUsers.length > 0 && (
  <div style={{ fontStyle: "italic", color: "#888" }}>
    {typingUsers.join(", ")} {typingUsers.length === 1 ? "is" : "are"} typing...
  </div>
)}


      {/* Message input */}
      <input
        placeholder="Type a message..."
        value={text}
        onChange={e => setText(e.target.value)}
        onKeyDown={e => e.key === "Enter" && handleSend()}
        style={{ width: "80%", marginTop: "10px" }}
      />
      <button onClick={handleSend} style={{ width: "18%", marginLeft: "2%", marginTop: "10px" }}>Send</button>
    </div>
  );
}
