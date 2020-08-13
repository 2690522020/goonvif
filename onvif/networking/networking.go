package networking

import (
	"bytes"
	auth "goonvif/digest"
	"net/http"
	"time"
)

// SendSoap send soap message
func SendSoap(transport *auth.DigestTransport, endpoint string, message []byte) (*http.Response, error) {
	return SendSoapWithTimeout(transport, endpoint, message, time.Second*6)
}

// SendSoapWithTimeout send soap message with timeOut
func SendSoapWithTimeout(transport *auth.DigestTransport, endpoint string, message []byte, timeout time.Duration) (*http.Response, error) {
	var httpClient *http.Client
	if transport != nil {
		httpClient = &http.Client{
			Transport: transport,
			Timeout:   timeout,
		}
	} else {
		httpClient = &http.Client{
			Timeout: timeout,
		}
	}
	return httpClient.Post(endpoint, "application/soap+xml; charset=utf-8", bytes.NewReader(message))

}
