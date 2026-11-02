# 部署模型评估

`scripts/evaluate_operating_point.py` 评估部署配置在固定阈值下的病害候选及人工复核负担。检测器由识别服务共用的 `build_detector` 构造，调用同一 `RecognitionService`，覆盖主模型、专项类别路由、剥落分块、EXIF 方向归一及整数坐标输出。任一成员失败即中止评估，不将失败图片作为零候选计分。

## 数据与标注范围

数据入口为转换后的 YOLO YAML，`val` 或 `test` 指向 `images` 下的目录，对应标签位于 `labels` 下的相同相对路径。每张图片都必须有标签文件；空文件表示在声明的标注类别中没有目标，缺少文件则报错。标签编号按 YAML 的语义名称解析，不以模型编号对齐数据编号。

`--annotated-classes` 必填，声明这些类别在每张所选图片上均已完整标注。类别没有在某张图中出现时，该类候选仍计误检；声明范围外的模型候选只记为未评估候选。若实际标签包含声明范围外的类别，脚本报错，不能用缩小范围隐藏已有标注。YAML 中的类别字典或观测到的框数量不能单独证明标注完整性。

MSD 覆盖 CRACK、DISCOLOURATION、EFFLORESCENCE、SURFACE_DISSOLUTION、MASONRY_LOSS、MOSS、PLANT 七类。FUNGUS、CORROSION、ARTISTIC 和 SURFACE_SPALLING 不在该标注范围。CIF 剥落入口只声明 SURFACE_SPALLING。不同来源的标注范围不一致时，使用分来源的 YAML 分别评价，不能将整份合并数据统一视为完整多类标注。

## 运行

使用 Python 3.12，安装 `recognizer/requirements-yolo.txt`。在仓库根目录执行：

```powershell
$MsdClasses = @('CRACK', 'DISCOLOURATION', 'EFFLORESCENCE', 'SURFACE_DISSOLUTION', 'MASONRY_LOSS', 'MOSS', 'PLANT')
C:\environment\citywall-python312\Scripts\python.exe scripts/evaluate_operating_point.py --data data/yolo/dataset_msd_oriented/data.yaml --split val --device cpu --annotated-classes $MsdClasses --output C:\code\tmp\deployment-val.json
C:\environment\citywall-python312\Scripts\python.exe scripts/evaluate_operating_point.py --data data/yolo/dataset_msd_oriented/data.yaml --weights data/yolo/weights/best.pt --split val --device cpu --annotated-classes $MsdClasses --output C:\code\tmp\primary-val.json
C:\environment\citywall-python312\Scripts\python.exe scripts/evaluate_operating_point.py --data data/yolo/dataset_cif_spalling/data.yaml --split val --device cpu --annotated-classes SURFACE_SPALLING --output C:\code\tmp\deployment-cif-val.json
```

| 参数 | 含义 |
| --- | --- |
| --config | 默认 RECOGNIZER_CONFIG 指定的文件或 conf/recognizer.json；读取与服务相同的环境变量覆盖，最终有效配置写入结果 |
| --weights | 单权重对照，关闭专项模型、类别路由及剥落分块 |
| --split | val 或 test，默认 val；冻结方案后才使用 test |
| --confidence | 覆盖主模型和专项模型置信度；剥落继续使用其独立配置值 |
| --device / --nms-iou / --imgsz | 覆盖所有成员的对应参数；未指定时保留配置、权重及推理库默认值 |
| --match-iou | 匹配 IoU 下限，默认 0.5 |
| --warmup | 首张图片预热次数，默认 1；预热不进入计分或稳定耗时统计，0 表示首张计入冷启动耗时 |
| --output | 输出 JSON，不覆盖已有文件 |

评估拒绝 dev 固定检测器。`mode=deployment` 表示从组合配置构造检测器；是否与运行服务完全一致，应核对有效配置、成员哈希及 runtime_members，而不能只看 model_version。单权重输出使用 single_weight 标识，部署权重和配置文件不会被修改。

## 匹配与输出

预测使用服务返回的整数像素框与四位小数置信度，标注框由归一化坐标换算为方向归一后的浮点像素框。按预测置信度降序，贪心选择同类别中 IoU 最大的未匹配标注，IoU 至少达到下限即为 TP，否则为 FP，未匹配标注为 FN。同分预测保持服务输出顺序；相同 IoU 选择较大的标注索引。每个框只能匹配一次。

输出包括：

- aggregate 与 per_class：微平均总指标和逐类 TP、FP、FN、Precision、Recall、F1，包含声明范围内的零样本类别。
- per_image：图片与标签哈希、方向与尺寸、标注框、完整服务候选、匹配索引与 IoU、漏检索引、逐类计数、未评估候选及耗时。索引均从 0 开始，预测索引对应完整 predictions 数组。
- candidate_burden：全部类别的候选总量、每图平均和最大数量，以及未评估候选的分类计数。未标注的剥落候选也计入复核负担。
- configuration、weights、runtime_members：有效识别配置、各权重 SHA-256、实际模型类别、输入尺寸、阈值、NMS 与设备等参数。
- dataset_sha256：按类别映射、排序后的图片相对路径、图片与标签哈希计算；另存 YAML、来源清单和推理/评估源码文件指纹。权重在结束时复查，期间变化则报错。
- environment 与 timing：Python、推理库、Pillow、PyTorch、系统、CPU、CUDA、线程数，以及预热、平均、中位数、P95、最小和最大耗时。

detector_ms 是同步调用检测器的墙钟耗时，包含解码、模型路由、分块和后处理；recognition_without_vlm_ms 另包含服务的图像校验。两者均不包含文件读取、HTTP、业务服务、视觉语言模型或人工操作，不能作为完整上传识别请求的耗时。CUDA 推理输出转换为服务坐标与分数后才停止计时。

这些结果是固定运行阈值下的 Precision、Recall、F1，不提供标准 mAP50 或 mAP50–95。标准单模型 mAP 使用 `train_model.py --evaluate`，并明确其原始模型验证口径；不能将其当作经过路由与分块后的组合指标。小样本、公开数据及未标注类别的候选数量均不足以证明现场可靠性。
