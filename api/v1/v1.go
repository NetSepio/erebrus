package v1

import (
	"os"

	"github.com/NetSepio/erebrus/api/v1/agents"
	"github.com/NetSepio/erebrus/api/v1/authenticate"
	"github.com/NetSepio/erebrus/api/v1/client"
	"github.com/NetSepio/erebrus/api/v1/server"
	caddy "github.com/NetSepio/erebrus/api/v1/service"
	"github.com/NetSepio/erebrus/api/v1/status"

	"github.com/gin-gonic/gin"
)

// ApplyRoutes Setup API EndPoints
func ApplyRoutes(r *gin.RouterGroup) {
	v1 := r.Group("/v1.0")
	{
		client.ApplyRoutes(v1)
		server.ApplyRoutes(v1)
		status.ApplyRoutes(v1)
		authenticate.ApplyRoutes(v1)
		nodeConfig := os.Getenv("NODE_CONFIG")
		if nodeConfig == "STANDARD" || nodeConfig == "HPC" || nodeConfig == "NEXUS" {
			caddy.ApplyRoutes(v1)
			agents.ApplyRoutes(v1)
		}

	}
}
