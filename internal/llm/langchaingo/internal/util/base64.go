package util

import (
	"encoding/base64"
	"errors"
	"strings"
)

// ExtractBase64Data extracts the MIME type and decoded bytes from a data URL.
// 例如：从 "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA..." 提取 "image/png" 和对应的字节数据
func ExtractBase64Data(dataURL string) ([]byte, string, error) {
	if !strings.HasPrefix(dataURL, "data:") {
		return nil, "", errors.New("not a data URL")
	}

	s := dataURL[5:]
	commaIndex := strings.Index(s, ",")
	if commaIndex == -1 {
		return nil, "", errors.New("invalid data URL format: no comma")
	}

	mediaTypeAndEncoding := s[:commaIndex]
	base64Data := s[commaIndex+1:]

	mimeType := mediaTypeAndEncoding
	if semicolonIndex := strings.Index(mediaTypeAndEncoding, ";"); semicolonIndex != -1 {
		mimeType = mediaTypeAndEncoding[:semicolonIndex]
	}

	if !strings.Contains(mediaTypeAndEncoding, ";base64") {
		return nil, "", errors.New("URL is not base64 encoded")
	}

	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, "", err
	}
	return data, mimeType, nil
}
