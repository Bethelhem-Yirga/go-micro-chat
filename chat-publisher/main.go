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
)

func main() {
    b := bnats.NewNatsBroker()
    if err := b.Connect(); err != nil {
        log.Fatal("Broker connect error:", err)
    }

    service := micro.NewService(
        micro.Name("chat.publisher"),
        micro.Broker(b),
    )
    service.Init()

    reader := bufio.NewReader(os.Stdin)

    // Enter username
    fmt.Print("Enter your username: ")
    username, _ := reader.ReadString('\n')
    username = strings.TrimSpace(username)

    // Enter chat room
    fmt.Print("Enter chat room: ")
    room, _ := reader.ReadString('\n')
    room = strings.TrimSpace(room)

    fmt.Println("Start typing messages. Press Ctrl+C to quit.")
    for {
        fmt.Print("> ")
        msgText, _ := reader.ReadString('\n')
        msgText = strings.TrimSpace(msgText)
        if msgText == "" {
            continue
        }

        msg := &broker.Message{
            Body: []byte(fmt.Sprintf("%s: %s", username, msgText)),
        }

        if err := broker.Publish(room, msg); err != nil {
            log.Println("Publish error:", err)
        }
    }
}
