package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	S3_REGION                        = "us-east-1"
	S3_BUCKET                        = "testimageresize121728927389778"
	BADGE_LOCAL_PATH                 = "/tmp/badge.png"
	HTML_PAGE_DEST_PATH              = "badge.html"
	EXTRACTED_BADGE_IMAGE_LOCAL_PATH = "/tmp/extracted-badge.png"
	EXTRACTED_BADGE_IMAGE_S3_PATH    = "/extracted/badge.png"
)

func Upload(s *session.Session, sourcePath, destPath string) error {

	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}

	defer file.Close()

	fileInfo, _ := file.Stat()
	var size int64 = fileInfo.Size()
	buffer := make([]byte, size)
	file.Read(buffer)

	_, err = s3.New(s).PutObject(&s3.PutObjectInput{
		Bucket:        aws.String(S3_BUCKET),
		Key:           aws.String(destPath),
		Body:          bytes.NewReader(buffer),
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(http.DetectContentType(buffer)),
	})
	return err
}

func uploadHTMLBadgeToS3() error {
	s, err := session.NewSession(&aws.Config{Region: aws.String(S3_REGION)})
	if err != nil {
		fmt.Printf("Error creating S3 session %+v\n", err)
		return err
	}

	err = Upload(s, BADGE_LOCAL_PATH, HTML_PAGE_DEST_PATH)
	if err != nil {
		fmt.Printf("Error uploading to S3 %+v\n", err)
		return err
	}
	return nil
}

func copyExtractedBadgeImageToS3(resizedImageURL string) error {
	s, err := session.NewSession(&aws.Config{Region: aws.String(S3_REGION)})
	if err != nil {
		fmt.Printf("Error creating S3 session: %+v\n", err)
		return err
	}

	response, err := http.Get(resizedImageURL)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != 200 {
		return errors.New("Received non 200 response code from HCTI API")
	}

	file, err := os.Create(EXTRACTED_BADGE_IMAGE_LOCAL_PATH)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}

	err = Upload(s, EXTRACTED_BADGE_IMAGE_LOCAL_PATH, EXTRACTED_BADGE_IMAGE_S3_PATH)
	if err != nil {
		return err
	}

	return nil
}
