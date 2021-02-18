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

const (
	S3_BADGE_PATH = "https://testimageresize121728927389778.s3.amazonaws.com/badge.html"
	HCTL_API_URL  = "https://hcti.io/v1/image"
)

type HCTIResponse struct {
	URL string `json:"url"`
}

func resizePostedBadge() (string, error) {
	// Sanity-check that the required HCTL env vars are set
	if os.Getenv("HCTI_USER_ID") == "" || os.Getenv("HCTI_API_KEY") == "" {
		return "", errors.New("HCTI_USER_ID and HCTI_API_KEY env vars are required")
	}

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
