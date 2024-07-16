package storage

import (
	"context"
	"io"

	"github.com/pkg/errors"
)

type StorageConfigType struct {
	uploader Uploader
	Driver   string        `yaml:"Driver"`
	S3       *s3ConfigType `yaml:"S3"`
}

type Uploader interface {
	// fileName and destPath should not contain leading or trailing slash
	Upload(ctx context.Context, file io.Reader, fileSize int64, fileName, contentType, destPath string) (resultUrl string, err error)
}

func (s *StorageConfigType) Initialize() (err error) {
	switch s.Driver {
	case "S3":
		{
			err = s.S3.Initialize()
			if err != nil {
				return
			}
			s.uploader = s.S3
			return
		}
	default:
		return errors.New("unsupported storage driver: " + s.Driver)
	}
}

var s *StorageConfigType

func InitializeStorage(inStorage *StorageConfigType) (err error) {
	err = inStorage.Initialize()
	if err != nil {
		return
	}
	s = inStorage
	return
}
func Upload(ctx context.Context, file io.Reader, fileSize int64, fileName, contentType string, destPath string) (resultUrl string, err error) {
	return s.uploader.Upload(ctx, file, fileSize, fileName, contentType, destPath)
}
