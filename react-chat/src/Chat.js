import { useEffect, useState } from "react";
import { fetchHistory, sendMessage } from "./api";

export default function Chat() {
  const [room, setRoom] = useState("room1");
  const [username, setUsername] = useState("");
  const [messages, setMessages] = useState([]);
  const [text, setText] = useState("");
  const [roomInput, setRoomInput] = useState("room1");

  const roomsList = ["room1", "room2", "room3"]; // example rooms

  // Fetch messages every 2 seconds
  useEffect(() => {
    const fetchMessages = async () => {
      try {
        const msgs = await fetchHistory(room);
        setMessages(msgs);
      } catch (err) {
        console.error(err);
      }
    };

    fetchMessages(); // initial fetch
    const interval = setInterval(fetchMessages, 2000); // poll every 2s
    return () => clearInterval(interval); // cleanup
  }, [room]);

  const handleSend = async () => {
    if (!text || !username) return;
    await sendMessage(room, username, text);
    setText("");
  };

  const switchRoom = (newRoom) => {
    setRoom(newRoom);
    setRoomInput(newRoom);
  };

  return (
    <div className="chat-container">
      <h2>Room: {room}</h2>

      {/* Username input */}
      <input
        placeholder="Username"
        value={username}
        onChange={e => setUsername(e.target.value)}
      />

      {/* Room selection */}
      <div style={{ display: "flex", gap: "10px", margin: "10px 0" }}>
        {roomsList.map(r => (
          <button
            key={r}
            onClick={() => switchRoom(r)}
            style={{
              fontWeight: r === room ? "bold" : "normal"
            }}
          >
            {r}
          </button>
        ))}
        {/* Optionally allow custom room input */}
        <input
          placeholder="Custom room"
          value={roomInput}
          onChange={e => setRoomInput(e.target.value)}
        />
        <button onClick={() => switchRoom(roomInput)}>Join</button>
      </div>

  <div className="messages">
  {messages.map((m, i) => (
    <div key={i}>
      <strong>{m.username}</strong> ({new Date(m.created_at).toLocaleTimeString()}): {m.content}
    </div>
  ))}
</div>


      {/* Send message input */}
      <input
        placeholder="Type a message..."
        value={text}
        onChange={e => setText(e.target.value)}
        onKeyDown={e => e.key === "Enter" && handleSend()}
      />
      <button onClick={handleSend}>Send</button>
    </div>
  );
}
