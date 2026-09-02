// Package media retrieves bounded article images for terminal mosaics.
package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Loader struct {
	Client   *http.Client
	MaxBytes int64
}

func NewLoader() *Loader {
	return &Loader{Client: &http.Client{Timeout: 10 * time.Second}, MaxBytes: 5 << 20}
}

func (l *Loader) Load(ctx context.Context, rawURL string) (image.Image, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("invalid image URL %q", rawURL)
	}
	client := l.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	limit := l.MaxBytes
	if limit <= 0 {
		limit = 5 << 20
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Newsfall/0.1 (+terminal feed reader)")
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/jpeg,image/gif,image/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	if resp.ContentLength > limit {
		return nil, fmt.Errorf("image exceeds %d bytes", limit)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("image exceeds %d bytes", limit)
	}
	img, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	if img == nil || img.Bounds().Empty() {
		return nil, errors.New("decoded image is empty")
	}
	return img, nil
}
