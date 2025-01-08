package caddy

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NetSepio/erebrus/api/v1/middleware"
	"github.com/NetSepio/erebrus/api/v1/tunnel/util"
	"github.com/NetSepio/erebrus/core"
	"github.com/NetSepio/erebrus/model"
	"github.com/gin-gonic/gin"
)

// ApplyRoutes applies router to gin Router
func ApplyRoutes(r *gin.RouterGroup) {

	g := r.Group("/caddy")
	{
		g.POST("", addTunnel)
		g.GET("", getTunnels)
		g.GET(":name", getTunnel)
		g.DELETE(":name", deleteTunnel)
	}
}

var resp map[string]interface{}

// addTunnel adds new tunnel config
func addTunnel(c *gin.Context) {
	//post form parameters
	name := strings.ToLower(c.PostForm("name"))
	ipAddress := c.PostForm("ip_address")

	port := c.PostForm("port")

	// convert port string to int
	portInt, err := strconv.Atoi(port)
	if err != nil {
		resp = util.Message(400, "Invalid Port")
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	// port allocation
	// max, _ := strconv.Atoi(os.Getenv("CADDY_UPPER_RANGE"))
	// min, _ := strconv.Atoi(os.Getenv("CADDY_LOWER_RANGE"))

	for {
		// port, err := core.GetPort(max, min)
		// if err != nil {
		// 	panic(err)
		// }

		// check validity of tunnel name and port
		value, msg, err := middleware.IsValidWeb(name, portInt)

		if err != nil {
			resp = util.Message(500, "Server error, Try after some time or Contact Admin..."+err.Error())
			c.JSON(http.StatusOK, resp)
			break
		} else if value == -1 {
			if msg == "Port Already in use" {
				continue
			}

			resp = util.Message(404, msg)
			c.JSON(http.StatusBadRequest, resp)
			break
		} else if value == 1 {
			//create a tunnel struct object
			var data model.Tunnel
			data.Name = name
			data.Port = port
			data.Domain = os.Getenv("CADDY_DOMAIN")
			data.IpAddress = ipAddress
			data.CreatedAt = time.Now().UTC().Format(time.RFC3339)

			//to add tunnel config
			err := middleware.AddWebTunnel(data)
			if err != nil {
				resp = util.Message(500, "Server error, Try after some time or Contact Admin...")
				c.JSON(http.StatusInternalServerError, resp)
				break
			} else {
				resp = util.MessageTunnel(200, data)
				c.JSON(http.StatusOK, resp)
				break
			}
		}
	}
}

// getTunnels gets all tunnel config
func getTunnels(c *gin.Context) {
	//read all tunnel config
	tunnels, err := middleware.ReadWebTunnels()
	if err != nil {
		resp = util.Message(500, "Server error, Try after some time or Contact Admin...")
		c.JSON(http.StatusInternalServerError, resp)
	} else {
		resp = util.MessageTunnels(200, tunnels.Tunnels)
		c.JSON(http.StatusOK, resp)
	}
}

// getTunnel get specific tunnel config
func getTunnel(c *gin.Context) {
	//get parameter
	name := c.Param("name")

	//read tunnel config
	tunnel, err := middleware.ReadWebTunnel(name)
	if err != nil {
		resp = util.Message(500, "Server error, Try after some time or Contact Admin...")
		c.JSON(http.StatusInternalServerError, resp)
	}

	//check if tunnel exists
	if tunnel.Name == "" {
		resp = util.Message(404, "Tunnel Doesn't Exists")
		c.JSON(http.StatusNotFound, resp)
	} else {
		port, err := strconv.Atoi(tunnel.Port)
		if err != nil {
			util.LogError("string conv error: ", err)
			resp = util.Message(500, "Server error, Try after some time or Contact Admin...")
			c.JSON(http.StatusInternalServerError, resp)
		} else {
			status, err := core.ScanPort(port)
			if err != nil {
				resp = util.Message(500, "Server error, Try after some time or Contact Admin...")
				c.JSON(http.StatusInternalServerError, resp)
			} else {
				tunnel.Status = status
				resp = util.MessageTunnel(200, *tunnel)
				c.JSON(http.StatusOK, resp)
			}
		}
	}
}

func deleteTunnel(c *gin.Context) {
	//get parameter
	name := c.Param("name")

	//read tunnel config
	tunnel, err := middleware.ReadWebTunnel(name)
	if err != nil {
		resp = util.Message(500, "Server error, Try after some time or Contact Admin...")
		c.JSON(http.StatusInternalServerError, resp)
	}

	//check if tunnel exists
	if tunnel.Name == "" {
		resp = util.Message(400, "Tunnel Doesn't Exists")
		c.JSON(http.StatusBadRequest, resp)
	} else {
		//delete tunnel config
		err = middleware.DeleteWebTunnel(name)
		if err != nil {
			resp = util.Message(500, "Server error, Try after some time or Contact Admin...")
			c.JSON(http.StatusInternalServerError, resp)
		} else {
			resp = util.Message(200, "Deleted Tunnel: "+name)
			c.JSON(http.StatusOK, resp)
		}
	}

}

// func MiddlewareForCaddy(c *gin.Context) {

// 	//check if NODE_CONFIG is set to standard or hpc

// 	if strings.ToLower(os.Getenv("NODE_CONFIG")) != "standard" && strings.ToLower(os.Getenv("NODE_CONFIG")) != "hpc" {
// 		util.LogError("NODE_CONFIG not allowed", nil)
// 		c.JSON(http.StatusNotAcceptable, resp)
// 		os.Exit(1)
// 	}
// }

// NodeConfigMiddleware checks if NODE_CONFIG is set to "standard" or "hpc".
func MiddlewareForCaddy() gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeConfig := os.Getenv("NODE_CONFIG")

		if nodeConfig != "standard" && nodeConfig != "hpc" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid NODE_CONFIG value. It must be 'standard' or 'hpc'.",
			})
			c.Abort() // Stop further processing of the request
			return
		}

		// Pass to the next middleware/handler
		c.Next()
	}
}
