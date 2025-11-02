package main

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
)

// TestContainsLinks verifies URL detection in posts
func TestContainsLinks(t *testing.T) {
	p := &Plugin{}

	tests := []struct {
		name      string
		post      *model.Post
		hasLinks  bool
	}{
		{
			name: "http URL in message",
			post: &model.Post{
				Message: "Check out http://example.com for more info",
			},
			hasLinks: true,
		},
		{
			name: "https URL in message",
			post: &model.Post{
				Message: "Visit https://example.com",
			},
			hasLinks: true,
		},
		{
			name: "www URL without protocol",
			post: &model.Post{
				Message: "Go to www.example.com",
			},
			hasLinks: true,
		},
		{
			name: "plain text without links",
			post: &model.Post{
				Message: "This is just regular text without any links",
			},
			hasLinks: false,
		},
		{
			name: "multiple URLs in message",
			post: &model.Post{
				Message: "Visit https://example.com and http://test.org",
			},
			hasLinks: true,
		},
		{
			name: "URL with query parameters",
			post: &model.Post{
				Message: "Search: https://example.com/search?q=test&page=1",
			},
			hasLinks: true,
		},
		{
			name: "URL in markdown link",
			post: &model.Post{
				Message: "Click [here](https://example.com) for details",
			},
			hasLinks: true,
		},
		{
			name: "post with OpenGraph embeds",
			post: &model.Post{
				Message: "https://example.com",
				Metadata: &model.PostMetadata{
					Embeds: []*model.PostEmbed{
						{
							Type: model.PostEmbedOpengraph,
							URL:  "https://example.com",
						},
					},
				},
			},
			hasLinks: true,
		},
		{
			name: "empty post",
			post: &model.Post{
				Message: "",
			},
			hasLinks: false,
		},
		{
			name: "text that looks like URL but isn't",
			post: &model.Post{
				Message: "example.com is a domain but not a link",
			},
			hasLinks: false,
		},
		{
			name: "URL with special characters",
			post: &model.Post{
				Message: "https://example.com/path-with_special.chars",
			},
			hasLinks: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.containsLinks(tt.post)
			assert.Equal(t, tt.hasLinks, result, "containsLinks() should return %v for %s", tt.hasLinks, tt.name)
		})
	}
}

