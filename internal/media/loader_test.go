package media

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoaderDecodesImageAndEnforcesLimits(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(1, 1, color.RGBA{R: 255, A: 255})
	var payload bytes.Buffer
	if err := png.Encode(&payload, img); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/image":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(payload.Bytes())
		case "/large":
			_, _ = w.Write(bytes.Repeat([]byte("x"), 2048))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	loader := NewLoader()
	loader.Client = server.Client()
	loader.MaxBytes = 1024
	got, err := loader.Load(context.Background(), server.URL+"/image")
	if err != nil || got.Bounds().Dx() != 3 || got.Bounds().Dy() != 2 {
		t.Fatalf("image = %#v, %v", got, err)
	}
	if _, err := loader.Load(context.Background(), server.URL+"/large"); err == nil {
		t.Fatal("large response should fail")
	}
	if _, err := loader.Load(context.Background(), "file:///tmp/image.png"); err == nil {
		t.Fatal("non-HTTP image should fail")
	}
}
