// GlbToHoneycomb is a service receiving Google Cloud LoadBalancer request events from
// Google Cloud Logging using pub/sub, and emitting they to Honeycomb as spans
package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	libhoney "github.com/honeycombio/libhoney-go"
)

func main() {
	writeKey := os.Getenv("HONEYCOMB_APIKEY")
	dataset := os.Getenv("HONEYCOMB_DATASET")

	if writeKey == "" {
		panic("Environment variable HONEYCOMB_APIKEY is missing")
	}

	if dataset == "" {
		panic("Environment variable HONEYCOMB_DATASET is missing")
	}

	libhoney.Init(libhoney.Config{
		WriteKey: writeKey,
		Dataset:  dataset,
	})
	defer libhoney.Close() // Flush any pending calls to Honeycomb

	http.HandleFunc("/pubsub_message", HandlePubSubMessage)

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	// Start HTTP server.
	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

type PubSubMessage struct {
	Message struct {
		Data string `json:"data,omitempty"`
	} `json:"message"`
}

// HelloPubSub receives and processes a Pub/Sub push message.
func HandlePubSubMessage(w http.ResponseWriter, r *http.Request) {
	var m PubSubMessage
	body, err := io.ReadAll(r.Body)

	if err != nil {
		log.Printf("ioutil.ReadAll: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Decode the JSON payload to get a pub/sub message struct
	if err := json.Unmarshal(body, &m); err != nil {
		log.Printf("json.Unmarshal: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// The data field is encoded as Base64
	decodedData, err := base64.RawStdEncoding.DecodeString(m.Message.Data)

	if err != nil {
		log.Printf("base64.DecodeString: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// And this base64 is a JSON string with our log event

	var event RequestEvent

	if err := json.Unmarshal(decodedData, &event); err != nil {
		log.Printf("event.json.Unmarshal: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := event.SendToHoneycomb(); err != nil {
		log.Printf("sendToHoneycomb: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
}
