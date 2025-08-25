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
	uploadDir	   = "./data/uploads"
)

type UploadResponse struct {
	ID			string `json:"id"`
	Bytes   	int64  `json:"bytesWritten"`
	ChecksumSHA string `json:"sha256"`
	ContentType string `json:"contentType"`
	Filename	string `json:"filename"`
}

func UploadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			return
		}
		
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		
		mr, err := r.MultipartReader() 
		if err != nil {
			return
		}

		id, err := randomHex(16)
		if err != nil {
			return
		}

		part, err := mpProc(mr)
		if err != nil {
			return
		}

		now := time.Now()
		dir := filepath.Join(uploadDir, now.Format("2006"), now.Format("01"))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return
		}

		tmpPath := filepath.Join(dir, id+".csv.part")
		finalPath := strings.TrimSuffix(tmpPath, ".part")

		dstFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
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
		if !isAllowedCSV(contentType, part.Part.FileName()) {
			return 
		}

		h := sha256.New()
		mw := io.MultiWriter(bufWriter, h)

		var written int64
		if nHead > 0 {
			if _, err := mw.Write(head); err != nil {
				return
			}
			written += int64(nHead)
		}

		n, err := io.Copy(mw, part)
		written += n
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return
			}
		}

		if err := bufWriter.Flush(); err != nil { return }
		if err := dstFile.Close(); err != nil { return }
		if err := os.Rename(tmpPath, finalPath); err != nil { return }

		resp := UploadResponse{
			ID:	id,
			Bytes: written,
			ChecksumSHA: hex.EncodeToString(h.Sum(nil)),
			ContentType: contentType,
			Filename: filepath.Base(finalPath),
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

type multipartPart struct {
	*multipart.Part
}

func mpProc(mr *multipart.Reader) (*multipartPart, error) {
	var part *multipartPart
	for {
		p, perr := mr.NextPart()
		if errors.Is(perr, io.EOF) {
			break
		}

		if perr != nil {
			return &multipartPart{Part: nil}, perr
		}

		if p.FormName() == "file" {
			part = &multipartPart{Part: p}
			defer p.Close()
			break
		}
		_ = p.Close()
	}
	if part == nil {
		return &multipartPart{Part: nil}, http.ErrBodyNotAllowed
	}
	return part, nil
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
	switch contentType {
	case "text/csv", "application/vnd.ms-excel", "text/plain", "application/octet-stream":
	default:
		return false
	}
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".csv"

}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("v1/files", UploadHandler())

	srv := &http.Server{
		Addr: ":8080",
		Handler: mux,
		ReadTimeout: 60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	log.Println("listening on :8080")
	log.Fatal(srv.ListenAndServe())
}