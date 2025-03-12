package s3pkg

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	appconfig "github.com/hiiamtrong/imap-bot-go/internal/config"
)

type S3Service struct {
	client *s3.Client
	bucket string
	region string
}

func NewS3Service(cfg *appconfig.Config) (*S3Service, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(cfg.AWS.Region),
		awsconfig.WithSharedConfigProfile(cfg.AWS.Profile),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %v", err)
	}

	client := s3.NewFromConfig(awsCfg)

	return &S3Service{
		client: client,
		bucket: cfg.AWS.S3Bucket,
		region: cfg.AWS.Region,
	}, nil
}

func (s *S3Service) UploadBase64Image(base64Data string, key string, prefix string) (string, error) {
	// Remove data URI prefix if exists
	const dataURIPrefix = "data:image/png;base64,"
	base64Data = strings.TrimPrefix(base64Data, dataURIPrefix)

	// Decode base64 data
	imageData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 data: %v", err)
	}

	// Generate unique filename with timestamp
	filename := fmt.Sprintf("%s/%d.png", prefix, time.Now().Unix())

	// Upload to S3
	_, err = s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(filename),
		Body:        bytes.NewReader(imageData),
		ContentType: aws.String("image/png"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %v", err)
	}

	// Return the S3 URL
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, filename), nil
}
