import pytest

from app.api import routes
from app.models.fixed import FixedDetector


@pytest.fixture(autouse=True)
def isolate_external_models(monkeypatch):
    monkeypatch.setattr(routes._service, "_detector", FixedDetector(detections=[]))
    monkeypatch.setattr(routes._service, "_analyzer", None)