// TestContainsImages verifies image detection in posts
func TestContainsImages(t *testing.T) {
	p := &Plugin{}

	tests := []struct {
		name      string
		post      *model.Post
		hasImages bool
	}{
		{
			name: "post with jpg file attachment",
			post: &model.Post{
				Message: "Here's a photo",
				Metadata: &model.PostMetadata{
					Files: []*model.FileInfo{
						{
							Extension: ".jpg",
							Name:      "photo.jpg",
						},
					},
				},
			},
			hasImages: true,
		},
		{
			name: "post with png file attachment",
			post: &model.Post{
				Message: "Screenshot attached",
				Metadata: &model.PostMetadata{
					Files: []*model.FileInfo{
						{
							Extension: ".png",
							Name:      "screenshot.png",
						},
					},
				},
			},
			hasImages: true,
		},
		{
			name: "post with gif file attachment",
			post: &model.Post{
				Message: "Funny gif",
				Metadata: &model.PostMetadata{
					Files: []*model.FileInfo{
						{
							Extension: ".gif",
							Name:      "animation.gif",
						},
					},
				},
			},
			hasImages: true,
		},
		{
			name: "post with webp file attachment",
			post: &model.Post{
				Message: "Modern image format",
				Metadata: &model.PostMetadata{
					Files: []*model.FileInfo{
						{
							Extension: ".webp",
							Name:      "image.webp",
						},
					},
				},
			},
			hasImages: true,
		},
		{
			name: "post with PDF attachment (not an image)",
			post: &model.Post{
				Message: "Document attached",
				Metadata: &model.PostMetadata{
					Files: []*model.FileInfo{
						{
							Extension: ".pdf",
							Name:      "document.pdf",
						},
					},
				},
			},
			hasImages: false,
		},
		{
			name: "post with text file (not an image)",
			post: &model.Post{
				Message: "Log file",
				Metadata: &model.PostMetadata{
					Files: []*model.FileInfo{
						{
							Extension: ".txt",
							Name:      "log.txt",
						},
					},
				},
			},
			hasImages: false,
		},
		{
			name: "post with image embed in metadata",
			post: &model.Post{
				Message: "https://example.com/image.jpg",
				Metadata: &model.PostMetadata{
					Images: map[string]*model.PostImage{
						"https://example.com/image.jpg": {},
					},
				},
			},
			hasImages: true,
		},
		{
			name: "post with markdown image syntax",
			post: &model.Post{
				Message: "![alt text](https://example.com/image.png)",
			},
			hasImages: true,
		},
		{
			name: "plain text post without images",
			post: &model.Post{
				Message: "Just a regular text message",
			},
			hasImages: false,
		},
		{
			name: "empty post",
			post: &model.Post{
				Message: "",
			},
			hasImages: false,
		},
		{
			name: "post with multiple image attachments",
			post: &model.Post{
				Message: "Multiple photos",
				Metadata: &model.PostMetadata{
					Files: []*model.FileInfo{
						{
							Extension: ".jpg",
							Name:      "photo1.jpg",
						},
						{
							Extension: ".png",
							Name:      "photo2.png",
						},
					},
				},
			},
			hasImages: true,
		},
		{
			name: "post with mixed attachments (image and non-image)",
			post: &model.Post{
				Message: "Photo and document",
				Metadata: &model.PostMetadata{
					Files: []*model.FileInfo{
						{
							Extension: ".pdf",
							Name:      "doc.pdf",
						},
						{
							Extension: ".jpg",
							Name:      "photo.jpg",
						},
					},
				},
			},
			hasImages: true,
		},
		{
			name: "post with uppercase extension",
			post: &model.Post{
				Message: "Image with uppercase extension",
				Metadata: &model.PostMetadata{
					Files: []*model.FileInfo{
						{
							Extension: ".JPG",
							Name:      "PHOTO.JPG",
						},
					},
				},
			},
			hasImages: true,
		},
		{
			name: "post with jpeg extension",
			post: &model.Post{
				Message: "JPEG image",
				Metadata: &model.PostMetadata{
					Files: []*model.FileInfo{
						{
							Extension: ".jpeg",
							Name:      "photo.jpeg",
						},
					},
				},
			},
			hasImages: true,
		},
		{
			name: "post with bmp file attachment",
			post: &model.Post{
				Message: "Bitmap image",
				Metadata: &model.PostMetadata{
					Files: []*model.FileInfo{
						{
							Extension: ".bmp",
							Name:      "image.bmp",
						},
					},
				},
			},
			hasImages: true,
		},
		{
			name: "markdown image without actual image URL",
			post: &model.Post{
				Message: "Text that mentions ![](but is incomplete",
			},
			hasImages: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.containsImages(tt.post)
			assert.Equal(t, tt.hasImages, result, "containsImages() should return %v for %s", tt.hasImages, tt.name)
		})
	}
}

