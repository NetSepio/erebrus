package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/NetSepio/erebrus/internal/store"
	"github.com/gin-gonic/gin"
)

func (s *Server) handlePutPeer(c *gin.Context) {
	id := c.Param("id")
	var req PeerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	if req.Name == "" || req.WGPublicKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and wg_public_key are required"})
		return
	}
	if s.status == "draining" {
		c.JSON(http.StatusConflict, gin.H{"error": "node is draining"})
		return
	}
	bundle, err := s.prov.UpsertPeer(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, store.ErrSubnetExhausted) {
			c.JSON(http.StatusConflict, gin.H{"error": "address pool exhausted"})
			return
		}
		// Bad WireGuard key is a client error; everything else is internal.
		// Detail is logged, never returned, to avoid leaking internals.
		slog.Warn("provision peer failed", "peer", id, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not provision peer"})
		return
	}
	c.JSON(http.StatusOK, bundle)
}

func (s *Server) handleDeletePeer(c *gin.Context) {
	if err := s.prov.DeletePeer(c.Request.Context(), c.Param("id")); err != nil {
		slog.Error("delete peer failed", "peer", c.Param("id"), "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) handleCredentials(c *gin.Context) {
	bundle, err := s.prov.Credentials(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown peer"})
			return
		}
		slog.Error("fetch credentials failed", "peer", c.Param("id"), "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, bundle)
}

func (s *Server) handleListPeers(c *gin.Context) {
	peers, err := s.prov.ListPeers(c.Request.Context())
	if err != nil {
		slog.Error("list peers failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, peers)
}
