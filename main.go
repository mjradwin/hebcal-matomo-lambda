package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type UserLocation struct {
	ZipCode   string
	Latitude  float64
	Longitude float64
	Tzid      string
	Name      string
	State     string
	Cc        string
	CityName  string
}

type EventDetails struct {
	Category string
	Action   string
	Name     string
	Label    string
}

type TrackingMessage struct {
	Timestamp   string
	Client      string
	RequestType string
	RequestId   string
	SessionId   string
	UserId      string
	Locale      string
	IntentName  string
	Slots       map[string]string `json:"slots,omitempty"`
	Location    *UserLocation     `json:"location,omitempty"`
	Details     *EventDetails     `json:"details,omitempty"`
}

func handler(ctx context.Context, sqsEvent events.SQSEvent) error {
	for _, message := range sqsEvent.Records {
		fmt.Printf("The message %s for event source %s = %s \n", message.MessageId, message.EventSource, message.Body)
		var trackingMsg TrackingMessage
		json.Unmarshal([]byte(message.Body), &trackingMsg)
		fmt.Println(trackingMsg)
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
