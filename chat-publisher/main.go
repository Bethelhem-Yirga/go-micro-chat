package main

import (
    "bufio"
    "fmt"
    "log"
    "os"

    "go-micro.dev/v5"
    bnats "go-micro.dev/v5/broker/nats"
    "go-micro.dev/v5/broker"
)

func main() {
    b := bnats.NewNatsBroker()
    
    // Connect to NATS
    if err := b.Connect(); err != nil {
        log.Fatal("Broker connect error:", err)
    }

    service := micro.NewService(
        micro.Name("chat.publisher"),
        micro.Broker(b),
    )
    service.Init()

    reader := bufio.NewReader(os.Stdin)
    fmt.Print("Enter your username: ")
    username, _ := reader.ReadString('\n')
    username = username[:len(username)-1]

    fmt.Println("Start typing messages. Press Ctrl+C to quit.")

    for {
        fmt.Print("> ")
        msgText, _ := reader.ReadString('\n')
        msgText = msgText[:len(msgText)-1]

        msg := &broker.Message{
            Body: []byte(fmt.Sprintf("%s: %s", username, msgText)),
        }

        if err := broker.Publish("chat.room", msg); err != nil {
            log.Println("Publish error:", err)
        }
    }
}
