package darajago

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type networkPackage struct {
	Payload  interface{}
	Endpoint string
	Method   string
	Headers  map[string]string
}

type networkResponse struct {
	Body       interface{}
	StatusCode int
}

func newRequestPackage(payload interface{}, endpoint string, method string, headers map[string]string, env Environment) *networkPackage {
	var reqUrl = baseUrlSandbox
	if env == ENVIRONMENT_PRODUCTION {
		reqUrl = baseUrlLive
	}
	reqUrl = reqUrl + endpoint

	if method == http.MethodGet {
		q := url.Values{}
		var mapPayload map[string]interface{} = struct2Map(payload)
		if len(mapPayload) > 0 {
			for key, value := range mapPayload {
				q.Add(key, value.(string))
			}
			if strings.Index(reqUrl, "?") == -1 {
				reqUrl += "?"
			} else {
				reqUrl += "&"
			}
			reqUrl += q.Encode()
		}
	}

	return &networkPackage{
		Payload:  payload,
		Endpoint: reqUrl,
		Method:   method,
		Headers:  headers,
	}
}

func (p *networkPackage) addHeader(key string, value string) {
	if p.Headers == nil {
		p.Headers = make(map[string]string)
	}
	p.Headers[key] = value
}

func newRequest(pac *networkPackage) (*http.Response, *ErrorResponse) {
	netResHolder := &networkResponse{}
	client := &http.Client{}
	var jsonDataBytes []byte
	var httpReq *http.Request

	if pac.Payload != nil {
		jsonDataBytes, _ = json.Marshal(pac.Payload)
		httpReq, _ = http.NewRequest(pac.Method, pac.Endpoint, bytes.NewBuffer(jsonDataBytes))
	} else {
		httpReq, _ = http.NewRequest(pac.Method, pac.Endpoint, nil)
	}

	for key, value := range pac.Headers {
		httpReq.Header.Add(key, value)
	}

	if pac.Method == http.MethodPost {
		// Set the content type to application/json
		httpReq.Header.Add("Content-Type", "application/json")
	}
	resp, err := client.Do(httpReq)

	if err != nil {
		return nil, &ErrorResponse{error: err}
	}

	netResHolder.StatusCode = resp.StatusCode

	//check 4xx or 5xx error
	if netResHolder.StatusCode >= 400 {
		if resp.Body != nil {
			var errorResponse *ErrorResponse
			// try to parse the error response
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, &ErrorResponse{error: err}
			}

			bodyString := string(body)

			err = json.Unmarshal(body, &errorResponse)

			if err != nil {
				// tell the user the status code and the body
				if bodyString != "" {
					return nil, &ErrorResponse{error: errors.New(resp.Status)}
				}
				return nil, &ErrorResponse{error: errors.New(resp.Status + " " + bodyString)}
			}
			if errorResponse.ErrorMessage == "" || errorResponse.ErrorCode == "" {
				errorResponse = &ErrorResponse{}
			}
			errorResponse.Raw = body
			errorResponse.error = errors.New(http.StatusText(netResHolder.StatusCode))

			return nil, errorResponse
		} else {
			return nil, &ErrorResponse{error: errors.New(resp.Status)}
		}
	}
	return resp, nil
}

func performSecurePostRequest(payload interface{}, endpoint string, d *DarajaApi) (*http.Response, *ErrorResponse) {
	var headers = make(map[string]string)

	if d.authorization.AccessToken == "" {
		_, err := d.Authorize()
		if err != nil {
			return nil, &ErrorResponse{error: err}
		}
	}
	if time.Now().After(d.nextAuthTime) {
		_, err := d.Authorize()
		if err != nil {
			return nil, &ErrorResponse{error: err}
		}
	}

	// attach the authorization header
	headers["Authorization"] = "Bearer " + d.authorization.AccessToken

	// bundle the request into a package
	netPackage := newRequestPackage(payload, endpoint, http.MethodPost, headers, d.environment)
	res, err := newRequest(netPackage)
	if err != nil {
		return nil, err
	}
	return res, nil
}
