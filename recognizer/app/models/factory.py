"""服务和离线评估共用的检测器装配入口。"""
from .fixed import FixedDetector
from .routed import ClassRoutedDetector
from .tiled import TiledDetector
from .yolo import YOLODetector


def build_detector(config):
    if config.detector_mode == "dev":
        return FixedDetector(detections=[])
    if config.detector_mode != "yolo":
        raise ValueError("detector_mode must be dev or yolo")
    parameters = {"device": config.device, "imgsz": config.imgsz, "nms_iou": config.nms_iou}
    detector = YOLODetector(config.weights_path, config.confidence_threshold, **parameters)
    if config.specialist_weights_path:
        codes = frozenset(config.specialist_class_codes)
        detector = ClassRoutedDetector(detector, YOLODetector(
            config.specialist_weights_path, config.confidence_threshold,
            required_class_codes=codes, **parameters,
        ), codes)
    if config.spalling_weights_path:
        spalling = YOLODetector(
            config.spalling_weights_path,
            config.spalling_confidence_threshold if config.spalling_confidence_threshold is not None else config.confidence_threshold,
            required_class_codes=frozenset({"SURFACE_SPALLING"}), **parameters,
        )
        if config.spalling_tile_size:
            spalling = TiledDetector(spalling, config.spalling_tile_size)
        detector = ClassRoutedDetector(detector, spalling, frozenset({"SURFACE_SPALLING"}))
    return detector
