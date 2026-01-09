package main

import (
    "encoding/json"
    "log"
    "net/http"

    "go-micro.dev/v5"
    "go-micro.dev/v5/broker"
    bnats "go-micro.dev/v5/broker/nats"

    "gorm.io/driver/postgres"
    "gorm.io/gorm"
	"time"
	"sync"
	"fmt"
)

// Message model
type Message struct {
    ID       uint   `json:"id"`
    Room     string `json:"room"`
    Username string `json:"username"`
    Content  string `json:"content"`
    CreatedAt time.Time `json:"created_at"` // new timestamp
}
// SSE client channel
type sseClient struct {
    room string
    ch   chan Message
}

var sseClients = struct {
    sync.RWMutex
    clients map[*sseClient]bool
}{clients: make(map[*sseClient]bool)}


// ✅ CORS helper MUST be outside main
func enableCORS(w http.ResponseWriter) {
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
    w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

func main() {
    // PostgreSQL
    dsn := "host=localhost user=postgres password=secret dbname=chat port=5432 sslmode=disable"
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatal(err)
    }

    if err := db.AutoMigrate(&Message{}); err != nil {
        log.Fatal(err)
    }

    // NATS Broker
    b := bnats.NewNatsBroker()
    if err := b.Connect(); err != nil {
        log.Fatal(err)
    }

    service := micro.NewService(
        micro.Name("chat.api"),
        micro.Broker(b),
    )
    service.Init()

    // Routes
   http.HandleFunc("/send", sendHandler(b, db))
   http.HandleFunc("/history", historyHandler(db))
   http.HandleFunc("/stream", streamHandler(b))
   http.HandleFunc("/rooms", roomsHandler(db))


    log.Println("Chat API running on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func sendHandler(b broker.Broker, db *gorm.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        enableCORS(w)

        if r.Method == http.MethodOptions {
            return
        }

        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        var body Message
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        // Save message to database
        if err := db.Create(&body).Error; err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        // Publish to NATS (optional for other subscribers)
        msg := body.Username + ": " + body.Content
        if err := broker.Publish(body.Room, &broker.Message{
            Body: []byte(msg),
        }); err != nil {
            log.Println("Publish error:", err)
        }

        w.WriteHeader(http.StatusOK)
    }
}

// GET /history?room=room1
func historyHandler(db *gorm.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        enableCORS(w)

        if r.Method == http.MethodOptions {
            return
        }

        room := r.URL.Query().Get("room")
        var messages []Message

       db.Where("room = ?", room).
    Order("created_at asc"). // ensures chronological order
    Find(&messages)


        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(messages)
    }
}

func streamHandler(b broker.Broker) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        enableCORS(w)
        if r.Method == http.MethodOptions {
            return
        }

        room := r.URL.Query().Get("room")
        if room == "" {
            http.Error(w, "room required", 400)
            return
        }

        // Set SSE headers
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")

        flusher, ok := w.(http.Flusher)
        if !ok {
            http.Error(w, "Streaming not supported", http.StatusInternalServerError)
            return
        }

        // New client
        client := &sseClient{
            room: room,
            ch:   make(chan Message),
        }

        // Register client
        sseClients.Lock()
        sseClients.clients[client] = true
        sseClients.Unlock()

        // Remove client on exit
        defer func() {
            sseClients.Lock()
            delete(sseClients.clients, client)
            sseClients.Unlock()
        }()

        // Listen to NATS messages for this room
        _, err := broker.Subscribe(room, func(p broker.Event) error {
            var msg Message
            msg.Content = string(p.Message().Body)
            msg.Room = room
            msg.CreatedAt = time.Now() // capture timestamp
            // broadcast to all SSE clients
            sseClients.RLock()
            for c := range sseClients.clients {
                if c.room == room {
                    select {
                    case c.ch <- msg:
                    default:
                        // skip if blocked
                    }
                }
            }
            sseClients.RUnlock()
            return nil
        })
        if err != nil {
            log.Println("Subscribe error:", err)
        }
		// Inside streamHandler, after subscribing to room messages:
_, err = broker.Subscribe(room+".typing", func(p broker.Event) error {
    // Broadcast typing event
    var typingMsg struct {
        Username string `json:"username"`
        Typing   bool   `json:"typing"`
    }
    json.Unmarshal(p.Message().Body, &typingMsg)

    sseClients.RLock()
    for c := range sseClients.clients {
        if c.room == room {
            select {
            case c.ch <- Message{
                Content: fmt.Sprintf("%s is typing", typingMsg.Username),
                Username: typingMsg.Username,
                Room: room,
                CreatedAt: time.Now(),
            }:
            default:
            }
        }
    }
    sseClients.RUnlock()
    return nil
})
	

        // Send messages to client
        for {
            select {
            case msg := <-client.ch:
                data, _ := json.Marshal(msg)
                fmt.Fprintf(w, "data: %s\n\n", data)
                flusher.Flush()
            case <-r.Context().Done():
                return
            }
        }
    }
}

func roomsHandler(db *gorm.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        enableCORS(w)
        if r.Method == http.MethodOptions {
            return
        }

        var rooms []string
        // SELECT DISTINCT room FROM messages
        db.Model(&Message{}).Distinct().Pluck("room", &rooms)

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(rooms)
    }
}
