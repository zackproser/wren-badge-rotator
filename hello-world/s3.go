package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func Upload(s *session.Session, filePath string) error {

	var targetPath = "badge.html"

	file, err := os.Open(filePath)
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
		Key:           aws.String(targetPath),
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

	err = Upload(s, BADGE_PATH)
	if err != nil {
		fmt.Printf("Error uploading to S3 %+v\n", err)
		return err
	}
	return nil
}
