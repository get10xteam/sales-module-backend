package storage

import (
	"context"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3ConfigType struct {
	client        *s3.Client
	UsePathStyle  bool   `yaml:"UsePathStyle"`
	Region        string `yaml:"Region"`
	BaseEndpoint  string `yaml:"BaseEndpoint"`
	BasePublicURL string `yaml:"BasePublicURL"`
	Bucket        string `yaml:"Bucket"`
	Key           string `yaml:"Key"`
	Secret        string `yaml:"Secret"`
}

func (s *s3ConfigType) Initialize() (err error) {
	options := s3.Options{
		Region: s.Region,
		Credentials: aws.NewCredentialsCache(
			credentials.NewStaticCredentialsProvider(s.Key, s.Secret, ""),
		),
		UsePathStyle: s.UsePathStyle,
	}
	if len(s.BaseEndpoint) > 0 {
		s.BaseEndpoint = strings.TrimSuffix(s.BaseEndpoint, "/")
		options.BaseEndpoint = &s.BaseEndpoint
	}
	s.client = s3.New(options)
	if len(s.BasePublicURL) == 0 && s.client.Options().BaseEndpoint != nil {
		s.BasePublicURL = *s.client.Options().BaseEndpoint
	}
	s.BasePublicURL = strings.TrimSuffix(s.BasePublicURL, "/")
	return
}

func (s *s3ConfigType) Upload(ctx context.Context, file io.Reader, fileSize int64, fileName, contentType, destPath string) (resultUrl string, err error) {
	contentDisposition := "filename=\"" + fileName + "\""
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:             &s.Bucket,
		Key:                &destPath,
		Body:               file,
		ContentDisposition: &contentDisposition,
		ContentType:        &contentType,
		ContentLength:      &fileSize,
	})
	resultUrl = s.BasePublicURL + "/" + destPath
	return
}
