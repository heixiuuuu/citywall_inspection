import io
import json

import httpx
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
