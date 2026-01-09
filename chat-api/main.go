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
)

// Message model
type Message struct {
    ID       uint   `json:"id"`
    Room     string `json:"room"`
    Username string `json:"username"`
    Content  string `json:"content"`
    CreatedAt time.Time `json:"created_at"` // new timestamp
}

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
