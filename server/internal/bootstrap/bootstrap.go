// Package bootstrap 装配配置、基础设施与业务模块，构建可启动的 HTTP 服务。
package bootstrap

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/api"
	"citywall/server/internal/config"
	"citywall/server/internal/infrastructure/database"
	"citywall/server/internal/infrastructure/recognizer"
	"citywall/server/internal/infrastructure/storage"
	"citywall/server/internal/module/observation"
	"citywall/server/internal/module/task"
	"citywall/server/internal/router"
)

// App 是装配完成的运行时对象。
type App struct {
	cfg        *config.Config
	dbReady    bool
	engine     *gin.Engine
	storage    storage.Storage
	recognizer recognizer.Recognizer
}

// New 按配置装配全部组件。
// dev 阶段 MySQL 未就绪且 database.required=false 时降级启动（/readyz 返回 503）。
func New(cfg *config.Config) (*App, error) {
	app := &App{cfg: cfg}

	// 受控存储目录。
	st, err := storage.New(cfg.Storage)
	if err != nil {
		return nil, err
	}
	app.storage = st

	// 识别服务客户端。
	app.recognizer = recognizer.New(cfg.Recognizer)

	// 数据库：required=true 时连接失败直接返回错误。
	db, err := database.Open(cfg.Database)
	if err != nil {
		if cfg.Database.Required {
			return nil, err
		}
		log.Printf("[bootstrap] mysql not ready, degrade start: %v", err)
	} else {
		app.dbReady = true
	}

	// 业务 Service（数据库未就绪时也可装配，Handler 骨架阶段不实际访问库）。
	taskSvc := task.NewService(task.NewGORMRepository(db))
	obsSvc := observation.NewService(observation.NewGORMRepository(db))

	h := router.Handlers{
		Task:        api.NewTaskHandler(taskSvc),
		Observation: api.NewObservationHandler(obsSvc),
	}
	app.engine = router.Router(h, cfg.Upload.RequestIDHeader)
	registerReadyz(app)
	return app, nil
}

// Router 返回 HTTP Handler。
func (a *App) Router() http.Handler { return a.engine }

// Close 释放资源（后续接入连接池关闭等）。
func (a *App) Close() error { return nil }

// DBReady 报告数据库就绪状态。
func (a *App) DBReady() bool { return a.dbReady }

func registerReadyz(a *App) {
	a.engine.GET("/readyz", func(c *gin.Context) {
		if a.dbReady {
			c.JSON(http.StatusOK, gin.H{"status": "ready"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "reason": "mysql not connected"})
	})
}
