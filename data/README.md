# 数据使用说明

业务数据由 MySQL 保存，现场照片和报告由受控存储保存。`yolo` 目录用于模型数据与权重，不作为现场巡检照片库。

## 模型数据入口

| 目录 | 内容 | train / val / test 图片数 | 使用方式 |
| --- | --- | --- | --- |
| yolo/dataset_msd_oriented | 方向归一后的 MSD 砖石病害数据 | 860 / 114 / 102 | 七类病害的基础训练与目标场景评估 |
| yolo/dataset_combined_oriented | MSD 与青砖标注的合并数据 | 1350 / 192 / 169 | 按 data.yaml 或分来源配置训练；W_E 不映射为业务病害 |
| yolo/dataset_crack_augmented | MSD 与砖石裂缝补充标注 | 2335 / 521 / 314 | 使用 crack_only/target.yaml 训练专项裂缝模型，目标场景验证和测试保持 MSD 划分 |
| yolo/dataset_cif_spalling | CIF 的独立 Spalling 标注 | 见 manifest.json | 表面剥落补充数据，主要为现代基础设施场景 |
| yolo/dataset_cif_spalling_background_tiles | 剥落局部裁剪与训练背景 | 见 manifest.json | 用于局部模型训练，裁剪图不是独立现场采样 |
| yolo/weights | 已部署权重与评估摘要 | 不适用 | manifest.json 定义成员、哈希、类别分工和推理参数 |

训练数据不随 Git 分发。各数据集中的 `manifest.json` 是来源、文件指纹、划分及隔离项的依据，`data.yaml` 或专项 YAML 是训练入口。不得通过移动验证或测试图片到训练目录来提高评估结果。

## 类别语义

| 业务编码 | 中文名称 | 标注来源 |
| --- | --- | --- |
| CRACK | 裂缝 | MSD、青砖及砖石裂缝补充数据 |
| DISCOLOURATION | 变色 | MSD |
| EFFLORESCENCE | 泛碱 | MSD，青砖 ALKALI |
| SURFACE_DISSOLUTION | 表面溶蚀 | MSD |
| MASONRY_LOSS | 砌体缺失 | MSD，青砖 MISS |
| MOSS | 苔藓 | MSD，青砖 MOSS |
| PLANT | 植物附着 | MSD |
| SURFACE_SPALLING | 表面剥落 | CIF Spalling |

青砖 W_E 保留来源编码，不自动合并为表面溶蚀或表面剥落。真菌、腐蚀等人工可选类别不代表已具备对应模型能力。不同来源图片的标注范围不同，专项单类入口用于避免把未标注的其他病害当作负样本。

## 业务数据与现场采样

预置区段、点位和账号用于本地演示。标题或位置说明中明确标注为公开数据集的任务与记录用于演示流程，其照片、定位缺失和结论不能作为真实现场成果。工作台展示全库当前状态，包含这些记录。

正式采集应登记实际区段、点位、采集位置与照片来源；定位失败保留缺失状态，通过补正补充证据，不填写推测坐标。通过记录按实际情况受理为问题或填写不立项原因。已提交照片、人工结论与审核意见由版本快照保留，数据整理不得直接改写历史证据。

现场训练样本应与公开数据分别登记来源，按墙段或采集场景分组划分，避免相近照片跨训练和留出集合；准确率只使用相应模型与数据指纹匹配的评估结果。
