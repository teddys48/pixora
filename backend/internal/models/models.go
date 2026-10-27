package models

import "time"

// Photo represents an indexed photo entity.
type Photo struct {
	ID               string     `json:"id"`
	RelativePath     string     `json:"relativePath"`
	OriginalFilename string     `json:"originalFilename"`
	MediaType        string     `json:"mediaType"`
	FileSize         int64      `json:"fileSize"`
	Width            *int       `json:"width,omitempty"`
	Height           *int       `json:"height,omitempty"`
	TakenAt          *time.Time `json:"takenAt,omitempty"`
	FileModifiedAt   time.Time  `json:"fileModifiedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// Album represents a virtual photo collection or album.
type Album struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	PhotoCount  int       `json:"photoCount"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// PhotoAlbum represents a many-to-many join relationship between a photo and an album.
type PhotoAlbum struct {
	PhotoID string    `json:"photoId"`
	AlbumID string    `json:"albumId"`
	AddedAt time.Time `json:"addedAt"`
}
