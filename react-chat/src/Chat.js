import { useEffect, useState } from "react";
import { fetchHistory, sendMessage } from "./api";

export default function Chat() {
  const [room, setRoom] = useState("room1");
  const [username, setUsername] = useState("");
  const [messages, setMessages] = useState([]);
  const [text, setText] = useState("");

  useEffect(() => {
    fetchHistory(room).then(setMessages);
  }, [room]);

  const handleSend = async () => {
    if (!text || !username) return;
    await sendMessage(room, username, text);
    setMessages([...messages, { username, content: text }]);
    setText("");
  };

  return (
    <div className="chat-container">
      <h2>Room: {room}</h2>

      <input
        placeholder="Username"
        value={username}
        onChange={e => setUsername(e.target.value)}
      />

      <div className="messages">
        {messages.map((m, i) => (
          <div key={i}>
            <strong>{m.username}:</strong> {m.content}
          </div>
        ))}
      </div>

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
