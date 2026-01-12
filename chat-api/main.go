package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"go-micro.dev/v5"
	"go-micro.dev/v5/broker"
	bnats "go-micro.dev/v5/broker/nats"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

/////////////////////////////////////////////////
// MODELS
/////////////////////////////////////////////////

type User struct {
	ID       uint   `json:"id"`
	Username string `json:"username" gorm:"unique"`
	Password string `json:"password"`
}

type Message struct {
	ID        uint      `json:"id"`
	Room      string    `json:"room"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

/////////////////////////////////////////////////
// JWT
/////////////////////////////////////////////////

var jwtKey = []byte("supersecretkey")

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func generateToken(username string) (string, error) {
	claims := Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func verifyToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}
	return claims, nil
}

/////////////////////////////////////////////////
// SSE CLIENTS
/////////////////////////////////////////////////

type sseClient struct {
	room string
	ch   chan any
}

var sseClients = struct {
	sync.RWMutex
	clients map[*sseClient]bool
}{
	clients: make(map[*sseClient]bool),
}

/////////////////////////////////////////////////
// HELPERS
/////////////////////////////////////////////////

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			return
		}

		tokenStr := r.Header.Get("Authorization")
		if tokenStr == "" {
			http.Error(w, "Missing token", 401)
			return
		}

		claims, err := verifyToken(tokenStr)
		if err != nil {
			http.Error(w, "Invalid token", 401)
			return
		}

		ctx := context.WithValue(r.Context(), "username", claims.Username)
		next(w, r.WithContext(ctx))
	}
}

/////////////////////////////////////////////////
// MAIN
/////////////////////////////////////////////////

func main() {
	// PostgreSQL
	dsn := "host=localhost user=postgres password=secret dbname=chat port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	db.AutoMigrate(&User{}, &Message{})

	// NATS
	b := bnats.NewNatsBroker()
	if err := b.Connect(); err != nil {
		log.Fatal(err)
	}

	service := micro.NewService(
		micro.Name("chat.api"),
		micro.Broker(b),
	)
	service.Init()

	// ROUTES
	http.HandleFunc("/register", registerHandler(db))
	http.HandleFunc("/login", loginHandler(db))

	http.HandleFunc("/send", authMiddleware(sendHandler(db, b)))
	http.HandleFunc("/history", authMiddleware(historyHandler(db)))
	http.HandleFunc("/stream", authMiddleware(streamHandler(b)))
	http.HandleFunc("/rooms", authMiddleware(roomsHandler(db)))
	http.HandleFunc("/typing", authMiddleware(typingHandler(b)))

	log.Println("🚀 Chat API running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

/////////////////////////////////////////////////
// AUTH
/////////////////////////////////////////////////

func registerHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		var u User
		json.NewDecoder(r.Body).Decode(&u)

		hash, _ := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		u.Password = string(hash)

		if err := db.Create(&u).Error; err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		token, _ := generateToken(u.Username)
		json.NewEncoder(w).Encode(map[string]string{"token": token})
	}
}

func loginHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		var req User
		json.NewDecoder(r.Body).Decode(&req)

		var u User
		db.Where("username = ?", req.Username).First(&u)

		if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)) != nil {
			http.Error(w, "Invalid credentials", 401)
			return
		}

		token, _ := generateToken(u.Username)
		json.NewEncoder(w).Encode(map[string]string{"token": token})
	}
}

/////////////////////////////////////////////////
// CHAT
/////////////////////////////////////////////////

func sendHandler(db *gorm.DB, b broker.Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.Context().Value("username").(string)

		var msg Message
		json.NewDecoder(r.Body).Decode(&msg)
		msg.Username = username
		msg.CreatedAt = time.Now()

		db.Create(&msg)

		data, _ := json.Marshal(msg)
		b.Publish(msg.Room, &broker.Message{Body: data})

		w.WriteHeader(200)
	}
}

func historyHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		room := r.URL.Query().Get("room")
		var msgs []Message

		db.Where("room = ?", room).
			Order("created_at asc").
			Find(&msgs)

		json.NewEncoder(w).Encode(msgs)
	}
}

func streamHandler(b broker.Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		room := r.URL.Query().Get("room")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		flusher := w.(http.Flusher)

		client := &sseClient{room: room, ch: make(chan any)}
		sseClients.Lock()
		sseClients.clients[client] = true
		sseClients.Unlock()

		defer func() {
			sseClients.Lock()
			delete(sseClients.clients, client)
			sseClients.Unlock()
		}()

		b.Subscribe(room, func(e broker.Event) error {
			var msg Message
			json.Unmarshal(e.Message().Body, &msg)

			sseClients.RLock()
			for c := range sseClients.clients {
				if c.room == room {
					c.ch <- msg
				}
			}
			sseClients.RUnlock()
			return nil
		})

		for {
			select {
			case data := <-client.ch:
				json.NewEncoder(w).Encode(data)
				fmt.Fprint(w, "\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}

/////////////////////////////////////////////////
// TYPING INDICATOR
/////////////////////////////////////////////////

func typingHandler(b broker.Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.Context().Value("username").(string)

		var body struct {
			Room   string `json:"room"`
			Typing bool   `json:"typing"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		payload, _ := json.Marshal(map[string]any{
			"type":     "typing",
			"username": username,
			"typing":   body.Typing,
		})

		b.Publish(body.Room, &broker.Message{Body: payload})
	}
}

/////////////////////////////////////////////////
// ROOMS
/////////////////////////////////////////////////

func roomsHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var rooms []string
		db.Model(&Message{}).Distinct().Pluck("room", &rooms)
		json.NewEncoder(w).Encode(rooms)
	}
}
