package main

import (
    "log"

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
        micro.Name("chat.subscriber"),
        micro.Broker(b),
    )
    service.Init()

    log.Println("Chat subscriber running...")

    _, err := broker.Subscribe("chat.room", func(p broker.Event) error {
        log.Println(string(p.Message().Body))
        return nil
    })
    if err != nil {
        log.Fatal(err)
    }

    if err := service.Run(); err != nil {
        log.Fatal(err)
    }
}
