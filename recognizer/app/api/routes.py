"""内部识别 API 路由。

POST /internal/v1/recognitions
  multipart/form-data：image 文件 + attempt_id、image_id、request_id 表单字段。
Go 只发送自己已校验并持久化的原图；本服务不持有业务用户身份。

契约（与 Go 客户端一致）：
  - 成功（200）：RecognitionResponse；
  - 失败（4xx/5xx）：RecognitionError（error.code / error.message / request_id），
    绝不以 200 携带错误体，避免 Go 端误判为成功。
"""
from __future__ import annotations

from fastapi import APIRouter, File, Form, Request, UploadFile
from fastapi.responses import JSONResponse

from ..config import settings
from ..schemas import RecognitionError, RecognitionResponse
from ..service import RecognitionService
from ..models.fixed import FixedDetector
from ..images import ImageValidationError

router = APIRouter(prefix="/internal/v1", tags=["internal"])

_service = RecognitionService(
    detector=FixedDetector(detections=[]),
    model_version=settings.recognition.model_version,
)


def _check_internal_auth(request: Request) -> bool:
    if not settings.internal.require_token:
        return True
    token = request.headers.get("X-Internal-Token", "")
    return token == settings.internal.token


def _error(status: int, code: str, message: str, request_id: str) -> JSONResponse:
    body = RecognitionError(
        error={"code": code, "message": message}, request_id=request_id
    )
    return JSONResponse(status_code=status, content=body.model_dump())


@router.post("/recognitions", response_model=RecognitionResponse)
async def create_recognition(
    request: Request,
    image: UploadFile = File(...),
    attempt_id: str = Form(...),
    image_id: str = Form(...),
    request_id: str = Form(default=""),
) -> RecognitionResponse | JSONResponse:
    if not _check_internal_auth(request):
        return _error(401, "UNAUTHORIZED", "invalid internal token", request_id)

    data = await image.read()
    if len(data) > settings.recognition.max_image_bytes:
        return _error(413, "IMAGE_TOO_LARGE", "image exceeds size limit", request_id)
    if len(data) == 0:
        return _error(422, "INVALID_IMAGE", "empty image", request_id)

    try:
        return _service.recognize(
            attempt_id,
            data,
            allowed_formats=settings.recognition.allowed_formats,
        )
    except ImageValidationError as e:
        return _error(422, "INVALID_IMAGE", e.message, request_id)
    except Exception:
        return _error(500, "INFERENCE_FAILED", "recognition failed", request_id)


@router.get("/healthz")
async def healthz() -> dict:
    return {"status": "ok", "model_version": settings.recognition.model_version}
