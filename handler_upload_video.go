package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	// The key the web browser is using is called "video"
	const key = "video"
	// Set an upload limit of 1 GB (1 << 30 bytes)
	const maxUploadSize = 1 << 30

	// Wrap the request body with MaxBytesReader
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	// Extract the videoID from the URL path and parse it as a UUID
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	// Authenticate the user to get a userID
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

	fmt.Println("uploading video", videoID, "by user", userID, "...")

	// Get the video metadata from the database.
	// If the user is not the video owner, return a http.StatusUnauthorized response.
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

	// Parse the uploaded video file from the form data
	file, header, err := r.FormFile(key)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Could not find file with key %s", key), err)
		return
	}
	defer file.Close()

	// Validate the uploaded file to ensure it's an MP4 video
	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil || mediaType == "" {
		respondWithError(w, http.StatusBadRequest, "No Content-Type header found", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("invalid media type: %v", mediaType), nil)
		return
	}

	// Save the uploaded file to a temporary file on disk
	tempFile, err := os.CreateTemp("", "tubely-upload*.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to create a temporary file for storing video upload", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	fmt.Println("created temp file", tempFile.Name())

	// io.Copy the contents over from the wire to the temp file
	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to stream video upload to local temp file", err)
		return
	}

	// Reset the tempFile's file pointer to the beginning with .Seek(0, io.SeekStart)
	// This will allow us to read the file again from the beginning
	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error uploading", err)
		return
	}

	// Create processed version of video with "fast start" encoding
	processedFilePath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error processing video for upload", err)
		return
	}
	processedFile, err := os.Open(processedFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error opening processed version of video before upload", err)
		return
	}
	defer processedFile.Close()

	// Prefix filename with aspect ratio identifier
	aspectRatioString, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		aspectRatioString = "other"
	}
	var aspectRatioPrefix string
	switch aspectRatioString {
	case "16:9":
		aspectRatioPrefix = "landscape"
	case "9:16":
		aspectRatioPrefix = "portrait"
	default:
		aspectRatioPrefix = "other"
	}

	// Put the object into S3 using PutObject
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".mp4"
	}
	randomBytes := make([]byte, 32)
	_, err = rand.Read(randomBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error uploading", err)
		return
	}
	fileKey := aspectRatioPrefix + "/" + hex.EncodeToString(randomBytes) + ext

	putObjectInput := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        processedFile,
		ContentType: &mediaType,
	}
	_, err = cfg.s3Client.PutObject(r.Context(), &putObjectInput)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error uploading", err)
		return
	}

	// Update the VideoURL of the video record in the database with the S3 bucket and key. S3 URLs are in the format https://<bucket-name>.s3.<region>.amazonaws.com/<key>. Make sure you use the correct region and bucket name!
	updatedVideoURL := fmt.Sprintf(
		"https://%s.s3.%s.amazonaws.com/%s",
		cfg.s3Bucket,
		cfg.s3Region,
		fileKey,
	)
	videoMetadata.VideoURL = &updatedVideoURL
	err = cfg.db.UpdateVideo(videoMetadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed updating file metadata", err)
		return
	}

	fmt.Println("video upload successful, updatedVideoURL:", updatedVideoURL)
	respondWithJSON(w, http.StatusOK, videoMetadata)
}
