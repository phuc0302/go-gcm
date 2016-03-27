package gcm

// https://developers.google.com/cloud-messaging/http
// https://developers.google.com/cloud-messaging/http-server-ref#downstream-http-messages-json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const (
	// Gateway defines gcm end point.
	Gateway = "https://android.googleapis.com/gcm/send"
	// MaxBackOffDelay defines a maximum given amount of time to delay until next request.
	MaxBackOffDelay = 1024000
	// BackOffInitialDelay defines a given amount of time to delay until next request.
	BackOffInitialDelay = 1000
)

// Client describes a gcm client.
type Client struct {
	APIKey     string
	Gateway    string
	HTTPClient *http.Client
}

// CreateClient returns default gcm client.
func CreateClient(apiKey string, gateway string) *Client {
	return &Client{
		APIKey:     apiKey,
		Gateway:    gateway,
		HTTPClient: http.DefaultClient,
	}
}

// SendMessage delivers push message to Google.
func (m *Client) SendMessage(message *Message) []*Response {
	/* Condition validation */
	if message == nil {
		return nil
	}

	// Encode template message
	messages := message.encode()
	responses := make([]*Response, len(messages))

	for idx, message := range messages {
		responses[idx] = m.send(message)
	}
	return responses
}

// MARK: Struct's private functions
func (m *Client) send(message *Message) *Response {
	// Encode Json
	data, _ := json.Marshal(message)

	// Prepare request
	request, _ := http.NewRequest("POST", m.Gateway, bytes.NewBuffer(data))
	request.Header.Add("Authorization", fmt.Sprintf("key=%s", m.APIKey))
	request.Header.Add("Content-Type", "application/json")

	// Send request
	httpResponse, _ := m.HTTPClient.Do(request)
	if httpResponse == nil {
		response := &Response{
			MulticastID:  -1,
			Success:      0,
			Failure:      len(message.RegistrationIDs),
			CanonicalIDs: 0,
			Results:      make([]Result, len(message.RegistrationIDs)),
		}

		for idx, registrationID := range message.RegistrationIDs {
			response.Results[idx] = Result{Error: Timeout, RegistrationID: registrationID}
		}

		return response
	}
	defer httpResponse.Body.Close()

	// Analyze response status
	if httpResponse.StatusCode != http.StatusOK {
		response := &Response{
			MulticastID:  -1,
			Success:      0,
			Failure:      len(message.RegistrationIDs),
			CanonicalIDs: 0,
			Results:      make([]Result, len(message.RegistrationIDs)),
		}

		// Define error message
		errorMessage := ""
		if httpResponse.StatusCode == http.StatusUnauthorized {
			errorMessage = AuthenticationError
		} else {
			errorMessage = InternalServerError
		}

		// Update result response
		for idx, registrationID := range message.RegistrationIDs {
			response.Results[idx] = Result{Error: errorMessage, RegistrationID: registrationID}
		}
		return response
	}

	body, err := ioutil.ReadAll(httpResponse.Body)

	// Validate response data
	if err == nil {
		response := Response{}
		err = json.Unmarshal(body, &response)

		// Update result response
		if err == nil {
			for idx, registrationID := range message.RegistrationIDs {
				response.Results[idx].RegistrationID = registrationID
			}
			return &response
		}
	}

	// Manual create response
	response := &Response{
		MulticastID:  -1,
		Success:      0,
		Failure:      len(message.RegistrationIDs),
		CanonicalIDs: 0,
		Results:      make([]Result, len(message.RegistrationIDs)),
	}

	// Update result response
	for idx, registrationID := range message.RegistrationIDs {
		response.Results[idx] = Result{Error: InvalidJSON, RegistrationID: registrationID}
	}
	return response
}
