package storage

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/get10xteam/sales-module-backend/errs"
	"github.com/get10xteam/sales-module-backend/plumbings/utils"

	"github.com/gofiber/fiber/v2"
)

type UploadConfig struct {
	// when true, will call c.Next() when
	// Content-Type != multipart/form-data
	NonFormCallNext bool
	// when true, will act more like middleware
	// and put the resulting uploadUrl in c.Locals
	CallNextWhenDone bool
	// empty means any types is allowed
	// matched as prefix
	AllowedTypes []string
	// when set, takes precedence over AllowedTypes
	AllowedTypeFunc func(string) error
	// zero means no limit
	MaxSize int
	// Folder within org, skipped if empty
	PathPrefix string
	// when true, hash is added after FolderPrefix
	PathIncludeHash bool
	// when true, original fileName is added at the end
	PathIncludeOriginalFileName bool
	// when nil, will use the exported Upload function
	Uploader Uploader
	// when true, upload config can be processed with empty payload
	AllowEmpty bool
}

var safeFileNameRegex = regexp.MustCompile(`[^0-9a-zA-Z-_\.]+`)

const _uploadedUrlLocalsKey = "uploadedUrl"

func GetUploadedUrlFromHttp(c *fiber.Ctx) string {
	if uploadUrl, ok := c.Locals(_uploadedUrlLocalsKey).(string); ok {
		return uploadUrl
	}
	return ""
}

func UploadHandlerFactory(u *UploadConfig) func(c *fiber.Ctx) (err error) {
	return func(c *fiber.Ctx) (err error) {
		if !strings.HasPrefix(c.Get("Content-Type"), "multipart/form-data") {
			if u.NonFormCallNext {
				return c.Next()
			}
			return errs.ErrBadParameter().WithMessage("expected Content-Type = multipart/form-data")
		}
		formFileHeader, err := c.FormFile("file")
		if err != nil {
			if u.AllowEmpty {
				c.Locals(_uploadedUrlLocalsKey, "")
				return c.Next()
			}
			return errs.ErrBadParameter().WithMessage("form-data \"name=file\" not found")
		}
		formFileSize := int(formFileHeader.Size)
		fileContentType := formFileHeader.Header.Get("Content-Type")
		formFileName := safeFileNameRegex.ReplaceAllString(formFileHeader.Filename, "_")
		if u.AllowedTypeFunc != nil {
			if err = u.AllowedTypeFunc(fileContentType); err != nil {
				return errs.ErrBadParameter().WithMessage("form-data name=file Content-Type is invalid").WithDetail(err)
			}
		} else {
			if len(u.AllowedTypes) > 0 {
				var allowed bool
				for _, t := range u.AllowedTypes {
					if strings.HasPrefix(fileContentType, t) {
						allowed = true
						break
					}
				}
				if !allowed {
					return errs.ErrBadParameter().WithMessage("form-data name=file Content-Type is invalid").WithDetail(fiber.Map{"allowedContentType": u.AllowedTypes})
				}
			}
		}
		if u.MaxSize > 0 && formFileSize > u.MaxSize {
			return errs.ErrBadParameter().WithMessage("form-data \"name=file\" size is to large").WithDetail(fiber.Map{"maxSize": u.MaxSize})
		}
		formFile, err := formFileHeader.Open()
		if err != nil {
			return errs.ErrBadParameter().WithMessage("form-data \"name=file\" failed to open")
		}
		formFileBytes, err := io.ReadAll(formFile)
		if err != nil {
			return
		}
		if len(formFileBytes) != int(formFileSize) {
			return errs.ErrBadParameter().WithMessage("form-data \"name=file\" length is " + strconv.Itoa(len(formFileBytes)) + ", while the file size in header is " + strconv.Itoa(formFileSize))
		}
		sumB := md5.Sum(formFileBytes)
		sumStrURL := base64.RawURLEncoding.EncodeToString(sumB[:])
		var fileNameSegments []string
		if len(u.PathPrefix) > 0 {
			fileNameSegments = append(fileNameSegments, u.PathPrefix)
		}
		if u.PathIncludeHash || len(formFileName) == 0 {
			fileNameSegments = append(fileNameSegments, sumStrURL)
		}
		if u.PathIncludeOriginalFileName && len(formFileName) > 0 {
			fileNameSegments = append(fileNameSegments, formFileName)
		}
		if len(fileNameSegments) == 0 {
			fileNameSegments = append(fileNameSegments, sumStrURL)
		}
		if len(formFileName) == 0 {
			formFileName = sumStrURL
		}

		a := make([]string, (len(fileNameSegments) + 1))
		a[0] = "uploads"
		for i, s := range fileNameSegments {
			a[i+1] = s
		}
		fileNameSegments = a
		destPath := strings.Join(fileNameSegments, "/")
		var uploader Uploader = u.Uploader
		if uploader == nil {
			uploader = s.uploader
		}
		resultUrl, err := uploader.Upload(c.Context(), bytes.NewReader(formFileBytes), int64(formFileSize), formFileName, fileContentType, destPath)
		if err != nil {
			return
		}
		if u.CallNextWhenDone {
			c.Locals(_uploadedUrlLocalsKey, resultUrl)
			return c.Next()
		}
		return utils.FiberJSONWrap(c, resultUrl)
	}
}
