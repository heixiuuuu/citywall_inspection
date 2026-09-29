"""FastAPI 入口：创建应用、挂载路由、管理模型生命周期。"""
from __future__ import annotations

from fastapi import FastAPI

from .api.routes import router as internal_router
from .config import settings

app = FastAPI(
    title=settings.app.name,
    version="0.1.0",
    description="古城墙病害识别服务（内部契约 /internal/v1）",
)


@app.get("/healthz", tags=["internal"])
async def healthz() -> dict:
    return {"status": "ok", "model_version": settings.recognition.model_version}


app.include_router(internal_router)


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "app.main:app",
        host=settings.server.get("host", "0.0.0.0"),
        port=int(settings.server.get("port", 8000)),
        reload=settings.app.env == "dev",
    )
