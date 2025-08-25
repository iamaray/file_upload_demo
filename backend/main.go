package main

import (
	// "fmt"
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxUploadBytes = 200 << 20
	uploadDir      = "./data/uploads"
)

type UploadResponse struct {
	ID          string `json:"id"`
	Bytes       int64  `json:"bytesWritten"`
	ChecksumSHA string `json:"sha256"`
	ContentType string `json:"contentType"`
	Filename    string `json:"filename"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func UploadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, "Only POST method is allowed for file uploads")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

		mr, err := r.MultipartReader()
		if err != nil {
			writeBadRequest(w, "Invalid multipart form data")
			return
		}

		id, err := randomHex(16)
		if err != nil {
			writeInternalError(w, "Failed to generate file ID")
			return
		}

		part, err := mpProc(mr)
		if err != nil {
			if errors.Is(err, http.ErrMissingFile) {
				writeBadRequest(w, "No file provided in 'file' field")
			} else if err.Error() == "no filename provided" {
				writeBadRequest(w, "No filename provided for uploaded file")
			} else {
				writeBadRequest(w, "Error processing multipart data: "+err.Error())
			}
			return
		}
		defer part.Close()

		now := time.Now()
		dir := filepath.Join(uploadDir, now.Format("2006"), now.Format("01"))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			writeInternalError(w, "Failed to create upload directory")
			return
		}

		tmpPath := filepath.Join(dir, id+".csv.part")
		finalPath := strings.TrimSuffix(tmpPath, ".part")

		dstFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			writeInternalError(w, "Failed to create temporary file")
			return
		}

		defer func() {
			dstFile.Close()
			if _, statErr := os.Stat(finalPath); os.IsNotExist(statErr) {
				_ = os.Remove(tmpPath)
			}
		}()

		bufWriter := bufio.NewWriterSize(dstFile, 1<<20)

		head := make([]byte, 512)
		nHead, _ := io.ReadFull(part, head)
		head = head[:nHead]
		contentType := http.DetectContentType(pad512(head))
		filename := part.Part.FileName()
		if !isAllowedCSV(contentType, filename) {
			ext := strings.ToLower(filepath.Ext(filename))
			if ext != ".csv" {
				writeUnsupportedMediaType(w, "Only CSV files are allowed. File extension '"+ext+"' is not supported")
			} else {
				writeUnsupportedMediaType(w, "File content type '"+contentType+"' is not supported for CSV files")
			}
			return
		}

		h := sha256.New()
		mw := io.MultiWriter(bufWriter, h)

		var written int64
		if nHead > 0 {
			if _, err := mw.Write(head); err != nil {
				writeInternalError(w, "Failed to write file data")
				return
			}
			written += int64(nHead)
		}

		n, err := io.Copy(mw, part)
		written += n
		if err != nil {
			if !errors.Is(err, io.EOF) {
				if strings.Contains(err.Error(), "request body too large") {
					writeRequestEntityTooLarge(w, "File size exceeds maximum allowed size of 200MB")
				} else {
					writeInternalError(w, "Failed to copy file data")
				}
				return
			}
		}

		if written == 0 {
			writeBadRequest(w, "Uploaded file is empty")
			return
		}

		if err := bufWriter.Flush(); err != nil {
			writeInternalError(w, "Failed to flush file buffer")
			return
		}
		if err := dstFile.Close(); err != nil {
			writeInternalError(w, "Failed to close file")
			return
		}
		if err := os.Rename(tmpPath, finalPath); err != nil {
			writeInternalError(w, "Failed to finalize file")
			return
		}

		resp := UploadResponse{
			ID:          id,
			Bytes:       written,
			ChecksumSHA: hex.EncodeToString(h.Sum(nil)),
			ContentType: contentType,
			Filename:    filepath.Base(finalPath),
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

type multipartPart struct {
	*multipart.Part
}

func mpProc(mr *multipart.Reader) (*multipartPart, error) {
	for {
		p, perr := mr.NextPart()
		if errors.Is(perr, io.EOF) {
			break
		}

		if perr != nil {
			return &multipartPart{Part: nil}, perr
		}

		if p.FormName() == "file" {
			if p.FileName() == "" {
				p.Close()
				return &multipartPart{Part: nil}, errors.New("no filename provided")
			}
			return &multipartPart{Part: p}, nil
		}
		_ = p.Close()
	}
	return &multipartPart{Part: nil}, http.ErrMissingFile
}

func randomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

func pad512(b []byte) []byte {
	if len(b) >= 512 {
		return b[:512]
	}
	tmp := make([]byte, 512)
	copy(tmp, b)
	return tmp
}

func isAllowedCSV(contentType, filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".csv" {
		return false
	}

	switch contentType {
	case "text/csv", "application/vnd.ms-excel", "text/plain", "application/octet-stream":
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, errorType, message string) {
	errResp := ErrorResponse{
		Error:   errorType,
		Message: message,
		Code:    status,
	}
	writeJSON(w, status, errResp)
}

func writeInternalError(w http.ResponseWriter, message string) {
	writeError(w, http.StatusInternalServerError, "internal_server_error", message)
}

func writeBadRequest(w http.ResponseWriter, message string) {
	writeError(w, http.StatusBadRequest, "bad_request", message)
}

func writeUnsupportedMediaType(w http.ResponseWriter, message string) {
	writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", message)
}

func writeMethodNotAllowed(w http.ResponseWriter, message string) {
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", message)
}

func writeRequestEntityTooLarge(w http.ResponseWriter, message string) {
	writeError(w, http.StatusRequestEntityTooLarge, "request_entity_too_large", message)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/files/", UploadHandler())

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	log.Println("listening on :8080")
	log.Fatal(srv.ListenAndServe())
}
