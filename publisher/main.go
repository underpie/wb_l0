package main

import (
	"fmt"
	"os"

	"github.com/nats-io/stan.go"
)

func main() {
	// Настройки подключения
	clusterID := os.Getenv("NATS_CLUSTER")
	if clusterID == "" {
		clusterID = "test-cluster"
	}

	clientID := "publisher-client"
	subject := os.Getenv("NATS_SUBJECT")
	if subject == "" {
		subject = "orders"
	}

	// Подключение к nats
	sc, err := stan.Connect(clusterID, clientID, stan.NatsURL(stan.DefaultNatsURL))
	if err != nil {
		panic(err)
	}
	defer sc.Close()

	// Читаем model.json — шаблон заказа
	data, err := os.ReadFile("model.json")
	if err != nil {
		panic(err)
	}

	// Публикуем сообщение в канал
	err = sc.Publish(subject, data)
	if err != nil {
		panic(err)
	}

	fmt.Println("Заказ успешно опубликован в канал:", subject)
}
