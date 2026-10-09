package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxImageBytes   = 5 << 20
	maxCaptionRunes = 140
	galleryPageSize = 48
)

// Only these types are accepted, detected from the file's bytes rather than
// trusting the browser-supplied name or Content-Type.
var imageExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

var mediaKeyPattern = regexp.MustCompile(`^(avatars|gallery)/[0-9a-f]{32}\.(jpg|png|gif|webp)$`)

func mediaPath(key string) string {
	if key == "" {
		return ""
	}
	return "/media/" + key
}

func newImageKey(prefix, ext string) string {
	buf := make([]byte, 16)
	rand.Read(buf)
	return prefix + "/" + hex.EncodeToString(buf) + ext
}

type upload struct {
	data        []byte
	contentType string
	ext         string
	caption     string
}

type uploadError struct {
	status  int
	message string
}

// readUpload streams a multipart form with an "image" file and an optional
// "caption", holding at most maxImageBytes of image in memory.
func readUpload(w http.ResponseWriter, r *http.Request) (upload, *uploadError) {
	tooLarge := &uploadError{http.StatusRequestEntityTooLarge, "Images must be 5 MB or smaller."}
	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes+16<<10)
	reader, err := r.MultipartReader()
	if err != nil {
		return upload{}, &uploadError{http.StatusBadRequest, "Send the image as a multipart form."}
	}
	var u upload
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			if errors.As(err, new(*http.MaxBytesError)) {
				return upload{}, tooLarge
			}
			return upload{}, &uploadError{http.StatusBadRequest, "Could not read the upload."}
		}
		switch part.FormName() {
		case "image":
			u.data, err = io.ReadAll(io.LimitReader(part, maxImageBytes+1))
			if len(u.data) > maxImageBytes {
				return upload{}, tooLarge
			}
		case "caption":
			var b []byte
			b, err = io.ReadAll(io.LimitReader(part, 4<<10))
			u.caption = strings.TrimSpace(strings.ToValidUTF8(string(b), ""))
		}
		part.Close()
		if err != nil {
			if errors.As(err, new(*http.MaxBytesError)) {
				return upload{}, tooLarge
			}
			return upload{}, &uploadError{http.StatusBadRequest, "Could not read the upload."}
		}
	}
	if len(u.data) == 0 {
		return upload{}, &uploadError{http.StatusBadRequest, "Choose an image to upload."}
	}
	if utf8.RuneCountInString(u.caption) > maxCaptionRunes {
		return upload{}, &uploadError{http.StatusBadRequest, "Keep captions to 140 characters or fewer."}
	}
	u.contentType = http.DetectContentType(u.data)
	ext, ok := imageExtensions[u.contentType]
	if !ok {
		return upload{}, &uploadError{http.StatusUnsupportedMediaType, "Upload a JPEG, PNG, GIF or WebP image."}
	}
	u.ext = ext
	return u, nil
}

// prepareUpload checks the caller may upload and reads the image.
func (a *App) prepareUpload(w http.ResponseWriter, r *http.Request) (User, upload, bool) {
	user, ok := a.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Sign in to upload images.")
		return User{}, upload{}, false
	}
	if a.images == nil {
		writeError(w, http.StatusServiceUnavailable, "Image uploads aren't set up on this server yet.")
		return User{}, upload{}, false
	}
	u, problem := readUpload(w, r)
	if problem != nil {
		writeError(w, problem.status, problem.message)
		return User{}, upload{}, false
	}
	return user, u, true
}

func (a *App) handleAvatarUpload(w http.ResponseWriter, r *http.Request) {
	user, u, ok := a.prepareUpload(w, r)
	if !ok {
		return
	}
	key := newImageKey("avatars", u.ext)
	if err := a.images.Put(r.Context(), key, u.contentType, u.data); err != nil {
		log.Printf("upload avatar to S3: %v", err)
		writeError(w, http.StatusBadGateway, "Could not save your photo. Please try again.")
		return
	}
	if err := a.store.SetAvatar(r.Context(), user.ID, key); err != nil {
		log.Printf("save avatar: %v", err)
		writeError(w, http.StatusInternalServerError, "Could not save your photo. Please try again.")
		return
	}
	user.AvatarKey = key
	writeJSON(w, http.StatusOK, signedIn(user))
}

type photoResponse struct {
	ID        int64     `json:"id"`
	ImageURL  string    `json:"imageUrl"`
	Caption   string    `json:"caption"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
}

func toPhotoResponse(p Photo) photoResponse {
	return photoResponse{ID: p.ID, ImageURL: mediaPath(p.ImageKey), Caption: p.Caption, Author: p.Author, CreatedAt: p.CreatedAt}
}

func (a *App) handleGalleryList(w http.ResponseWriter, r *http.Request) {
	photos, err := a.store.ListPhotos(r.Context(), galleryPageSize)
	if err != nil {
		log.Printf("list photos: %v", err)
		writeError(w, http.StatusInternalServerError, "Could not load the gallery.")
		return
	}
	body := struct {
		UploadsEnabled bool            `json:"uploadsEnabled"`
		Photos         []photoResponse `json:"photos"`
	}{UploadsEnabled: a.images != nil, Photos: make([]photoResponse, len(photos))}
	for i, p := range photos {
		body.Photos[i] = toPhotoResponse(p)
	}
	writeJSON(w, http.StatusOK, body)
}

func (a *App) handleGalleryUpload(w http.ResponseWriter, r *http.Request) {
	user, u, ok := a.prepareUpload(w, r)
	if !ok {
		return
	}
	key := newImageKey("gallery", u.ext)
	if err := a.images.Put(r.Context(), key, u.contentType, u.data); err != nil {
		log.Printf("upload gallery photo to S3: %v", err)
		writeError(w, http.StatusBadGateway, "Could not save your photo. Please try again.")
		return
	}
	photo, err := a.store.AddPhoto(r.Context(), user.ID, key, u.caption)
	if err != nil {
		log.Printf("save gallery photo: %v", err)
		writeError(w, http.StatusInternalServerError, "Could not save your photo. Please try again.")
		return
	}
	writeJSON(w, http.StatusCreated, toPhotoResponse(photo))
}

// handleMedia redirects to the image in S3, so the bucket can stay private
// and image URLs stored in the database never expire.
func (a *App) handleMedia(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if a.images == nil || !mediaKeyPattern.MatchString(key) {
		http.NotFound(w, r)
		return
	}
	target, err := a.images.URL(r.Context(), key)
	if err != nil {
		log.Printf("image URL for %s: %v", key, err)
		http.Error(w, "image unavailable", http.StatusBadGateway)
		return
	}
	// Presigned links last an hour; let browsers reuse the redirect for half that.
	w.Header().Set("Cache-Control", "private, max-age=1800")
	http.Redirect(w, r, target, http.StatusFound)
}
