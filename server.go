package main

import (
	"fmt"
	"net/http"

	"github.com/mmq88/quickcerts/api"
	cfg "github.com/mmq88/quickcerts/configs"
	"github.com/mmq88/quickcerts/data"
	"github.com/mmq88/quickcerts/middleware"
	"github.com/mmq88/quickcerts/utils"

	_ "github.com/mmq88/quickcerts/docs"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var (
	runtimeCode string
	router      *gin.Engine
)

func init() {
	utils.InitLogger()
	utils.Record(logrus.InfoLevel, "Initializing the server...")

	gin.SetMode(gin.ReleaseMode)
	router = gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.AccessLogger())

	if cfg.SERVER_CONFIG.USE_RUNTIME_CODE {
		var err error
		runtimeCode, err = utils.GenerateRunTimeCode()

		if err != nil {
			utils.Record(logrus.FatalLevel, "Failed to generate the run time code. Due to: "+err.Error())
		}

		runtimeCodeMsg := color.HiCyanString("[USE_RUNTIME_CODE] is enabled, Runtime code: ")
		runtimeCodeMsg += color.HiMagentaString("%s", runtimeCode)
		utils.Record(logrus.InfoLevel, runtimeCodeMsg)
	}
}

// @title QuickCertS API
// @version 1.0
// @description This is the API for QuickCertS.
// @contact.name MMQ
// @contact.email mail@mmq.dev
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:33333
// @basePath /api/v1
// @schemes http https
// @accept json
// @produce json
func main() {
	err := data.ConnectDB()

	if err != nil {
		utils.Record(logrus.FatalLevel, err.Error())
	}
	utils.Record(logrus.InfoLevel, "Successfully connected the database.")

	defer func() {
		err := data.DisconnectDB()
		if err != nil {
			if err.Error() == "currently not connecting the database" {
				utils.Record(logrus.WarnLevel, "Currently not connecting the database.")
				return
			}
			utils.Record(logrus.FatalLevel, err.Error())
		}

		utils.Record(logrus.InfoLevel, "Successfully disconnected the database.")
	}()

	err = data.ConnectRDB()
	if err != nil {
		utils.Record(logrus.FatalLevel, err.Error())
	}
	utils.Record(logrus.InfoLevel, "Successfully connected the redis database.")

	defer func() {
		err := data.DisconnectRDB()
		if err != nil {
			if err.Error() == "currently not connecting the redis database" {
				utils.Record(logrus.WarnLevel, "Currently not connecting the redis database.")
				return
			}
			utils.Record(logrus.FatalLevel, err.Error())
		}

		utils.Record(logrus.InfoLevel, "Successfully disconnected the redis database.")
	}()

	registerRoutes()

	if !cfg.SERVER_CONFIG.USE_TLS {
		run(router)

	} else {
		if cfg.SERVER_CONFIG.TLS_CERT_PATH == "" || cfg.SERVER_CONFIG.TLS_KEY_PATH == "" {
			utils.Record(logrus.FatalLevel, "TLS_CERT_PATH or TLS_KEY_PATH is empty. Please fill in the configs file.")
		}
		runTLS(router)
	}
}

func registerRoutes() {
	registerRoutesForDocs()

	rootGroup := router.Group("/api/v1")
	registerRoutesForAdmin(rootGroup)
	registerRoutesForClient(rootGroup)
}

func registerRoutesForDocs() {
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

func registerRoutesForAdmin(rootGroup *gin.RouterGroup) {
	snGroup := rootGroup.Group("/sn")

	snGroup.POST("/create",
		middleware.IPAddressAuth(),
		middleware.AdminAccessAuth(runtimeCode),
		api.CreateSN,
	)
	snGroup.POST("/generate",
		middleware.IPAddressAuth(),
		middleware.AdminAccessAuth(runtimeCode),
		api.GenerateSN,
	)
	snGroup.POST("/update",
		middleware.IPAddressAuth(),
		middleware.AdminAccessAuth(runtimeCode),
		api.UpdateCertNote,
	)
	snGroup.GET("/get-available",
		middleware.IPAddressAuth(),
		middleware.AdminAccessAuth(runtimeCode),
		api.GetAvaliableSN,
	)
	snGroup.GET("/get-all",
		middleware.IPAddressAuth(),
		middleware.AdminAccessAuth(runtimeCode),
		api.GetAllRecords,
	)
}

func registerRoutesForClient(rootGroup *gin.RouterGroup) {
	applyGroup := rootGroup.Group("/apply")

	applyGroup.POST("/cert", middleware.ClientAccessAuth(), api.ApplyCertificate)
	applyGroup.POST("/temp-permit", middleware.ClientAccessAuth(), api.ApplyTemporaryPermit)
}

func run(router *gin.Engine) {
	httpServer := &http.Server{
		Addr:        cfg.SERVER_CONFIG.PORT,
		Handler:     router,
		IdleTimeout: cfg.SERVER_CONFIG.KEEP_ALIVE_TIMEOUT,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.Record(logrus.FatalLevel, "Failed to start the server. Due to: "+err.Error())
		}
	}()

	runningMsg := fmt.Sprintf("Server is running in %s mode. listening on port: %s",
		color.HiCyanString("http"), color.HiCyanString("%s", cfg.SERVER_CONFIG.PORT[1:]))
	utils.Record(logrus.InfoLevel, runningMsg)

	utils.WaitForShutdown(httpServer)
}

func runTLS(router *gin.Engine) {
	httpsServer := &http.Server{
		Addr:        cfg.SERVER_CONFIG.TLS_PORT,
		Handler:     router,
		IdleTimeout: cfg.SERVER_CONFIG.KEEP_ALIVE_TIMEOUT,
	}

	httpsServer.SetKeepAlivesEnabled(false)

	go func() {
		if err := httpsServer.ListenAndServeTLS(
			cfg.SERVER_CONFIG.TLS_CERT_PATH,
			cfg.SERVER_CONFIG.TLS_KEY_PATH,
		); err != nil && err != http.ErrServerClosed {
			utils.Record(logrus.FatalLevel, "Failed to start the server. Due to: "+err.Error())
		}
	}()

	runningMsg := fmt.Sprintf("Server is running in %s mode. listening on port: %s",
		color.HiMagentaString("https"), color.HiMagentaString("%s", cfg.SERVER_CONFIG.TLS_PORT[1:]))
	utils.Record(logrus.InfoLevel, runningMsg)

	utils.WaitForShutdown(httpsServer)
}