// TestIsUserTooNew verifies user age validation logic
func TestIsUserTooNew(t *testing.T) {
	p := &Plugin{}

	now := time.Now()
	newUserCreateAt := now.Unix() * 1000                          // Created now
	oldUserCreateAt := now.Add(-48 * time.Hour).Unix() * 1000     // Created 48 hours ago
	edgeCaseCreateAt := now.Add(-24 * time.Hour).Unix() * 1000    // Created exactly 24 hours ago
	almostOldCreateAt := now.Add(-23*time.Hour - 59*time.Minute).Unix() * 1000 // Just under 24h

	tests := []struct {
		name          string
		user          *model.User
		blockDuration string
		contentType   string
		expectTooNew  bool
		expectError   bool
		errorContains string
	}{
		{
			name: "new user within 24h blocking period",
			user: &model.User{
				Id:       model.NewId(),
				CreateAt: newUserCreateAt,
			},
			blockDuration: "24h",
			contentType:   "links",
			expectTooNew:  true,
			expectError:   false,
		},
		{
			name: "old user outside 24h blocking period",
			user: &model.User{
				Id:       model.NewId(),
				CreateAt: oldUserCreateAt,
			},
			blockDuration: "24h",
			contentType:   "links",
			expectTooNew:  false,
			expectError:   false,
		},
		{
			name: "indefinite blocking with -1 duration",
			user: &model.User{
				Id:       model.NewId(),
				CreateAt: oldUserCreateAt, // Even old users are blocked
			},
			blockDuration: "-1",
			contentType:   "direct messages",
			expectTooNew:  true,
			expectError:   false,
		},
		{
			name: "new user with indefinite blocking",
			user: &model.User{
				Id:       model.NewId(),
				CreateAt: newUserCreateAt,
			},
			blockDuration: "-1",
			contentType:   "images",
			expectTooNew:  true,
			expectError:   false,
		},
		{
			name: "invalid duration format",
			user: &model.User{
				Id:       model.NewId(),
				CreateAt: newUserCreateAt,
			},
			blockDuration: "invalid",
			contentType:   "links",
			expectTooNew:  false,
			expectError:   true,
			errorContains: "failed to parse duration",
		},
		{
			name: "empty duration string",
			user: &model.User{
				Id:       model.NewId(),
				CreateAt: newUserCreateAt,
			},
			blockDuration: "",
			contentType:   "links",
			expectTooNew:  false,
			expectError:   true,
			errorContains: "failed to parse duration",
		},
		{
			name: "user at exact boundary of 24h duration",
			user: &model.User{
				Id:       model.NewId(),
				CreateAt: edgeCaseCreateAt,
			},
			blockDuration: "24h",
			contentType:   "links",
			expectTooNew:  false, // At exactly 24h, should be allowed
			expectError:   false,
		},
		{
			name: "user just under 24h duration",
			user: &model.User{
				Id:       model.NewId(),
				CreateAt: almostOldCreateAt,
			},
			blockDuration: "24h",
			contentType:   "links",
			expectTooNew:  true, // Just under 24h, should still be blocked
			expectError:   false,
		},
		{
			name: "short 1h blocking period",
			user: &model.User{
				Id:       model.NewId(),
				CreateAt: now.Add(-30 * time.Minute).Unix() * 1000,
			},
			blockDuration: "1h",
			contentType:   "direct messages",
			expectTooNew:  true,
			expectError:   false,
		},
		{
			name: "long 7d blocking period with new user",
			user: &model.User{
				Id:       model.NewId(),
				CreateAt: now.Add(-3 * 24 * time.Hour).Unix() * 1000, // 3 days old
			},
			blockDuration: "168h", // 7 days
			contentType:   "images",
			expectTooNew:  true,
			expectError:   false,
		},
		{
			name: "long 7d blocking period with old user",
			user: &model.User{
				Id:       model.NewId(),
				CreateAt: now.Add(-8 * 24 * time.Hour).Unix() * 1000, // 8 days old
			},
			blockDuration: "168h", // 7 days
			contentType:   "images",
			expectTooNew:  false,
			expectError:   false,
		},
		{
			name: "complex duration format 12h30m",
			user: &model.User{
				Id:       model.NewId(),
				CreateAt: now.Add(-10 * time.Hour).Unix() * 1000, // 10 hours old
			},
			blockDuration: "12h30m",
			contentType:   "links",
			expectTooNew:  true,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isTooNew, errorMsg, err := p.isUserTooNew(tt.user, tt.blockDuration, tt.contentType)

			if tt.expectError {
				assert.Error(t, err, "Expected error for test: %s", tt.name)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains, "Error should contain expected text")
				}
			} else {
				assert.NoError(t, err, "Expected no error for test: %s", tt.name)
				assert.Equal(t, tt.expectTooNew, isTooNew, "isUserTooNew() should return %v for %s", tt.expectTooNew, tt.name)

				if isTooNew {
					assert.NotEmpty(t, errorMsg, "Error message should not be empty when user is too new")
					assert.Contains(t, errorMsg, tt.contentType, "Error message should mention content type")
				} else {
					assert.Empty(t, errorMsg, "Error message should be empty when user is allowed")
				}
			}
		})
	}
}
