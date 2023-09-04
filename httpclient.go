package httpmodule

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	neturl "net/url"
	"strconv"
	"strings"
	"time"
)

type HttpClient struct {
	DefaultHeaders map[string]string
}

type HttpResponse struct {
	Protocol   string
	StatusCode int
	Status     string
	Headers    map[string]string
	Body       string
}

func New() *HttpClient {
	return &HttpClient{
		DefaultHeaders: make(map[string]string),
	}
}

func (client *HttpClient) constructRequest(method, url, body string, headers map[string]string) (string, error) {
	// Extract the path and host from the URL
	parsedURL, err := neturl.Parse(url)
	if err != nil {
		return "", err
	}
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}
	host := parsedURL.Host

	// Default headers
	defaultHeaders := map[string]string{
		"Host":            host,
		"User-Agent":      "CustomHttpClient/1.0",
		"Accept":          "*/*",
		"Accept-Language": "en-US,en;q=0.8",
		"Accept-Encoding": "gzip, deflate, br",
		"Connection":      "keep-alive",
	}

	// Merge default headers with client's default headers
	for k, v := range client.DefaultHeaders {
		defaultHeaders[k] = v
	}

	// Override with user-provided headers
	for k, v := range headers {
		defaultHeaders[k] = v
	}

	if method == "" || url == "" {
		return "", fmt.Errorf("method and url cannot be empty")
	}

	// Construct the request
	requestBuilder := &strings.Builder{}
	requestBuilder.WriteString(fmt.Sprintf("%s %s HTTP/1.1\r\n", method, path))

	// Add headers
	for k, v := range defaultHeaders {
		requestBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	// Add Content-Length header
	requestBuilder.WriteString(fmt.Sprintf("Content-Length: %d\r\n", len(body)))

	// End of headers
	requestBuilder.WriteString("\r\n")

	// Append body if present
	if body != "" {
		requestBuilder.WriteString(body)
	}

	return requestBuilder.String(), nil
}

func (client *HttpClient) sendRequest(request string, scheme string, host string) (*HttpResponse, error) {
	var conn net.Conn
	var err error

	// Create a dialer with custom options (e.g., timeout)
	dialer := &net.Dialer{
		Timeout:   30 * time.Second, // Example timeout
		KeepAlive: 30 * time.Second, // Example keep-alive
	}

	// Determine if the request is HTTPS based on the host
	if strings.HasPrefix(scheme, "https://") {
		// Establish a TLS connection for HTTPS
		conf := &tls.Config{
			InsecureSkipVerify: false, // This skips certificate verification; for production, you'd want to verify certificates
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", strings.TrimPrefix(host, "https://")+":443", conf)
	} else {
		// Establish a regular TCP connection for HTTP
		conn, err = dialer.Dial("tcp", strings.TrimPrefix(host, "http://")+":80")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to establish connection: %v", err)
	}
	defer conn.Close()

	// Send the request
	_, err = conn.Write([]byte(request))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	return parseHTTPResponse(conn)
}

func parseHTTPResponse(conn net.Conn) (*HttpResponse, error) {
	reader := bufio.NewReader(conn)

	// Read the status line
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, errors.New("failed to read status line")
	}
	// Ensure the status line ends with \r\n
	if !strings.HasSuffix(statusLine, "\r\n") {
		return nil, errors.New("malformed status line: missing CR LF at the end")
	}
	// Split the status line into protocol, status code, and status
	parts := strings.SplitN(strings.TrimSpace(statusLine), " ", 3)
	if len(parts) < 3 {
		return nil, errors.New("malformed status line")
	}
	// Parse the protocol version
	protocol := parts[0]
	// Parse the status code
	statusCode, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, errors.New("invalid status code")
	}
	// Parse the status
	status := parts[2]

	// Parse headers
	headers := make(map[string]string)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, errors.New("failed to read header line")
		}
		// Check for the end of the headers section
		if line == "\r\n" || err == io.EOF {
			break
		}
		// Ensure the header line ends with \r\n
		if !strings.HasSuffix(line, "\r\n") {
			return nil, errors.New("malformed header line: missing CR LF at the end")
		}

		// Split the header line into key and value
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			return nil, errors.New("malformed header line: " + line)
		}

		// Add the header to the map
		headerKey := strings.TrimSpace(parts[0])
		// Header keys are case-insensitive, so we lowercase them
		headerValue := strings.TrimSpace(parts[1])
		headers[headerKey] = headerValue
	}

	// Read body
	body, err := parseBody(reader, headers)
	if err != nil {
		return nil, err
	}

	// Return the response
	return &HttpResponse{
		Protocol:   protocol,
		StatusCode: statusCode,
		Status:     status,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func parseBody(reader *bufio.Reader, headers map[string]string) (string, error) {
	// Check for "Transfer-Encoding: chunked"
	if headers["Transfer-Encoding"] == "chunked" {
		var body bytes.Buffer
		for {
			// Read chunk size
			sizeStr, err := reader.ReadString('\n')
			if err != nil {
				return "", err
			}

			// Convert chunk size from hex to int64
			size, err := strconv.ParseInt(strings.TrimSpace(sizeStr), 16, 64)
			if err != nil {
				return "", errors.New("invalid chunk size")
			}

			// Check for last chunk
			if size == 0 {
				break
			}

			// Read chunk data
			chunk := make([]byte, size)
			_, err = io.ReadFull(reader, chunk)
			if err != nil {
				return "", err
			}

			// Append chunk to body
			body.Write(chunk)
			// Read trailing CRLF after chunk
			reader.ReadString('\n')
		}
		// Read trailing headers after last chunk
		for {
			line, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				return "", err
			}
			if line == "\r\n" || err == io.EOF {
				break
			}
		}
		return body.String(), nil
	}

	// Check for "Content-Length" header
	if contentLength, ok := headers["Content-Length"]; ok {
		length, err := strconv.Atoi(contentLength)
		if err != nil {
			return "", errors.New("invalid Content-Length header")
		}
		bodyBytes := make([]byte, length)
		_, err = io.ReadFull(reader, bodyBytes)
		if err != nil {
			return "", err
		}
		return string(bodyBytes), nil
	}

	// If neither header is present, read until EOF (not recommended for real-world use)
	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}

func (client *HttpClient) Get(url string, headers map[string]string) (*HttpResponse, error) {
	request, err := client.constructRequest("GET", url, "", headers)
	if err != nil {
		return nil, err
	}

	// Extract the path and host from the URL
	hostParts := strings.Split(url, "//")
	if len(hostParts) < 2 {
		return nil, fmt.Errorf("invalid URL format: %s", url)
	}

	return client.sendRequest(request, hostParts[0], hostParts[1])

}

func (client *HttpClient) Post(url, body string, headers map[string]string) (*HttpResponse, error) {
	// Construct the request
	request, err := client.constructRequest("POST", url, body, headers)
	if err != nil {
		return nil, err
	}

	// Extract the path and host from the URL
	hostParts := strings.Split(url, "//")
	if len(hostParts) < 2 {
		return nil, fmt.Errorf("invalid URL format: %s", url)
	}

	return client.sendRequest(request, hostParts[0], hostParts[1])

}

func (client *HttpClient) Options(url string, headers map[string]string) (*HttpResponse, error) {
	// Construct the request
	request, err := client.constructRequest("OPTIONS", url, "", headers)
	if err != nil {
		return nil, err
	}

	// Extract the path and host from the URL
	hostParts := strings.Split(url, "//")
	if len(hostParts) < 2 {
		return nil, fmt.Errorf("invalid URL format: %s", url)
	}

	return client.sendRequest(request, hostParts[0], hostParts[1])
}
