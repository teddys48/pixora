package api_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"pixora/backend/internal/api"
	"pixora/backend/internal/models"
	"pixora/backend/internal/repository"

	"github.com/google/uuid"
)

func encodeCursorWithVersion(v int, scope string, data *repository.CursorData) string {
	payload := api.CursorPayload{
		V:     v,
		Scope: scope,
		ID:    data.ID,
	}
	b, _ := json.Marshal(payload)
	return base64.RawURLEncoding.EncodeToString(b)
}

func TestCursorEncodingAndScopeValidation(t *testing.T) {
	data := &repository.CursorData{
		ID:        "photo-123",
		StringVal: "test.jpg",
	}

	scopeA := "scope-query-a"
	scopeB := "scope-query-b"

	token, err := api.EncodeCursor(data, scopeA)
	if err != nil {
		t.Fatalf("EncodeCursor failed: %v", err)
	}
	if token == "" {
		t.Fatal("EncodeCursor returned empty token")
	}

	// Successful decode with matching scope
	decoded, err := api.DecodeCursor(token, scopeA)
	if err != nil {
		t.Fatalf("DecodeCursor failed with matching scope: %v", err)
	}
	if decoded.ID != "photo-123" || decoded.StringVal != "test.jpg" {
		t.Fatalf("Decoded cursor data mismatch: %+v", decoded)
	}

	// Mismatched scope must fail
	_, err = api.DecodeCursor(token, scopeB)
	if err == nil {
		t.Fatal("Expected error when decoding cursor with mismatched scope, got nil")
	}

	// Malformed tokens
	malformedTokens := []string{
		"not-base64!!!",
		"e30=", // "{}" (valid json, but missing version/data)
		encodeCursorWithVersion(999, scopeA, data), // unsupported version
	}

	for _, badToken := range malformedTokens {
		_, err := api.DecodeCursor(badToken, scopeA)
		if err == nil {
			t.Errorf("Expected DecodeCursor to reject bad token %q, but got nil", badToken)
		}
	}
}

