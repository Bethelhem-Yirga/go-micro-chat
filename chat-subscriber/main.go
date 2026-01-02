package main

import (
    "bufio"
    "fmt"
    "log"
    "os"
    "strings"

    "go-micro.dev/v5"
    bnats "go-micro.dev/v5/broker/nats"
    "go-micro.dev/v5/broker"

    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

// Message model for PostgreSQL
type Message struct {
    ID       uint   `gorm:"primaryKey"`
    Room     string `gorm:"index"`
    Username string
    Content  string
    CreatedAt gorm.DeletedAt
}

func main() {
    // Connect to PostgreSQL
	dsn := "host=localhost user=postgres password=secret dbname=chat port=5432 sslmode=disable"

    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatal("failed to connect to database:", err)
    }

    // Auto-migrate table
    if err := db.AutoMigrate(&Message{}); err != nil {
        log.Fatal("failed to migrate database:", err)
    }

    // Connect to NATS
    b := bnats.NewNatsBroker()
    if err := b.Connect(); err != nil {
        log.Fatal("Broker connect error:", err)
    }

    service := micro.NewService(
        micro.Name("chat.subscriber"),
        micro.Broker(b),
    )
    service.Init()

    reader := bufio.NewReader(os.Stdin)
    fmt.Print("Enter your username: ")
    username, _ := reader.ReadString('\n')
    username = strings.TrimSpace(username)

    fmt.Print("Enter chat room to join: ")
    room, _ := reader.ReadString('\n')
    room = strings.TrimSpace(room)

    // Print previous messages from PostgreSQL
    printMessageHistory(db, room)

    log.Printf("Joined room '%s'. Waiting for messages...\n", room)

    // Subscribe to messages in this room
    _, err = broker.Subscribe(room, func(p broker.Event) error {
        msgText := string(p.Message().Body)

        // Save to PostgreSQL
        parts := strings.SplitN(msgText, ": ", 2)
        sender := "unknown"
        content := msgText
        if len(parts) == 2 {
            sender = parts[0]
            content = parts[1]
        }

        db.Create(&Message{
            Room:     room,
            Username: sender,
            Content:  content,
        })

        fmt.Println(msgText)
        return nil
    })
    if err != nil {
        log.Fatal(err)
    }

    if err := service.Run(); err != nil {
        log.Fatal(err)
    }
}

// Fetch previous messages from PostgreSQL
func printMessageHistory(db *gorm.DB, room string) {
    var msgs []Message
    result := db.Where("room = ?", room).Order("id asc").Find(&msgs)
    if result.Error != nil {
        log.Println("Failed to load message history:", result.Error)
        return
    }

    if len(msgs) > 0 {
        fmt.Println("Previous messages in", room)
        for _, msg := range msgs {
            fmt.Printf("%s: %s\n", msg.Username, msg.Content)
        }
        fmt.Println("---- End of history ----")
    }
}
