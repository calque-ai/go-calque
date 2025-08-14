package ai

import (
	"io"
)

// MultimodalInput represents input that can contain multiple types of content.
//
// Supports text, images, audio, and video content through ContentPart entries.
// Uses streaming-first design where binary data is provided via io.Reader.
// When serialized to JSON, only metadata is included (not binary data).
//
// Example:
//
//	input := ai.MultimodalInput{
//		Parts: []ai.ContentPart{
//			{Type: "text", Text: "What's in this image?"},
//			{Type: "image", Reader: imageReader, MimeType: "image/jpeg"},
//		},
//	}
type MultimodalInput struct {
	Parts []ContentPart `json:"parts"`
}

// ContentPart represents a single piece of content within multimodal input.
//
// Supports different content types through the Type field:
// - "text": Text content in the Text field
// - "image": Image data via Reader field (streaming) OR Data field (simple cases)
// - "audio": Audio data via Reader field with MimeType (streaming only)
// - "video": Video data via Reader field with MimeType (streaming only)
//
// Two approaches for binary data:
// 1. Streaming: Use Reader field (not serialized, for large files)
// 2. Simple: Use Data field (base64 serialized in JSON, for small files)
//
// Example:
//
//	textPart := ai.ContentPart{Type: "text", Text: "Analyze this image"}
//	streamingImage := ai.ContentPart{Type: "image", Reader: imageReader, MimeType: "image/jpeg"}
//	simpleImage := ai.ContentPart{Type: "image", Data: imageBytes, MimeType: "image/jpeg"}
type ContentPart struct {
	Type     string    `json:"type"`                // "text", "image", "audio", "video"
	Text     string    `json:"text,omitempty"`      // For text parts
	Reader   io.Reader `json:"-"`                   // For streaming binary data (not serialized)
	Data     []byte    `json:"data,omitempty"`      // For simple binary data (base64 in JSON)
	MimeType string    `json:"mime_type,omitempty"` // For binary data
}

// Helper functions for creating content parts

// Text creates a text content part.
//
// Input: text string
// Output: ContentPart with type "text"
// Behavior: Creates a text-only content part
//
// Example:
//
//	part := ai.Text("What do you see in this image?")
func Text(text string) ContentPart {
	return ContentPart{
		Type: "text",
		Text: text,
	}
}

// Image creates an image content part for streaming data.
//
// Input: io.Reader containing image data, MIME type string
// Output: ContentPart with type "image" using streaming approach
// Behavior: Creates streaming image content part for large files
//
// Supports common image formats: image/jpeg, image/png, image/gif, etc.
// Data is read from the Reader when needed by the AI client.
// Use ImageData() for simple cases with small files.
//
// Example:
//
//	part := ai.Image(imageReader, "image/jpeg")
func Image(reader io.Reader, mimeType string) ContentPart {
	return ContentPart{
		Type:     "image",
		Reader:   reader,
		MimeType: mimeType,
	}
}

// ImageData creates an image content part for simple data.
//
// Input: []byte containing image data, MIME type string
// Output: ContentPart with type "image" using simple approach
// Behavior: Creates image content part that serializes data to JSON as base64
//
// Best for small image files where streaming is not needed.
// Data is embedded in JSON and sent to AI client directly.
// Use Image() for large files or streaming scenarios.
//
// Example:
//
//	part := ai.ImageData(imageBytes, "image/jpeg")
func ImageData(data []byte, mimeType string) ContentPart {
	return ContentPart{
		Type:     "image",
		Data:     data,
		MimeType: mimeType,
	}
}

// Audio creates an audio content part.
//
// Input: io.Reader containing audio data, MIME type string
// Output: ContentPart with type "audio"
// Behavior: Creates streaming audio content part
//
// Supports common audio formats: audio/wav, audio/mp3, audio/ogg, etc.
// Data is read from the Reader when needed by the AI client.
//
// Example:
//
//	part := ai.Audio(audioReader, "audio/wav")
func Audio(reader io.Reader, mimeType string) ContentPart {
	return ContentPart{
		Type:     "audio",
		Reader:   reader,
		MimeType: mimeType,
	}
}

// Video creates a video content part.
//
// Input: io.Reader containing video data, MIME type string
// Output: ContentPart with type "video"
// Behavior: Creates streaming video content part
//
// Supports common video formats: video/mp4, video/avi, video/webm, etc.
// Data is read from the Reader when needed by the AI client.
//
// Example:
//
//	part := ai.Video(videoReader, "video/mp4")
func Video(reader io.Reader, mimeType string) ContentPart {
	return ContentPart{
		Type:     "video",
		Reader:   reader,
		MimeType: mimeType,
	}
}

// Multimodal creates a MultimodalInput from the provided content parts.
//
// Input: variadic ContentPart arguments
// Output: MultimodalInput containing all parts
// Behavior: Convenience function for creating multimodal input
//
// Example:
//
//	input := ai.Multimodal(
//		ai.Text("What's in this image?"),
//		ai.Image(imageReader, "image/jpeg"),
//		ai.Audio(audioReader, "audio/wav"),
//	)
func Multimodal(parts ...ContentPart) MultimodalInput {
	return MultimodalInput{Parts: parts}
}