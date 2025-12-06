package ai

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

const (
	typeImage = "image"
	typeAudio = "audio"
	typeVideo = "video"
	typeText  = "text"
)

func TestText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected ContentPart
	}{
		{
			name: "simple text",
			text: "Hello, world!",
			expected: ContentPart{
				Type: "text",
				Text: "Hello, world!",
			},
		},
		{
			name: "empty text",
			text: "",
			expected: ContentPart{
				Type: "text",
				Text: "",
			},
		},
		{
			name: "multiline text",
			text: "Line 1\nLine 2\nLine 3",
			expected: ContentPart{
				Type: "text",
				Text: "Line 1\nLine 2\nLine 3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Text(tt.text)

			if result.Type != tt.expected.Type {
				t.Errorf("Text() Type = %v, want %v", result.Type, tt.expected.Type)
			}
			if result.Text != tt.expected.Text {
				t.Errorf("Text() Text = %v, want %v", result.Text, tt.expected.Text)
			}
			if result.Reader != nil {
				t.Error("Text() Reader should be nil")
			}
			if result.Data != nil {
				t.Error("Text() Data should be nil")
			}
			if result.MimeType != "" {
				t.Error("Text() MimeType should be empty")
			}
		})
	}
}

func TestImage(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		mimeType string
	}{
		{
			name:     "jpeg image",
			content:  "fake-jpeg-data",
			mimeType: "image/jpeg",
		},
		{
			name:     "png image",
			content:  "fake-png-data",
			mimeType: "image/png",
		},
		{
			name:     "gif image",
			content:  "fake-gif-data",
			mimeType: "image/gif",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.content)
			result := Image(reader, tt.mimeType)

			if result.Type != typeImage {
				t.Errorf("Image() Type = %v, want %v", result.Type, typeImage)
			}
			if result.MimeType != tt.mimeType {
				t.Errorf("Image() MimeType = %v, want %v", result.MimeType, tt.mimeType)
			}
			if result.Reader == nil {
				t.Error("Image() Reader should not be nil")
			}
			if result.Data != nil {
				t.Error("Image() Data should be nil for streaming approach")
			}
			if result.Text != "" {
				t.Error("Image() Text should be empty")
			}

			// Verify reader contains expected content
			data, err := io.ReadAll(result.Reader)
			if err != nil {
				t.Errorf("Failed to read from Reader: %v", err)
			}
			if string(data) != tt.content {
				t.Errorf("Image() Reader content = %v, want %v", string(data), tt.content)
			}
		})
	}
}

func TestImageData(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		mimeType string
	}{
		{
			name:     "jpeg image data",
			data:     []byte("fake-jpeg-bytes"),
			mimeType: "image/jpeg",
		},
		{
			name:     "png image data",
			data:     []byte("fake-png-bytes"),
			mimeType: "image/png",
		},
		{
			name:     "empty data",
			data:     []byte{},
			mimeType: "image/jpeg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ImageData(tt.data, tt.mimeType)

			if result.Type != typeImage {
				t.Errorf("ImageData() Type = %v, want %v", result.Type, typeImage)
			}
			if result.MimeType != tt.mimeType {
				t.Errorf("ImageData() MimeType = %v, want %v", result.MimeType, tt.mimeType)
			}
			if result.Reader != nil {
				t.Error("ImageData() Reader should be nil for simple approach")
			}
			if !bytes.Equal(result.Data, tt.data) {
				t.Errorf("ImageData() Data = %v, want %v", result.Data, tt.data)
			}
			if result.Text != "" {
				t.Error("ImageData() Text should be empty")
			}
		})
	}
}

func TestAudio(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		mimeType string
	}{
		{
			name:     "wav audio",
			content:  "fake-wav-data",
			mimeType: "audio/wav",
		},
		{
			name:     "mp3 audio",
			content:  "fake-mp3-data",
			mimeType: "audio/mp3",
		},
		{
			name:     "ogg audio",
			content:  "fake-ogg-data",
			mimeType: "audio/ogg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.content)
			result := Audio(reader, tt.mimeType)

			if result.Type != typeAudio {
				t.Errorf("Audio() Type = %v, want %v", result.Type, typeAudio)
			}
			if result.MimeType != tt.mimeType {
				t.Errorf("Audio() MimeType = %v, want %v", result.MimeType, tt.mimeType)
			}
			if result.Reader == nil {
				t.Error("Audio() Reader should not be nil")
			}
			if result.Data != nil {
				t.Error("Audio() Data should be nil")
			}
			if result.Text != "" {
				t.Error("Audio() Text should be empty")
			}

			// Verify reader contains expected content
			data, err := io.ReadAll(result.Reader)
			if err != nil {
				t.Errorf("Failed to read from Reader: %v", err)
			}
			if string(data) != tt.content {
				t.Errorf("Audio() Reader content = %v, want %v", string(data), tt.content)
			}
		})
	}
}

