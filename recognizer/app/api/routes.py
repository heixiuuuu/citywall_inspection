"""内部识别 API 路由。

POST /internal/v1/recognitions
  multipart/form-data：image 文件 + attempt_id、image_id、request_id 表单字段。
Go 只发送自己已校验并持久化的原图；本服务不持有业务用户身份。
"""
from __future__ import annotations

from fastapi import APIRouter, File, Form, HTTPException, Request, UploadFile

from ..config import settings
from ..schemas import RecognitionError, RecognitionResponse
from ..service import RecognitionService
from ..models.fixed import FixedDetector

router = APIRouter(prefix="/internal/v1", tags=["internal"])

_service = RecognitionService(
    detector=FixedDetector(
        image_width=1600, image_height=1200, detections=[]
    ),
    model_version=settings.recognition.model_version,
)


def _check_internal_auth(request: Request) -> None:
    if not settings.internal.require_token:
        return
    token = request.headers.get("X-Internal-Token", "")
    if token != settings.internal.token:
        raise HTTPException(status_code=401, detail="invalid internal token")


@router.post("/recognitions", response_model=RecognitionResponse | RecognitionError)
async def create_recognition(
    request: Request,
    image: UploadFile = File(...),
    attempt_id: str = Form(...),
    image_id: str = Form(...),
    request_id: str = Form(default=""),
) -> RecognitionResponse | RecognitionError:
    _check_internal_auth(request)

    data = await image.read()
    if len(data) > settings.recognition.max_image_bytes:
        return RecognitionError(
            error={"code": "IMAGE_TOO_LARGE", "message": "image exceeds size limit"},
            request_id=request_id,
        )

    try:
        return _service.recognize(attempt_id, data)
    except HTTPException:
        raise
    except Exception:
        return RecognitionError(
            error={"code": "INFERENCE_FAILED", "message": "recognition failed"},
            request_id=request_id,
        )


@router.get("/healthz")
async def healthz() -> dict:
    return {"status": "ok", "model_version": settings.recognition.model_version}
