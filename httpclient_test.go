package httpmodule

import (
	"fmt"
	"testing"
)

// Create a global instance of our HTTP client.
var hc *HttpClient

var bold = "\033[1m"
var reset = "\033[0m"

// Create an init function that will be called before any tests are run and creates the HTTP client.
func init() {
	hc = New()
}

// TestNew tests the New function.
func TestNew(t *testing.T) {
	if hc == nil {
		t.Error("Expected non-nil HttpClient instance.")
	}
}

// TestConstructRequest tests the constructRequest function.
func TestConstructRequest(t *testing.T) {
	request, err := hc.constructRequest("GET", "/", "", nil)
	if err != nil {
		t.Error("Expected nil error.")
	}
	if request != "GET / HTTP/1.1\r\nContent-Length: 0\r\n\r\n" {
		t.Error("Expected different request string.")
	}
}

// TestSendRequest tests the sendRequest function.
func TestSendRequest(t *testing.T) {
	response, err := hc.sendRequest("GET / HTTP/1.1\r\nContent-Length: 0\r\n\r\n", "https://", "google.com")
	if err != nil {
		t.Error("Expected nil error.")
	}
	if response == nil {
		t.Error("Expected non-nil HttpResponse instance.")
	}
}

// TestGet tests the Get function.
func TestGet(t *testing.T) {
	response, err := hc.Get("https://www.google.com", nil)
	if err != nil {
		t.Error("Expected nil error.", err)
	}
	if response == nil {
		t.Error("Expected non-nil HttpResponse instance.")
	}

	// Print the full response to the console.
	// Print the status code in bold and string format.

	fmt.Printf("\n%sProtocol%s: %s\n", bold, reset, response.Protocol)
	fmt.Printf("\n%sStatus Code%s: %s\n", bold, reset, fmt.Sprint(response.StatusCode))
	fmt.Printf("\n%sStatus%s: %s\n", bold, reset, response.Status)
	fmt.Printf("\n%sResponse Headers%s:\n", bold, reset)
	for key, value := range response.Headers {
		fmt.Printf("%s: %s\n", key, value)
	}
	fmt.Printf("\n%sResponse Body%s: %s\n", bold, reset, response.Body)

}

// TestPost tests the Post function.
func TestPost(t *testing.T) {
	response, err := hc.Post("google.com", "", nil)
	if err != nil {
		t.Error("Expected nil error.")
	}
	if response == nil {
		t.Error("Expected non-nil HttpResponse instance.")
	}
}
