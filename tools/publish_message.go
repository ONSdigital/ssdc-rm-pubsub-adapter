package main

import (
	"cloud.google.com/go/pubsub"
	"context"
	"fmt"
)

func main() {
	jsonMessageStr := `{"timeCreated": "2008-08-24T00:00:00Z",
	"metadata": {"tx_id": "abc123xxx", "questionnaire_id": "01213213213"}}`

	publishMessage("project", "eq-submission-topic", jsonMessageStr)

}

func publishMessage(projectID, topicID, msg string) error {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		fmt.Printf("pubsub.NewClient: %v\n", err)
		return fmt.Errorf("pubsub.NewClient: %v", err)
	}

	t := client.Topic(topicID)

	result := t.Publish(ctx, &pubsub.Message{
		Data: []byte(msg),
	})
	// Block until the result is returned and a server-generated
	// ID is returned for the published message.
	id, err := result.Get(ctx)
	if err != nil {
		fmt.Printf("get: %v\n", err)
		return fmt.Errorf("get: %v", err)
	}
	fmt.Printf("Published a message; msg ID: %v\n", id)
	return nil
}
