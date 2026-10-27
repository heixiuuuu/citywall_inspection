"""训练或恢复 YOLO，并按指定数据划分保存评估指标。"""
import argparse
import json
import shutil
import hashlib
from pathlib import Path


def json_scalar(value):
    if hasattr(value, "item"):
        return value.item()
    raise TypeError(f"unsupported JSON value: {type(value).__name__}")


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
    parser.add_argument("--optimizer", choices=("auto", "SGD", "Adam", "AdamW"), default="auto")
    parser.add_argument("--learning-rate", type=float)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--resume", type=Path)
    mode.add_argument("--evaluate", type=Path, help="evaluate existing weights without training")
    parser.add_argument("--deploy", action="store_true", help="copy evaluated weights to the inference directory")
    parser.add_argument("--evaluation-split", choices=("val", "test"), default="test")
    parser.add_argument("--evaluation-name")
    args = parser.parse_args()
    if args.learning_rate is not None:
        if not 0 < args.learning_rate < 1:
            parser.error("learning rate must be between zero and one")
        if args.optimizer == "auto" or args.resume or args.evaluate:
            parser.error("learning rate requires an explicit optimizer for a new training run")
    args.evaluation_name = args.evaluation_name or args.evaluation_split
    if not args.evaluation_name or any(not (c.isalnum() or c in "-_") for c in args.evaluation_name):
        raise ValueError("evaluation name must contain only letters, digits, hyphens and underscores")
    project = Path("data/yolo/runs").resolve()
    output = project / args.name
    if output.exists() and not args.resume and not args.evaluate:
        raise ValueError("run name already exists")
    if args.model.endswith(".pt"):
        Path(args.model).parent.mkdir(parents=True, exist_ok=True)
    if args.evaluate:
        weights = args.evaluate.resolve()
        output = weights.parent.parent
        data = args.data.resolve()
        image_size = args.imgsz
    elif args.resume:
        model = YOLO(str(args.resume.resolve()))
        model.train(resume=True, device=args.device, workers=0)
    else:
        model = YOLO(args.model)
        training = {"optimizer": args.optimizer}
        if args.learning_rate is not None:
            training["lr0"] = args.learning_rate
        model.train(data=str(args.data.resolve()), epochs=args.epochs, imgsz=args.imgsz, device=args.device,
                    batch=args.batch, workers=0, seed=42, deterministic=True, project=str(project), name=args.name,
                    **training)
    if not args.evaluate:
        output = Path(model.trainer.save_dir)
        weights = output / "weights" / "best.pt"
        data = Path(model.trainer.args.data).resolve()
        image_size = model.trainer.args.imgsz
    metrics = YOLO(str(weights)).val(data=str(data), split=args.evaluation_split,
                                  imgsz=image_size, batch=args.batch, device=args.device,
                                  project=str(output.resolve()), name=args.evaluation_name, exist_ok=True)
    with weights.open("rb") as stream:
        weights_hash = hashlib.file_digest(stream, "sha256").hexdigest()
    evaluation = {**metrics.results_dict, "per_class": metrics.summary(), "speed_ms": metrics.speed,
                  "dataset": str(data), "split": args.evaluation_split, "imgsz": image_size, "weights_sha256": weights_hash}
    filename = "evaluation.json" if args.evaluation_name == "test" else f"evaluation-{args.evaluation_name}.json"
    (output / filename).write_text(json.dumps(evaluation, indent=2, default=json_scalar), encoding="utf-8")
    if args.deploy:
        destination = Path("data/yolo/weights/best.pt")
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(weights, destination)


if __name__ == "__main__":
    main()
