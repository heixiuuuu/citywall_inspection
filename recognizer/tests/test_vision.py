import io
import json

import httpx
import pytest
from PIL import Image

from app.config import VisionConfig
from app.vision import VisionAnalyzer


def photo():
    output = io.BytesIO()
    Image.new("RGB", (20, 10)).save(output, "PNG")
    return output.getvalue()


def test_disabled_does_not_request():
    assert VisionAnalyzer(VisionConfig()).analyze(photo(), []).status == "DISABLED"


def test_visual_request_and_structured_response():
    def handler(request):
        body = json.loads(request.content)
        assert request.headers["Authorization"] == "Bearer test"
        assert body["messages"][1]["content"][1]["image_url"]["url"].startswith("data:image/jpeg;base64,")
        content = json.dumps({"description": "表面可见裂纹", "possible_causes": ["需复查"], "recommendations": ["补拍近景"]})
        return httpx.Response(200, json={"choices": [{"message": {"content": content}}]})
    analyzer = VisionAnalyzer(VisionConfig(base_url="https://vision.example/v1", model="vision", api_key="test"), httpx.MockTransport(handler))
    result = analyzer.analyze(photo(), ["CRACK"])
    assert result.status == "SUCCEEDED"
    assert result.description == "表面可见裂纹"


def test_invalid_result_does_not_fabricate_analysis():
    transport = httpx.MockTransport(lambda request: httpx.Response(200, json={"choices": []}))
    analyzer = VisionAnalyzer(VisionConfig(base_url="https://vision.example/v1", model="vision"), transport)
    result = analyzer.analyze(photo(), [])
    assert result.status == "FAILED"
    assert result.description == ""


@pytest.mark.parametrize("content", [None, json.dumps({"description": "  ", "possible_causes": [], "recommendations": []})])
def test_invalid_analysis_preserves_candidates_and_total_latency(monkeypatch, content):
    from types import SimpleNamespace
    from app.models.fixed import FixedDetector
    from app.schemas import BBox, Detection
    from app.service import RecognitionService

    clock = [0.0]
    monkeypatch.setattr("app.service.time", SimpleNamespace(monotonic=lambda: clock[0]))
    def handler(request):
        clock[0] = 2.5
        return httpx.Response(200, json={"choices": [{"message": {"content": content}}]})
    analyzer = VisionAnalyzer(VisionConfig(base_url="https://vision.example/v1", model="vision"), httpx.MockTransport(handler))
    candidate = Detection(detection_id="candidate", class_code="CRACK", confidence=.8, bbox=BBox(x1=1,y1=1,x2=10,y2=8))
    service = RecognitionService(FixedDetector([candidate]), "fixture", analyzer)
    result = service.recognize("attempt", photo(), ["png"])
    assert result.detections == [candidate]
    assert result.analysis.status == "FAILED" and result.analysis.error_code == "VLM_INVALID_RESPONSE"
    assert result.analysis.description == ""
    assert result.latency_ms == 2500