func TestVideo(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		mimeType string
	}{
		{
			name:     "mp4 video",
			content:  "fake-mp4-data",
			mimeType: "video/mp4",
		},
		{
			name:     "avi video",
			content:  "fake-avi-data",
			mimeType: "video/avi",
		},
		{
			name:     "webm video",
			content:  "fake-webm-data",
			mimeType: "video/webm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.content)
			result := Video(reader, tt.mimeType)

			if result.Type != typeVideo {
				t.Errorf("Video() Type = %v, want %v", result.Type, typeVideo)
			}
			if result.MimeType != tt.mimeType {
				t.Errorf("Video() MimeType = %v, want %v", result.MimeType, tt.mimeType)
			}
			if result.Reader == nil {
				t.Error("Video() Reader should not be nil")
			}
			if result.Data != nil {
				t.Error("Video() Data should be nil")
			}
			if result.Text != "" {
				t.Error("Video() Text should be empty")
			}

			// Verify reader contains expected content
			data, err := io.ReadAll(result.Reader)
			if err != nil {
				t.Errorf("Failed to read from Reader: %v", err)
			}
			if string(data) != tt.content {
				t.Errorf("Video() Reader content = %v, want %v", string(data), tt.content)
			}
		})
	}
}

func TestMultimodal(t *testing.T) {
	tests := []struct {
		name          string
		parts         []ContentPart
		expectedLen   int
		expectedTypes []string
	}{
		{
			name:          "empty parts",
			parts:         []ContentPart{},
			expectedLen:   0,
			expectedTypes: []string{},
		},
		{
			name: "single text part",
			parts: []ContentPart{
				Text("Hello"),
			},
			expectedLen:   1,
			expectedTypes: []string{"text"},
		},
		{
			name: "text and image",
			parts: []ContentPart{
				Text("What's in this image?"),
				ImageData([]byte("image-data"), "image/jpeg"),
			},
			expectedLen:   2,
			expectedTypes: []string{"text", "image"},
		},
		{
			name: "multiple content types",
			parts: []ContentPart{
				Text("Analyze this media"),
				ImageData([]byte("image-data"), "image/jpeg"),
				Audio(strings.NewReader("audio-data"), "audio/wav"),
				Video(strings.NewReader("video-data"), "video/mp4"),
			},
			expectedLen:   4,
			expectedTypes: []string{"text", "image", "audio", "video"},
		},
		{
			name: "multiple text parts",
			parts: []ContentPart{
				Text("First instruction"),
				Text("Second instruction"),
				Text("Third instruction"),
			},
			expectedLen:   3,
			expectedTypes: []string{"text", "text", "text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Multimodal(tt.parts...)

			if len(result.Parts) != tt.expectedLen {
				t.Errorf("Multimodal() length = %v, want %v", len(result.Parts), tt.expectedLen)
			}

			for i, expectedType := range tt.expectedTypes {
				if i >= len(result.Parts) {
					t.Errorf("Multimodal() missing part at index %d", i)
					continue
				}
				if result.Parts[i].Type != expectedType {
					t.Errorf("Multimodal() Parts[%d].Type = %v, want %v", i, result.Parts[i].Type, expectedType)
				}
			}
		})
	}
}

func TestMultimodalVariadic(t *testing.T) {
	// Test that variadic arguments work correctly
	text1 := Text("First")
	text2 := Text("Second")
	image := ImageData([]byte("data"), "image/jpeg")

	result := Multimodal(text1, text2, image)

	if len(result.Parts) != 3 {
		t.Errorf("Multimodal() with variadic args length = %v, want 3", len(result.Parts))
	}
}

func TestContentPartFields(t *testing.T) {
	// Test that content parts have correct field structure
	t.Run("text part only sets text fields", func(t *testing.T) {
		part := Text("test")
		if part.Type != typeText || part.Text != "test" {
			t.Error("Text part should set Type and Text")
		}
		if part.Reader != nil || part.Data != nil || part.MimeType != "" {
			t.Error("Text part should not set Reader, Data, or MimeType")
		}
	})

	t.Run("streaming image sets correct fields", func(t *testing.T) {
		reader := strings.NewReader("data")
		part := Image(reader, "image/jpeg")
		if part.Type != typeImage || part.MimeType != "image/jpeg" || part.Reader == nil {
			t.Error("Image part should set Type, MimeType, and Reader")
		}
		if part.Data != nil || part.Text != "" {
			t.Error("Image part should not set Data or Text")
		}
	})

	t.Run("simple image sets correct fields", func(t *testing.T) {
		data := []byte("data")
		part := ImageData(data, "image/png")
		if part.Type != typeImage || part.MimeType != "image/png" || !bytes.Equal(part.Data, data) {
			t.Error("ImageData part should set Type, MimeType, and Data")
		}
		if part.Reader != nil || part.Text != "" {
			t.Error("ImageData part should not set Reader or Text")
		}
	})

	t.Run("audio sets correct fields", func(t *testing.T) {
		reader := strings.NewReader("audio")
		part := Audio(reader, "audio/mp3")
		if part.Type != typeAudio || part.MimeType != "audio/mp3" || part.Reader == nil {
			t.Error("Audio part should set Type, MimeType, and Reader")
		}
		if part.Data != nil || part.Text != "" {
			t.Error("Audio part should not set Data or Text")
		}
	})

	t.Run("video sets correct fields", func(t *testing.T) {
		reader := strings.NewReader("video")
		part := Video(reader, "video/webm")
		if part.Type != typeVideo || part.MimeType != "video/webm" || part.Reader == nil {
			t.Error("Video part should set Type, MimeType, and Reader")
		}
		if part.Data != nil || part.Text != "" {
			t.Error("Video part should not set Data or Text")
		}
	})
}