func TestPhotosCursorPaginationLifecycle(t *testing.T) {
	app, db, _, _, _, cleanup := setupTestApp(t)
	defer cleanup()

	photoRepo := repository.NewSQLitePhotoRepository(db)
	ctx := context.Background()

	// Seed 7 photos with sequential taken_at and predictable filenames
	now := time.Now().UTC().Truncate(time.Second)
	for i := 1; i <= 7; i++ {
		taken := now.Add(time.Duration(i) * time.Hour)
		p := &models.Photo{
			ID:               fmt.Sprintf("photo-%02d", i),
			RelativePath:     fmt.Sprintf("img%02d.jpg", i),
			OriginalFilename: fmt.Sprintf("img%02d.jpg", i),
			FileSize:         int64(i * 1000),
			TakenAt:          &taken,
			FileModifiedAt:   taken,
			MediaType:        "image/jpeg",
		}
		if err := photoRepo.Create(ctx, p); err != nil {
			t.Fatalf("Failed to seed photo %d: %v", i, err)
		}
	}

	type PhotosPage struct {
		Photos     []api.PhotoResponse `json:"photos"`
		Items      []api.PhotoResponse `json:"items"`
		Total      int                 `json:"total"`
		NextCursor *string             `json:"nextCursor"`
		HasMore    bool                `json:"hasMore"`
	}

	// 1. Fetch Page 1: limit=3, sort=date, order=desc (newest first: photo-07, photo-06, photo-05)
	req1 := httptest.NewRequest("GET", "/api/photos?limit=3&sort=date&order=desc", nil)
	resp1, _ := app.Test(req1, -1)
	if resp1.StatusCode != 200 {
		t.Fatalf("Page 1 returned status %d", resp1.StatusCode)
	}
	body1, _ := io.ReadAll(resp1.Body)
	var page1 PhotosPage
	if err := json.Unmarshal(body1, &page1); err != nil {
		t.Fatalf("Failed to unmarshal page 1: %v", err)
	}

	if len(page1.Photos) != 3 {
		t.Fatalf("Expected 3 photos on page 1, got %d", len(page1.Photos))
	}
	if !page1.HasMore || page1.NextCursor == nil || *page1.NextCursor == "" {
		t.Fatalf("Expected hasMore=true and valid nextCursor on page 1")
	}
	if page1.Photos[0].ID != "photo-07" || page1.Photos[1].ID != "photo-06" || page1.Photos[2].ID != "photo-05" {
		t.Fatalf("Unexpected photos on page 1: %v, %v, %v", page1.Photos[0].ID, page1.Photos[1].ID, page1.Photos[2].ID)
	}

	// 2. Fetch Page 2: limit=3 using cursor from Page 1 (expected: photo-04, photo-03, photo-02)
	req2 := httptest.NewRequest("GET", fmt.Sprintf("/api/photos?limit=3&sort=date&order=desc&cursor=%s", *page1.NextCursor), nil)
	resp2, _ := app.Test(req2, -1)
	if resp2.StatusCode != 200 {
		t.Fatalf("Page 2 returned status %d", resp2.StatusCode)
	}
	body2, _ := io.ReadAll(resp2.Body)
	var page2 PhotosPage
	if err := json.Unmarshal(body2, &page2); err != nil {
		t.Fatalf("Failed to unmarshal page 2: %v", err)
	}

	if len(page2.Photos) != 3 {
		t.Fatalf("Expected 3 photos on page 2, got %d", len(page2.Photos))
	}
	if !page2.HasMore || page2.NextCursor == nil || *page2.NextCursor == "" {
		t.Fatalf("Expected hasMore=true and valid nextCursor on page 2")
	}
	if page2.Photos[0].ID != "photo-04" || page2.Photos[1].ID != "photo-03" || page2.Photos[2].ID != "photo-02" {
		t.Fatalf("Unexpected photos on page 2: %v, %v, %v", page2.Photos[0].ID, page2.Photos[1].ID, page2.Photos[2].ID)
	}

	// 3. Fetch Page 3: limit=3 using cursor from Page 2 (expected: photo-01, terminal page)
	req3 := httptest.NewRequest("GET", fmt.Sprintf("/api/photos?limit=3&sort=date&order=desc&cursor=%s", *page2.NextCursor), nil)
	resp3, _ := app.Test(req3, -1)
	if resp3.StatusCode != 200 {
		t.Fatalf("Page 3 returned status %d", resp3.StatusCode)
	}
	body3, _ := io.ReadAll(resp3.Body)
	var page3 PhotosPage
	if err := json.Unmarshal(body3, &page3); err != nil {
		t.Fatalf("Failed to unmarshal page 3: %v", err)
	}

	if len(page3.Photos) != 1 || page3.Photos[0].ID != "photo-01" {
		t.Fatalf("Expected photo-01 on page 3, got: %+v", page3.Photos)
	}
	if page3.HasMore || page3.NextCursor != nil {
		t.Fatalf("Expected hasMore=false and nextCursor=null on final page, got hasMore=%v, nextCursor=%v", page3.HasMore, page3.NextCursor)
	}

	// 4. Test Incompatible Cursor: Page 1's cursor (for sort=date&order=desc) used with sort=size
	reqIncompatible := httptest.NewRequest("GET", fmt.Sprintf("/api/photos?limit=3&sort=size&order=desc&cursor=%s", *page1.NextCursor), nil)
	respIncompatible, _ := app.Test(reqIncompatible, -1)
	if respIncompatible.StatusCode != 400 {
		t.Fatalf("Expected 400 Bad Request for incompatible cursor, got %d", respIncompatible.StatusCode)
	}

	// 5. Test Malformed Cursor (garbage string)
	reqBad := httptest.NewRequest("GET", "/api/photos?cursor=not-a-valid-cursor", nil)
	respBad, _ := app.Test(reqBad, -1)
	if respBad.StatusCode != 400 {
		t.Fatalf("Expected 400 Bad Request for malformed cursor, got %d", respBad.StatusCode)
	}
}

