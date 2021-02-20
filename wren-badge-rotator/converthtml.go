package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

type HCTIResponse struct {
	URL string `json:"url"`
}

// resizePostedBadge makes an API call to the HCTI API, passing it the URL of the S3-hosted badge.html file
// HCTI will return a URL at which it is hosting the extracted badge image
func resizePostedBadge() (string, error) {
	// Sanity-check that the required HCTL env vars are set
	if os.Getenv("HCTI_USER_ID") == "" || os.Getenv("HCTI_API_KEY") == "" {
		return "", errors.New("HCTI_USER_ID and HCTI_API_KEY env vars are required")
	}

	// Set parameters to pass to the HCTI API
	data := map[string]string{
		"url":             S3_BADGE_PATH,
		"viewport_width":  "300",
		"viewport_height": "117",
		"selector":        ".container",
	}

	reqBody, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", HCTL_API_URL, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(os.Getenv("HCTI_USER_ID"), os.Getenv("HCTI_API_KEY"))
	client := &http.Client{Timeout: time.Second * 15}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	hr := &HCTIResponse{}

	unmarshalErr := json.Unmarshal(body, hr)

	if unmarshalErr != nil {
		return "", err
	}

	fmt.Printf("Got HCTI API Response: %+v\n", hr)

	return hr.URL, nil
}
