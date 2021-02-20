package main

import (
	"errors"
	"os"
)

func sanityCheckEnvVars() error {

	if os.Getenv("GITHUB_OAUTH_TOKEN") == "" || os.Getenv("HCTI_API_KEY") == "" || os.Getenv("HCTI_USER_ID") == "" {
		return errors.New("Missing required env var")
	}
	return nil
}
