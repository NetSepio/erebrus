package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/NetSepio/erebrus/api"
	"github.com/NetSepio/erebrus/api/v1/agents"
	"github.com/NetSepio/erebrus/core"
	grpc "github.com/NetSepio/erebrus/gRPC"
	"github.com/NetSepio/erebrus/p2p"
	"github.com/NetSepio/erebrus/util"
	"github.com/NetSepio/erebrus/util/pkg/auth"
	"github.com/NetSepio/erebrus/util/pkg/node"
	"github.com/gin-contrib/static"

	helmet "github.com/danielkov/gin-helmet"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
)

var wg sync.WaitGroup

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stderr)
	log.SetLevel(log.DebugLevel)
	node.Init()

	// Get Hostname for updating Log StandardFields
	HostName, err := os.Hostname()
	if err != nil {
		log.Infof("Error in getting the Hostname: %v", err)
	} else {
		util.StandardFields = log.Fields{
			"hostname": HostName,
			"appname":  "Erebrus",
		}
	}
	// Check if loading environment variables from .env file is required
	if os.Getenv("LOAD_CONFIG_FILE") == "" {

		// Load environment variables from .env file
		err = godotenv.Load()
		if err != nil {
			log.WithFields(util.StandardFields).Fatalf("Error in reading the config file: %v", err)

		}
	}

	core.GetIPInfo()

	auth.Init()
	agents.EnsureDockerAndCaddy()

}

func RungRPCServer() {
	grpc_server := grpc.Initialize()

	port := os.Getenv("GRPC_PORT")

	log.WithFields(util.StandardFields).Info("Starting gRPC Api, Listening on Port :", port)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		wg.Done()
		log.Fatal("Unable to listen on port", port)

	}

	//Server GRPC
	if err := grpc_server.Serve(listener); err != nil {
		wg.Done()
		log.Fatal("Faied to create GRPC server!")

	}
	wg.Done()

}

func main() {
	log.WithFields(util.StandardFields).Infof("Starting NetSepio - Erebrus Version: %s", util.Version)

	// check directories or create it
	if !util.DirectoryExists(filepath.Join(os.Getenv("WG_CONF_DIR"))) {
		err := os.Mkdir(os.Getenv("WG_CONF_DIR"), 0755)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
				"dir": filepath.Join(os.Getenv("WG_CONF_DIR")),
			}).Fatal("failed to create wireguard configuration directory")
		}
	}

	// check directories or create it
	fmt.Println(os.Getenv("WG_CLIENTS_DIR"))
	if !util.DirectoryExists(filepath.Join(os.Getenv("WG_CLIENTS_DIR"))) {
		err := os.Mkdir(os.Getenv("WG_CLIENTS_DIR"), 0755)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
				"dir": filepath.Join(os.Getenv("WG_CLIENTS_DIR")),
			}).Fatal("failed to create wireguard clients directory")
		}
	}

	// check if server.json exists otherwise create it with default values
	if !util.FileExists(filepath.Join(os.Getenv("WG_CONF_DIR"), "server.json")) {
		_, err := core.ReadServer()
		if err != nil {
			log.WithFields(util.StandardFields).Fatal("server.json does not exist and unable to open")
		}
	}

	if os.Getenv("RUNTYPE") == "debug" {
		// set gin release debug
		gin.SetMode(gin.DebugMode)
	} else {
		// set gin release mode
		gin.SetMode(gin.ReleaseMode)
		// disable console color
		gin.DisableConsoleColor()
		// log level info
		log.SetLevel(log.InfoLevel)
	}

	// dump wg config file
	err := core.UpdateServerConfigWg()
	util.CheckError("Error while creating WireGuard config file: ", err)
	// Call the function to generate the wallet address and store it in the global variable

	core.LoadNodeDetails()

	go p2p.Init()
	//running updater
	wg.Add(1)

	if os.Getenv("GRPC_PORT") != "" {
		//Add gRPC routine to wait group
		wg.Add(1)
		//run gRPC server
		go RungRPCServer()
	}

	if os.Getenv("HTTP_PORT") != "" {
		// creates a gin router with default middleware: logger and recovery (crash-free) middleware
		ginApp := gin.Default()
		// cors middleware
		config := cors.DefaultConfig()
		config.AllowOrigins = []string{os.Getenv("GATEWAY_DOMAIN")}
		ginApp.Use(cors.New(config))

		// protection middleware
		ginApp.Use(helmet.Default())

		// add cache storage to gin ginApp
		ginApp.Use(func(ctx *gin.Context) {
			ctx.Set("cache", cache.New(60*time.Minute, 10*time.Minute))
			ctx.Next()
		})
		// serve static files
		ginApp.Use(static.Serve("/", static.LocalFile("./webapp", false)))
		//ginApp.Use(static.Serve("/docs", static.LocalFile("./docs", false)))

		/*opt := openapimiddleware.RedocOpts{SpecURL: "/docs/swagger.yml"}
		handler := openapimiddleware.Redoc(opt, nil)
		*/
		//ginApp.Static("docs", "./docs")
		// no route redirect to frontend app
		ginApp.NoRoute(func(c *gin.Context) {
			c.JSON(404, gin.H{"status": 404, "message": "Invalid Endpoint Request"})
		})

		// Apply API Routes
		api.ApplyRoutes(ginApp)
		err = ginApp.Run(fmt.Sprintf("%s:%s", os.Getenv("SERVER"), os.Getenv("HTTP_PORT")))
		util.CheckError("Failed to Start HTTP Server: ", err)
	}
	//wait untill all servers are stopped
	wg.Wait()

}
