package caddy

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NetSepio/erebrus/api/v1/middleware"
	"github.com/NetSepio/erebrus/api/v1/service/util"
	"github.com/NetSepio/erebrus/core"
	"github.com/NetSepio/erebrus/model"
	"github.com/gin-gonic/gin"
)

// ApplyRoutes applies router to gin Router
func ApplyRoutes(r *gin.RouterGroup) {

	g := r.Group("/caddy")
	{
		g.POST("", addServices)
		g.GET("", getServicess)
		g.GET(":name", getServices)
		g.DELETE(":name", deleteServices)
	}
}

var resp map[string]interface{}

// addTunnel adds new tunnel config
func addServices(c *gin.Context) {
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

		// check validity of Services name and port
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
			//create a Services struct object
			var data model.Service
			data.Name = name
			data.Type = os.Getenv("NODE_TYPE")
			data.Port = port
			data.Domain = os.Getenv("DOMAIN")
			data.IpAddress = ipAddress
			data.CreatedAt = time.Now().UTC().Format(time.RFC3339)

			//to add Services config
			err := middleware.AddWebServices(data)
			if err != nil {
				resp = util.Message(500, "Server error, Try after some time or Contact Admin..."+err.Error())
				c.JSON(http.StatusInternalServerError, resp)
				break
			} else {
				resp = util.MessageService(200, data)
				c.JSON(http.StatusOK, resp)
				break
			}
		}
	}
}

// getServicess gets all Services config
func getServicess(c *gin.Context) {
	services, err := middleware.ReadWebServices()
	if err != nil {
		resp = util.Message(500, "Server error, Try after some time or Contact Admin...")
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	c.JSON(http.StatusOK, services)
}

// getServices get specific Services config
func getServices(c *gin.Context) {
	//get parameter
	name := c.Param("name")

	//read Services config
	Services, err := middleware.ReadWebService(name)
	if err != nil {
		resp = util.Message(500, "Server error, Try after some time or Contact Admin...")
		c.JSON(http.StatusInternalServerError, resp)
	}

	//check if Services exists
	if Services.Name == "" {
		resp = util.Message(404, "Services Doesn't Exists")
		c.JSON(http.StatusNotFound, resp)
	} else {
		port, err := strconv.Atoi(Services.Port)
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
				Services.Status = status
				resp = util.MessageService(200, *Services)
				c.JSON(http.StatusOK, resp)
			}
		}
	}
}

func deleteServices(c *gin.Context) {
	//get parameter
	name := c.Param("name")

	//read Services config
	Services, err := middleware.ReadWebService(name)
	if err != nil {
		resp = util.Message(500, "Server error, Try after some time or Contact Admin...")
		c.JSON(http.StatusInternalServerError, resp)
	}

	//check if Services exists
	if Services.Name == "" {
		resp = util.Message(400, "Services Doesn't Exists")
		c.JSON(http.StatusBadRequest, resp)
	} else {
		//delete Services config
		err = middleware.DeleteWebServices(name)
		if err != nil {
			resp = util.Message(500, "Server error, Try after some time or Contact Admin...")
			c.JSON(http.StatusInternalServerError, resp)
		} else {
			resp = util.Message(200, "Deleted Services: "+name)
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