func TestAlbumAndFolderCursorPagination(t *testing.T) {
	app, db, store, _, _, cleanup := setupTestApp(t)
	defer cleanup()

	photoRepo := repository.NewSQLitePhotoRepository(db)
	albumRepo := repository.NewSQLiteAlbumRepository(db)
	ctx := context.Background()

	// Ensure /api/folders/photos is registered on the test app
	app.Get("/api/folders/photos", api.GetFolderPhotosHandler(store, photoRepo))

	// Create physical folder directory and dummy files
	travelAbs, _ := store.ResolvePath("Travel")
	_ = os.MkdirAll(travelAbs, 0755)
	otherAbs, _ := store.ResolvePath("OtherFolder")
	_ = os.MkdirAll(otherAbs, 0755)

	// Create an album
	album := &models.Album{
		ID:   uuid.New().String(),
		Name: "Vacation 2026",
	}
	if err := albumRepo.Create(ctx, album); err != nil {
		t.Fatalf("Failed to create album: %v", err)
	}

	// Seed 5 photos and add to album and physical folder
	now := time.Now().UTC()
	for i := 1; i <= 5; i++ {
		relPath := fmt.Sprintf("Travel/p%02d.jpg", i)
		absFile, _ := store.ResolvePath(relPath)
		_ = os.WriteFile(absFile, []byte("fake-jpeg-data"), 0644)

		taken := now.Add(time.Duration(i) * time.Minute)
		p := &models.Photo{
			ID:               fmt.Sprintf("alb-photo-%02d", i),
			RelativePath:     relPath,
			OriginalFilename: fmt.Sprintf("p%02d.jpg", i),
			FileSize:         1024,
			TakenAt:          &taken,
			FileModifiedAt:   taken,
			MediaType:        "image/jpeg",
		}
		if err := photoRepo.Create(ctx, p); err != nil {
			t.Fatalf("Failed to seed photo: %v", err)
		}
		// Distinct added_at timestamps
		time.Sleep(5 * time.Millisecond)
		if _, err := albumRepo.AddPhotos(ctx, album.ID, []string{p.ID}); err != nil {
			t.Fatalf("Failed to add photo to album: %v", err)
		}
	}

	type AlbumPhotosPage struct {
		Photos     []api.PhotoResponse `json:"photos"`
		Items      []api.PhotoResponse `json:"items"`
		Total      int                 `json:"total"`
		NextCursor *string             `json:"nextCursor"`
		HasMore    bool                `json:"hasMore"`
	}

	// 1. Fetch Album Page 1: limit=2
	reqA1 := httptest.NewRequest("GET", fmt.Sprintf("/api/albums/%s/photos?limit=2", album.ID), nil)
	respA1, _ := app.Test(reqA1, -1)
	if respA1.StatusCode != 200 {
		t.Fatalf("Album page 1 returned status %d", respA1.StatusCode)
	}
	var aPage1 AlbumPhotosPage
	bodyA1, _ := io.ReadAll(respA1.Body)
	_ = json.Unmarshal(bodyA1, &aPage1)

	if len(aPage1.Photos) != 2 || !aPage1.HasMore || aPage1.NextCursor == nil {
		t.Fatalf("Unexpected album page 1: %+v", aPage1)
	}

	// 2. Fetch Album Page 2: limit=2 using cursor
	reqA2 := httptest.NewRequest("GET", fmt.Sprintf("/api/albums/%s/photos?limit=2&cursor=%s", album.ID, *aPage1.NextCursor), nil)
	respA2, _ := app.Test(reqA2, -1)
	if respA2.StatusCode != 200 {
		t.Fatalf("Album page 2 returned status %d", respA2.StatusCode)
	}
	var aPage2 AlbumPhotosPage
	bodyA2, _ := io.ReadAll(respA2.Body)
	_ = json.Unmarshal(bodyA2, &aPage2)

	if len(aPage2.Photos) != 2 || !aPage2.HasMore || aPage2.NextCursor == nil {
		t.Fatalf("Unexpected album page 2: %+v", aPage2)
	}

	// 3. Album cursor used with a different album must be rejected (400)
	otherAlbumID := uuid.New().String()
	reqAIncompat := httptest.NewRequest("GET", fmt.Sprintf("/api/albums/%s/photos?limit=2&cursor=%s", otherAlbumID, *aPage1.NextCursor), nil)
	respAIncompat, _ := app.Test(reqAIncompat, -1)
	if respAIncompat.StatusCode != 400 {
		t.Fatalf("Expected 400 Bad Request for mismatched album cursor scope, got %d", respAIncompat.StatusCode)
	}

	// 4. Folder photos cursor pagination (/api/folders/photos)
	type FolderPhotosPage struct {
		Photos     []api.PhotoResponse `json:"photos"`
		Items      []api.PhotoResponse `json:"items"`
		Total      int                 `json:"total"`
		NextCursor *string             `json:"nextCursor"`
		HasMore    bool                `json:"hasMore"`
	}

	reqF1 := httptest.NewRequest("GET", "/api/folders/photos?path=Travel&limit=2", nil)
	respF1, _ := app.Test(reqF1, -1)
	if respF1.StatusCode != 200 {
		t.Fatalf("Folder photos page 1 returned status %d", respF1.StatusCode)
	}
	var fPage1 FolderPhotosPage
	bodyF1, _ := io.ReadAll(respF1.Body)
	_ = json.Unmarshal(bodyF1, &fPage1)

	if len(fPage1.Photos) != 2 || !fPage1.HasMore || fPage1.NextCursor == nil {
		t.Fatalf("Unexpected folder page 1: %+v", fPage1)
	}

	// Folder cursor used on different folder path must be rejected (400)
	reqFIncompat := httptest.NewRequest("GET", fmt.Sprintf("/api/folders/photos?path=OtherFolder&limit=2&cursor=%s", *fPage1.NextCursor), nil)
	respFIncompat, _ := app.Test(reqFIncompat, -1)
	if respFIncompat.StatusCode != 400 {
		t.Fatalf("Expected 400 Bad Request for mismatched folder cursor scope, got %d", respFIncompat.StatusCode)
	}
}
