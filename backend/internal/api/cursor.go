package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"pixora/backend/internal/repository"
)

// CursorVersion defines the current cursor format version.
const CursorVersion = 1

// CursorPayload is serialized into the opaque cursor token.
type CursorPayload struct {
	V              int        `json:"v"`
	Scope          string     `json:"s"`
	ID             string     `json:"id"`
	TakenAt        *time.Time `json:"t,omitempty"`
	FileModifiedAt *time.Time `json:"m,omitempty"`
	StringVal      string     `json:"sv,omitempty"`
	IntVal         int64      `json:"iv,omitempty"`
	TimeVal        *time.Time `json:"tv,omitempty"`
}

// ComputePhotoScope computes a deterministic scope fingerprint for a photo query.
func ComputePhotoScope(filter repository.PhotoFilter) string {
	var startStr, endStr string
	if filter.StartDate != nil {
		startStr = filter.StartDate.UTC().Format(time.RFC3339Nano)
	}
	if filter.EndDate != nil {
		endStr = filter.EndDate.UTC().Format(time.RFC3339Nano)
	}
	undatedStr := ""
	if filter.IsUndated != nil {
		undatedStr = fmt.Sprintf("%v", *filter.IsUndated)
	}

	canonical := fmt.Sprintf(
		"photos|q=%s|start=%s|end=%s|folder=%s|rec=%t|album=%s|media=%s|undated=%s|sort=%s|order=%s",
		strings.TrimSpace(filter.Query),
		startStr,
		endStr,
		filter.Folder,
		filter.Recursive,
		filter.AlbumID,
		filter.MediaType,
		undatedStr,
		strings.ToLower(filter.SortBy),
		strings.ToLower(filter.SortOrder),
	)
	hash := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", hash)[:16]
}

// ComputeTimelineScope computes a deterministic scope fingerprint for timeline queries.
func ComputeTimelineScope(filter repository.PhotoFilter) string {
	var startStr, endStr string
	if filter.StartDate != nil {
		startStr = filter.StartDate.UTC().Format(time.RFC3339Nano)
	}
	if filter.EndDate != nil {
		endStr = filter.EndDate.UTC().Format(time.RFC3339Nano)
	}
	undatedStr := ""
	if filter.IsUndated != nil {
		undatedStr = fmt.Sprintf("%v", *filter.IsUndated)
	}

	canonical := fmt.Sprintf(
		"timeline|q=%s|start=%s|end=%s|folder=%s|rec=%t|album=%s|media=%s|undated=%s",
		strings.TrimSpace(filter.Query),
		startStr,
		endStr,
		filter.Folder,
		filter.Recursive,
		filter.AlbumID,
		filter.MediaType,
		undatedStr,
	)
	hash := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", hash)[:16]
}

// ComputeFolderScope computes a deterministic scope fingerprint for folder photo queries.
func ComputeFolderScope(folderPath, sortBy, order string) string {
	canonical := fmt.Sprintf(
		"folder:%s|sort=%s|order=%s",
		folderPath,
		strings.ToLower(sortBy),
		strings.ToLower(order),
	)
	hash := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", hash)[:16]
}

// ComputeAlbumScope computes a deterministic scope fingerprint for album photo queries.
func ComputeAlbumScope(albumID string) string {
	canonical := fmt.Sprintf("album:%s", strings.TrimSpace(albumID))
	hash := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", hash)[:16]
}

// EncodeCursor generates an opaque base64 continuation token for a photo in a scoped collection.
func EncodeCursor(cursorData *repository.CursorData, scope string) (string, error) {
	if cursorData == nil || cursorData.ID == "" {
		return "", errors.New("cannot encode empty cursor")
	}

	payload := CursorPayload{
		V:              CursorVersion,
		Scope:          scope,
		ID:             cursorData.ID,
		TakenAt:        cursorData.TakenAt,
		FileModifiedAt: cursorData.FileModifiedAt,
		StringVal:      cursorData.StringVal,
		IntVal:         cursorData.IntVal,
		TimeVal:        cursorData.TimeVal,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cursor: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}

// DecodeCursor parses, validates, and unpacks an opaque cursor token against an expected scope.
func DecodeCursor(token string, expectedScope string) (*repository.CursorData, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil
	}

	// Support RawURLEncoding, standard URLEncoding, and standard StdEncoding
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		data, err = base64.URLEncoding.DecodeString(token)
		if err != nil {
			data, err = base64.StdEncoding.DecodeString(token)
			if err != nil {
				return nil, errors.New("invalid base64 encoding")
			}
		}
	}

	var payload CursorPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, errors.New("invalid cursor format")
	}

	if payload.V != CursorVersion {
		return nil, fmt.Errorf("unsupported cursor version: %d", payload.V)
	}

	if payload.ID == "" {
		return nil, errors.New("cursor missing identifier")
	}

	if payload.Scope != expectedScope {
		return nil, errors.New("cursor is incompatible with the current query parameters or filters")
	}

	return &repository.CursorData{
		ID:             payload.ID,
		TakenAt:        payload.TakenAt,
		FileModifiedAt: payload.FileModifiedAt,
		StringVal:      payload.StringVal,
		IntVal:         payload.IntVal,
		TimeVal:        payload.TimeVal,
	}, nil
}
