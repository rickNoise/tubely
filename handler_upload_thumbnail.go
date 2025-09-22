package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	key := "thumbnail" // The key the web browser is using is called "thumbnail"

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse request into Multipart Form", err)
		return
	}

	file, header, err := r.FormFile(key)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Could not find file with key %s", key), err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")
	if mediaType == "" {
		respondWithError(w, http.StatusBadRequest, "No Content-Type header found", err)
		return
	}

	imageData, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not read from posted file data", err)
		return
	}

	videoMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error retrieving video metadata", err)
		return
	}
	// ensure the authenticated user is the video owner
	if videoMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Authenticated user is not the video owner", fmt.Errorf("authenticated user is not the video owner"))
		return
	}

	// Save the thumbnail to the global map
	newThumbnail := thumbnail{
		data:      imageData,
		mediaType: mediaType,
	}
	videoThumbnails[videoID] = newThumbnail

	// Update the video metadata so that it has a new thumbnail URL, then update the record in the database by using the cfg.db.UpdateVideo function.
	newThumbnailUrl := fmt.Sprintf("http://localhost:%s/api/thumbnails/%s", cfg.port, videoID)
	videoMetadata.ThumbnailURL = &newThumbnailUrl
	err = cfg.db.UpdateVideo(videoMetadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not update video metadata with new video thumbnail image", err)
		return
	}

	// Respond with updated JSON of the video's metadata. Use the provided respondWithJSON function and pass it the updated database.Video struct to marshal.
	respondWithJSON(w, http.StatusOK, videoMetadata)
}
