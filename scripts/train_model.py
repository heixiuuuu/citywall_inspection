"""训练 YOLO，并使用独立测试集保存真实评估指标。"""
import argparse
import json
import shutil
from pathlib import Path


def main():
    from ultralytics import YOLO
    parser = argparse.ArgumentParser()
    parser.add_argument("--data", type=Path, default=Path("data/yolo/dataset/data.yaml"))
    parser.add_argument("--model", default="data/yolo/pretrained/yolov8n.pt")
    parser.add_argument("--epochs", type=int, default=50)
    parser.add_argument("--imgsz", type=int, default=640)
    parser.add_argument("--device", default="cpu")
    parser.add_argument("--name", default="baseline")
    parser.add_argument("--batch", type=int, default=8)
    args = parser.parse_args()
    output = Path("data/yolo/runs") / args.name
    if output.exists():
        raise ValueError("run name already exists")
    if args.model.endswith(".pt"):
        Path(args.model).parent.mkdir(parents=True, exist_ok=True)
    YOLO(args.model).train(data=str(args.data.resolve()), epochs=args.epochs, imgsz=args.imgsz, device=args.device,
                          batch=args.batch, workers=0, seed=42, deterministic=True, project="data/yolo/runs", name=args.name)
    weights = output / "weights" / "best.pt"
    metrics = YOLO(str(weights)).val(data=str(args.data.resolve()), split="test", imgsz=args.imgsz, device=args.device,
                                  project=str(output), name="test")
    (output / "evaluation.json").write_text(json.dumps(metrics.results_dict, indent=2), encoding="utf-8")
    destination = Path("data/yolo/weights/best.pt")
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(weights, destination)


if __name__ == "__main__":
    main()
