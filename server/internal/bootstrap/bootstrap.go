// Package bootstrap 装配配置、基础设施与业务模块，构建可启动的 HTTP 服务。
package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"citywall/server/internal/module/idempotency"
	"citywall/server/internal/module/identity"
	"citywall/server/internal/module/issue"
	"citywall/server/internal/module/observation"
	"citywall/server/internal/module/report"
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
	issueRepo := issue.NewGORMRepository(db)

	// Service 装配（依赖方向：task → 记录状态接口；observation → 任务点/生命周期接口；site → 点位快照读取）。
	// 审计记录器：MySQL 就绪时落库，否则 no-op（dev 降级，审计失败不阻断业务）。
	recorder := audit.NewRecorder(db)
	identitySvc := identity.NewService(identityRepo, tm, recorder, identity.WechatConfig{AppID: cfg.Auth.WechatAppID, Secret: cfg.Auth.WechatAppSecret})
	siteSvc := site.NewService(siteRepo)
	obsSvc := observation.NewService(obsRepo, st, rc, taskRepo, taskRepo, recorder)
	taskSvc := task.NewService(taskRepo, obsSvc, func(ctx context.Context, pointID int64) (task.PointSnapshot, error) {
		p, err := siteRepo.GetPoint(ctx, pointID)
		if err != nil {
			return task.PointSnapshot{}, err
		}
		if !p.Active {
			return task.PointSnapshot{}, task.ErrPointNotFound
		}
		segment, err := siteRepo.GetSegment(ctx, p.SegmentID)
		if err != nil || !segment.Active {
			return task.PointSnapshot{}, task.ErrPointNotFound
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
	// 问题模块：受理数据访问使用 observation 服务（obsSvc 实现 issue.ObservationAccess）。
	var issueSvc issue.Service
	// 落库幂等（派发/提交/审核/关闭/上传等重复操作去重；DB 就绪时装配）。
	var idemSvc idempotency.Service
	var idemStore observation.IdempotencyStore
	if app.dbReady {
		issueSvc = issue.NewService(issueRepo, obsSvc, &issueImageStore{st: st}, recorder)
		idemSvc = idempotency.NewService(idempotency.NewRepository(db))
		idemStore, _ = idemSvc.(observation.IdempotencyStore)
	}
	// 图片上传幂等：observation 服务支持落库幂等存储时注入（接口可选）。
	if s, ok := obsSvc.(interface {
		SetIdempotencyStore(st observation.IdempotencyStore)
	}); ok {
		s.SetIdempotencyStore(idemStore)
	}
	// 报告模块：快照查询 + PDF 生成 + 受控存储。
	var reportSvc report.Service
	if app.dbReady {
		reportRepo := report.NewRepository(db)
		reportSrc := report.NewSnapshotSource(db)
		reportSvc = report.NewService(reportRepo, reportSrc, st, cfg.Report, cfg.Storage.TempDir, recorder)
	}

	h := router.Handlers{
		Identity:    api.NewIdentityHandler(identitySvc),
		Site:        api.NewSiteHandler(siteSvc),
		Task:        api.NewTaskHandler(taskSvc),
		Observation: api.NewObservationHandler(obsSvc, cfg.Upload.IdempotencyKeyHeader),
	}
	if app.dbReady {
		h.Issue = api.NewIssueHandler(issueSvc)
		h.Report = api.NewReportHandler(reportSvc)
	}
	// 审计查询：仅数据库就绪时可用（否则返回 DB_UNAVAILABLE）。
	if app.dbReady {
		h.Audit = api.NewAuditHandler(audit.NewQueryService(audit.NewGORMRepository(db)))
	} else {
		h.Audit = api.NewAuditHandler(audit.NewQueryService(nil))
	}
	app.engine = router.Router(h, tm, cfg.Upload.RequestIDHeader, idemSvc, cfg.Upload.IdempotencyKeyHeader)
	registerReadyz(app)
	return app, nil
}

// Router 返回 HTTP Handler。
func (a *App) Router() http.Handler { return a.engine }

// Close 释放资源。
func (a *App) Close() error { return nil }

// DBReady 报告数据库就绪状态。
func (a *App) DBReady() bool { return a.dbReady }

// issueImageStore 复用受控图片目录。
type issueImageStore struct {
	st storage.Storage
}

func (a *issueImageStore) Save(data []byte, mediaType string) (string, string, error) {
	key, err := a.st.SaveImage(data, mediaType)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(data)
	return key, hex.EncodeToString(sum[:]), nil
}

func (a *issueImageStore) Discard(key string) {
	if err := a.st.RemoveImage(key); err != nil {
		log.Printf("remove uncommitted issue image: %v", err)
	}
}

func registerReadyz(a *App) {
	a.engine.GET("/readyz", func(c *gin.Context) {
		if a.dbReady {
			c.JSON(http.StatusOK, gin.H{"status": "ready"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "reason": "mysql not connected"})
	})
}
