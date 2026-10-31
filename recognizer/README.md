# 识别服务

识别端接收业务后端传入的文件，不直接接受客户端图片 URL。执行格式与尺寸校验、EXIF 方向归一、YOLO 检测和可选视觉语言模型分析，输出原图像素坐标的候选框。候选与分析均须人工复核。

- Python 3.12，基础依赖 requirements.txt，推理及数据转换依赖 requirements-yolo.txt。
- POST /internal/v1/recognitions：multipart 字段 image、attempt_id、image_id、request_id。
- GET /internal/v1/healthz：服务状态、检测模式、权重文件是否存在、视觉模型是否配置。文件存在不等于模型可加载。
- RECOGNIZER_CONFIG 指定 JSON 配置。RECOGNIZER_MODE 仅允许 yolo 或 dev；dev 返回空候选，仅用于接口联调。
- RECOGNIZER_WEIGHTS 覆盖权重路径；模型类别名称必须与业务类别码匹配，含未定义类别的权重拒绝加载。W_E 仅保留在来源训练数据中，不属于业务类别。
- RECOGNIZER_TOKEN 非空时要求 X-Internal-Token。
- VLM_BASE_URL、VLM_MODEL、VLM_API_KEY 配置兼容图像输入的模型服务；分析状态为 SUCCEEDED / DISABLED / FAILED。

本地启动：在 recognizer 目录执行 `python -m uvicorn app.main:app --host 127.0.0.1 --port 4100`。测试执行 `python -m pytest tests -q`，测试替身不会下载或调用真实模型。

检测采用串行模型访问，避免并发调用共享模型状态；耗时操作在线程池执行。模型缺失、加载失败、推理失败使用明确错误码。分析失败不会删除 YOLO 候选。

specialist_weights_path 和 specialist_class_codes 同时配置时，指定类别仅采用专项权重输出。spalling_weights_path 配置表面剥落权重，spalling_tile_size 配置局部裁剪边长，spalling_confidence_threshold 配置剥落候选阈值。各模型输出统一换算为原图坐标并重新编号，调用串行执行；任一模型不可用或坐标系不一致时识别失败。健康接口的 weights_available 检查全部已配置的权重文件。

当前 model_version 为 yolo_routed_msd_spalling_001，主权重、裂缝专项权重和剥落权重分别位于 data/yolo/weights/best.pt、specialist.pt 和 spalling.pt，成员哈希与类别分工保存在同目录 manifest.json。主模型和裂缝专项模型的输入尺寸分别为 640、960，置信度均为 0.35；剥落模型按 1280 像素分块，模型输入 640，置信度为 0.25。剥落候选误检、漏检较多，必须人工复核。

latency_ms 记录图像检查、检测及可选视觉分析的总耗时。视觉服务返回空内容或缺少有效描述时，辅助分析标记为 FAILED，已生成的检测候选继续返回。
