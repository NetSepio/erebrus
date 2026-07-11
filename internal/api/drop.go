package api

import (
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/NetSepio/erebrus/internal/drop"
	"github.com/gin-gonic/gin"
)

const (
	dropDeclaredSizeHeader = "X-Erebrus-Declared-Size"
	dropSHA256Header       = "X-Erebrus-SHA256"
)

func (s *Server) handleDropStatus(c *gin.Context) {
	if s.drop == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Drop unavailable"})
		return
	}
	c.JSON(http.StatusOK, s.drop.Snapshot())
}

func (s *Server) handleDropUpload(c *gin.Context) {
	if s.drop == nil || !s.drop.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Drop disabled"})
		return
	}
	if contentType := c.GetHeader("Content-Type"); contentType != "" &&
		!strings.HasPrefix(contentType, "application/octet-stream") {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "content type must be application/octet-stream"})
		return
	}
	declaredSize, err := strconv.ParseInt(strings.TrimSpace(c.GetHeader(dropDeclaredSizeHeader)), 10, 64)
	if err != nil || declaredSize < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid declared size"})
		return
	}
	maxUpload := min(s.cfg.DropStorageMaxBytes, drop.MaxObjectBytes)
	if declaredSize > maxUpload ||
		(c.Request.ContentLength >= 0 && c.Request.ContentLength > declaredSize) {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "upload exceeds reserved size"})
		return
	}
	digest := strings.TrimSpace(c.GetHeader(dropSHA256Header))
	if digest != "" {
		raw, err := hex.DecodeString(digest)
		if err != nil || len(raw) != 32 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid SHA-256"})
			return
		}
	}
	result, err := s.drop.Upload(c.Request.Context(), drop.AddRequest{
		UploadID: c.Param("upload_id"), Body: c.Request.Body, DeclaredSize: declaredSize,
		MaxBytes: declaredSize, SHA256: digest,
	})
	if err != nil {
		writeDropError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"cid": result.CID, "size_bytes": result.Size, "pinned": true})
}

func (s *Server) handleDropRead(c *gin.Context) {
	body, err := s.drop.Read(c.Request.Context(), c.Param("cid"))
	if err != nil {
		writeDropError(c, err)
		return
	}
	defer body.Close()
	c.Header("Content-Type", "application/octet-stream")
	c.Status(http.StatusOK)
	written, err := io.Copy(c.Writer, body)
	if written > 0 {
		s.drop.RecordDownload(written)
	}
	if err != nil {
		c.Error(err)
	}
}

func (s *Server) handleDropPinStatus(c *gin.Context) {
	pinned, err := s.drop.PinStatus(c.Request.Context(), c.Param("cid"))
	if err != nil {
		writeDropError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"cid": c.Param("cid"), "pinned": pinned})
}

func (s *Server) handleDropUnpin(c *gin.Context) {
	if err := s.drop.Unpin(c.Request.Context(), c.Param("cid")); err != nil {
		writeDropError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) handleDropWebUI(c *gin.Context) {
	if s.drop == nil || !s.drop.WebUIAvailable() {
		c.JSON(http.StatusNotFound, gin.H{"error": "Drop WebUI unavailable"})
		return
	}
	target, _ := url.Parse(drop.DefaultKuboRPCURL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(req *http.Request) {
		director(req)
		path := strings.TrimPrefix(c.Param("path"), "/")
		req.URL.Path = "/" + path
		req.Host = target.Host
		req.Header.Del("Authorization")
		req.Header.Del("X-Erebrus-Node-Key")
		req.Header.Del("Origin")
		req.Header.Del("Referer")
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		location := resp.Header.Get("Location")
		if location == "" {
			return nil
		}
		redirect, err := url.Parse(location)
		if err != nil {
			return nil
		}
		if redirect.IsAbs() && redirect.Host != target.Host {
			return nil
		}
		redirect.Scheme = ""
		redirect.Host = ""
		redirect.Path = "/api/v2/drop/webui/" + strings.TrimPrefix(redirect.Path, "/")
		resp.Header.Set("Location", redirect.String())
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "Kubo WebUI unavailable", http.StatusBadGateway)
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func writeDropError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, drop.ErrDisabled):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Drop disabled"})
	case errors.Is(err, drop.ErrByteLimit):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "stream exceeded byte limit"})
	case errors.Is(err, drop.ErrStorageFull):
		c.JSON(http.StatusInsufficientStorage, gin.H{"error": "Drop storage is full"})
	case errors.Is(err, drop.ErrSizeMismatch), errors.Is(err, drop.ErrHashMismatch):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "upload verification failed"})
	case strings.Contains(err.Error(), "invalid CID"):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid CID"})
	default:
		c.JSON(http.StatusBadGateway, gin.H{"error": "Drop operation failed"})
	}
}
