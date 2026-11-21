package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"

	"github.com/astianmuchui/mobilesonar/internal/env"
	"github.com/astianmuchui/mobilesonar/internal/postgres"
	"github.com/astianmuchui/mobilesonar/internal/schemas"
	"github.com/astianmuchui/mobilesonar/internal/stream"
)

var Client pahomqtt.Client

func Connect() {
	opts := pahomqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("mqtt://%s:%s", env.C.MQTTBroker, env.C.MQTTPort)).
		SetClientID(fmt.Sprintf("%s-%d", env.C.MQTTClientID, uuid.New().ID())).
		SetAutoReconnect(true).
		SetCleanSession(false).
		SetOnConnectHandler(func(c pahomqtt.Client) {
			log.Println("mqtt: connected")
			subscribe(c)
		}).
		SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
			log.Printf("mqtt: connection lost: %v", err)
		})

	if env.C.MQTTUser != "" {
		opts.SetUsername(env.C.MQTTUser).SetPassword(env.C.MQTTPassword)
	}

	Client = pahomqtt.NewClient(opts)
	if tok := Client.Connect(); tok.Wait() && tok.Error() != nil {
		log.Fatalf("mqtt: failed to connect: %v", tok.Error())
	}
}

func subscribe(c pahomqtt.Client) {
	topics := map[string]byte{
		"sonar/devices/+/readings":  1,
		"sonar/devices/+/heartbeat": 1,
	}
	if tok := c.SubscribeMultiple(topics, handleMessage); tok.Wait() && tok.Error() != nil {
		log.Printf("mqtt: subscribe failed: %v", tok.Error())
		return
	}
	log.Println("mqtt: subscribed to sonar/devices/+/readings and sonar/devices/+/heartbeat")
}

func handleMessage(_ pahomqtt.Client, msg pahomqtt.Message) {
	parts := strings.Split(msg.Topic(), "/")
	if len(parts) != 4 {
		return
	}

	switch parts[3] {
	case "readings":
		handleReading(msg.Payload())
	case "heartbeat":
		handleHeartbeat(parts[2])
	}
}

func handleReading(payload []byte) {
	var req schemas.IngestReadingRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("mqtt: bad reading payload: %v", err)
		return
	}
	if err := req.Validate(); err != nil {
		log.Printf("mqtt: invalid reading [%s]: %v", req.DeviceID, err)
		return
	}
	if err := stream.PublishReading(context.Background(), &req); err != nil {
		log.Printf("mqtt: publish failed [%s]: %v", req.DeviceID, err)
	}
}

func handleHeartbeat(deviceID string) {
	if deviceID == "" {
		return
	}
	if err := postgres.UpdateDeviceHeartbeat(postgres.DB, deviceID); err != nil {
		log.Printf("mqtt: heartbeat failed [%s]: %v", deviceID, err)
	}
}
