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

// Upload takes a local sourch path and a remote S3 path and uploads the file at the sourcePath to the destPath in S3
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

// uploadHTMLBagdeToS3 takes the modified HTML page and hosts it live via the public S3 bucket so that the HCTI API
// will be able to fetch it and extract the badge image from it
func uploadHTMLBadgeToS3() error {
	s, err := session.NewSession(&aws.Config{Region: aws.String(S3_REGION)})
	if err != nil {
		fmt.Printf("Error creating S3 session %+v\n", err)
		return err
	}

	err = Upload(s, BADGE_LOCAL_PATH, HTML_PAGE_DEST_S3_PATH)
	if err != nil {
		fmt.Printf("Error uploading to S3 %+v\n", err)
		return err
	}
	return nil
}

// copyExtractedBadgeImageToS3 takes in the URL that was returned by the HCTI API, where the extracted, updated badge is hosted,
// and reads then write it to a local file first. Next, it uploads the local file to a special S3 prefix /extracted for
// safe keeping and sanity checking - even though this S3 hosted badge is not used itself - you could also link to it
// directly and then just keep running this or a similar function to update it in place if you did not want to go through
// the hassle of programmatically handling the git / Github operations
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
