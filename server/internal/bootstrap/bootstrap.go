// Package bootstrap 装配配置、基础设施与业务模块，构建可启动的 HTTP 服务。
package bootstrap

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"citywall/server/internal/api"
	"citywall/server/internal/authn"
	"citywall/server/internal/config"
	"citywall/server/internal/infrastructure/database"
	"citywall/server/internal/infrastructure/recognizer"
	"citywall/server/internal/infrastructure/storage"
	"citywall/server/internal/module/audit"
	"citywall/server/internal/module/identity"
	"citywall/server/internal/module/observation"
	"citywall/server/internal/module/site"
	"citywall/server/internal/module/task"
	"citywall/server/internal/router"
)

// App 是装配完成的运行时对象。
type App struct {
	cfg     *config.Config
	dbReady bool
	engine  *gin.Engine
	storage storage.Storage
}

// New 按配置装配全部组件。
// dev 阶段 MySQL 未就绪且 database.required=false 时降级启动（/readyz 返回 503）。
func New(cfg *config.Config) (*App, error) {
	app := &App{cfg: cfg}

	st, err := storage.New(cfg.Storage)
	if err != nil {
		return nil, err
	}
	app.storage = st

	db, err := database.Open(cfg.Database)
	if err != nil {
		if cfg.Database.Required {
			return nil, err
		}
		log.Printf("[bootstrap] mysql not ready, degrade start: %v", err)
	} else {
		app.dbReady = true
	}

	tm := authn.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.TokenTTL())
	rc := recognizer.New(cfg.Recognizer)

	// 仓储。
	identityRepo := identity.NewGORMRepository(db)
	siteRepo := site.NewGORMRepository(db)
	taskRepo := task.NewGORMRepository(db)
	obsRepo := observation.NewGORMRepository(db)

	// Service 装配（依赖方向：task → 记录状态接口；observation → 任务点/生命周期接口；site → 点位快照读取）。
	// 审计记录器：MySQL 就绪时落库，否则 no-op（dev 降级，审计失败不阻断业务）。
	recorder := audit.NewRecorder(db)
	identitySvc := identity.NewService(identityRepo, tm)
	siteSvc := site.NewService(siteRepo)
	obsSvc := observation.NewService(obsRepo, st, rc, taskRepo, taskRepo, taskRepo, recorder)
	taskSvc := task.NewService(taskRepo, obsSvc, func(ctx context.Context, pointID int64) (task.PointSnapshot, error) {
		p, err := siteRepo.GetPoint(ctx, pointID)
		if err != nil {
			return task.PointSnapshot{}, err
		}
		return task.PointSnapshot{
			PointID:   p.ID,
			Name:      p.Name,
			SegmentID: p.SegmentID,
			Latitude:  p.Latitude,
			Longitude: p.Longitude,
			RadiusM:   p.RadiusM,
			Hint:      p.LocationHint,
		}, nil
	}, recorder)

	h := router.Handlers{
		Identity:    api.NewIdentityHandler(identitySvc),
		Site:        api.NewSiteHandler(siteSvc),
		Task:        api.NewTaskHandler(taskSvc),
		Observation: api.NewObservationHandler(obsSvc),
	}
	app.engine = router.Router(h, tm, cfg.Upload.RequestIDHeader)
	registerReadyz(app)
	return app, nil
}

// Router 返回 HTTP Handler。
func (a *App) Router() http.Handler { return a.engine }

// Close 释放资源。
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
