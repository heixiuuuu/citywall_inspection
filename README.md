# 古城墙病害巡检系统

系统由巡检端、管理端、业务后端、识别端组成。巡检员采集照片和位置，YOLO 提供候选区域，视觉语言模型提供现象描述与复查建议；人工结论经审核后进入问题台账和报告。

## 目录

| 目录 | 职责 |
| --- | --- |
| miniapp | uni-app / Vue 3 巡检端，可构建 H5、微信小程序及 App 资源 |
| android | 加载 H5 资源的 Android WebView 工程 |
| web | Vue 3 / TypeScript / Element Plus 管理端 |
| server | Go / Gin / GORM 业务服务及 SQL 迁移 |
| recognizer | FastAPI 图像校验、YOLO 推理、视觉语言模型适配 |
| conf | 本地默认配置，密钥通过环境变量覆盖 |
| scripts | 数据转换、训练和业务验收脚本 |
| docs | 当前架构、接口补充与验收边界 |

## Docker 启动

安装并启动 Docker Desktop，在仓库根目录执行完整启动命令（本地视觉模型需要 NVIDIA GPU）：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/start.ps1
```

首次启动会创建 .env 并在缺少视觉模型时下载模型；已有镜像可使用 `scripts/start.ps1 -SkipBuild`。管理端：[http://localhost:5173](http://localhost:5173)。业务 API：[http://localhost:8080/healthz](http://localhost:8080/healthz)。MySQL 仅绑定本机，数据卷名为 citywall_mysql-data，图片与报告卷名为 citywall_server-data。端口冲突时在 .env 修改 MYSQL_PORT、SERVER_PORT 或 WEB_PORT。

| 账号 | 密码 | 角色 |
| --- | --- | --- |
| admin | Admin123 | 管理员、审核员 |
| reviewer | Review123 | 审核员 |
| inspector | Insp123 | 巡检员 |

这些是本地演示账号。部署到外部网络前需更换账号密码、JWT_SECRET、数据库密码及内部服务令牌。

停止完整服务执行 `docker compose -f docker-compose.yml -f docker-compose.vision.yml down`，保留数据卷。删除数据卷会清除业务数据。

## 巡检端

```powershell
cd miniapp
npm ci
npm run build:h5
npm run build:mp-weixin
npm run build:app
```

账号登录页可填写业务服务地址：真机填写 `http://电脑局域网IP:8080/api/v1`，Android 模拟器填写 `http://10.0.2.2:8080/api/v1`。也可在构建时设置 VITE_API_BASE。定位需要系统权限；浏览器定位要求 HTTPS 或 localhost。

`build:app` 生成 uni-app 资源，不等于 APK。`android` 工程加载 `miniapp/dist/build/h5`，APK 构建要求 JDK、Android SDK 和 Gradle，详见 [巡检端说明](miniapp/README.md)。

## 识别与模型实验

当前检测采用类别分工：主权重位于 `data/yolo/weights/best.pt`，裂缝、泛碱和苔藓的专项权重位于 `data/yolo/weights/specialist.pt`，表面剥落权重位于 `data/yolo/weights/spalling.pt`，成员哈希保存在同目录 manifest.json。剥落模型训练场景以现代基础设施为主，候选误检和漏检较多，必须人工复核。缺失任一所需权重时返回 MODEL_UNAVAILABLE，保留失败尝试，巡检员可继续人工填写。

Python 使用 3.12。建议环境建在 `C:\environment\citywall-python312`：

```powershell
python -m venv C:\environment\citywall-python312
C:\environment\citywall-python312\Scripts\python.exe -m pip install -r recognizer/requirements-yolo.txt
C:\environment\citywall-python312\Scripts\python.exe scripts/prepare_dataset.py --source C:\a\school\毕设 --output data/yolo/dataset_msd_oriented
C:\environment\citywall-python312\Scripts\python.exe scripts/train_model.py --data data/yolo/dataset_msd_oriented/data.yaml --name msd_oriented_baseline --device cpu --evaluation-split val
```

数据转换输出 train / val / test、data.yaml 和 manifest.json。训练输出 best.pt、训练记录和独立测试集 evaluation.json（含逐类指标）；--deploy 仅复制权重到主模型路径，不更新专项权重。发布时须按 conf/recognizer.json 准备全部成员，并同步 model_version 与权重清单中的哈希和类别分工。CUDA 环境使用 --device 0，断点继续可传 --resume data/yolo/runs/运行名/weights/last.pt。模型选择应依据验证集，独立测试集仅用于最终评估。训练与推理代码存在不代表模型准确率已通过验收，参见 [验收边界](docs/acceptance.md)。

标注尺寸与编码尺寸一致时保留原坐标；只有 EXIF 方向归一后的尺寸与标注完全一致时才采用归一图片，不通过缩放猜测标注。输出 JPEG 不携带方向标签。当前完整转换得到 1,076 张图片，其中 134 张进行了方向归一；原有 942 张的图片字节、标签和划分保持一致。输出目录已有 data.yaml 时拒绝覆盖。

已转换 MSD 后，可使用资料目录中的青砖标注补充训练，需安装 7-Zip：

本机 NVIDIA GPU 环境使用 Python 3.12 和已验证的 CUDA 13.0 PyTorch 包；首次建立环境执行：

```powershell
python -m venv C:\environment\citywall-cuda312
C:\environment\citywall-cuda312\Scripts\python.exe -m pip install torch==2.14.1 torchvision==0.29.1 --index-url https://download.pytorch.org/whl/cu130
C:\environment\citywall-cuda312\Scripts\python.exe -m pip install -r recognizer/requirements-yolo.txt
```

```powershell
C:\environment\citywall-python312\Scripts\python.exe scripts/prepare_combined_dataset.py --base data/yolo/dataset_msd_oriented --output data/yolo/dataset_combined_oriented
C:\environment\citywall-cuda312\Scripts\python.exe scripts/train_model.py --data data/yolo/dataset_combined_oriented/data.yaml --name combined_oriented_n960 --imgsz 960 --epochs 80 --batch 8 --device 0 --evaluation-split val
```

合并数据保留 MSD 的全部划分；青砖数据保留原训练集，将原验证集按图片哈希分为验证与测试集。同像素图片归为一组，有标注冲突的组整体隔离，来源、数量及隔离项写入 manifest.json。W_E 保留来源标签，不自动视为 MSD 的表面溶蚀；含该未定义类别的权重不能直接部署。

msd-target.yaml 只使用 MSD 图片和七个已标注业务类别，保持原划分和文件内容，可用于原场景微调。自定义学习率必须同时指定优化器，避免 auto 优化器覆盖学习率；断点继续使用检查点中的训练参数。

```powershell
C:\environment\citywall-cuda312\Scripts\python.exe scripts/train_model.py --data data/yolo/dataset_combined_oriented/msd-target.yaml --model data/yolo/runs/combined_oriented_n960/weights/best.pt --name msd_oriented_finetune_n960 --epochs 40 --imgsz 960 --device 0 --batch 8 --optimizer AdamW --learning-rate 0.0002 --evaluation-split val
```

data.yaml 提供合并入口，msd.yaml 和 bluebrick.yaml 提供分来源入口。评估现有权重可使用 --evaluate 权重路径、--evaluation-split val 或 test，以及 --evaluation-name 自定义名称；输出对应命名的指标 JSON 和图表，记录权重哈希。新权重的类别顺序从其名称元数据读取，须属于业务类别白名单，不能依靠数字编号猜测病害。

部署前还需在相同验证集、设备和推理参数下比较固定置信度的候选数量。下面的脚本针对转换后的 images/labels 目录，默认使用权重的输入尺寸、0.35 置信度和 0.7 NMS IoU；按置信度顺序匹配同语义类别、IoU 至少 0.5 的未匹配标注，输出 TP、FP、FN、逐类及逐图结果。输出记录数据集与权重指纹，拒绝覆盖已有文件。该指标用于比较人工复核负担，不代替标准 mAP 或现场效果评估。

```powershell
C:\environment\citywall-python312\Scripts\python.exe scripts/evaluate_operating_point.py --data data/yolo/dataset_msd_oriented/data.yaml --weights data/yolo/weights/best.pt --output C:\code\tmp\operating-val.json
```

--confidence、--nms-iou、--imgsz 可指定运行参数。阈值选择使用验证集，确定模型与参数后再使用 --split test 进行最终评估；更换数据配置时须重新核对类别语义和划分。

资料中的 Brick and Stone(annotated).zip 可作为裂缝补充标注。转换脚本仅读取 brick 和 stone 两套数据，隔离不明类别、像素相同但标注冲突的图片，以及与已有 MSD 重复的图片。相同原图名或像素内容相连的图片整体保留在同一划分中，留出集优先；这不能证明不同照片来自不同墙段。边界舍入误差在归一化坐标 0.0001 以内时裁至图像边界并计数，明显越界拒绝处理。

```powershell
C:\environment\citywall-python312\Scripts\python.exe scripts/prepare_crack_dataset.py --base data/yolo/dataset_msd_oriented --output data/yolo/dataset_crack_augmented
C:\environment\citywall-cuda312\Scripts\python.exe scripts/train_model.py --data data/yolo/dataset_crack_augmented/crack_only/target.yaml --model data/yolo/runs/msd_finetune_n960/weights/best.pt --name crack_expert_n960 --epochs 40 --imgsz 960 --device 0 --batch 8 --optimizer AdamW --learning-rate 0.0002 --evaluation-split val
```

训练入口 crack_only 只包含 CRACK 标签，避免将补充图片中未标注的其他病害作为多类模型的负样本。图片通过硬链接复用父目录的图像文件，标签独立；图像修改须另存副本。target.yaml 使用全部训练图片，仍以 MSD 的 114 张验证图选模，102 张测试图用于最终评估；data.yaml 提供全部来源的验证和测试入口。原数据目录不修改，父目录保留语义重编号后的来源标注及清单。训练结果须经目标场景复验后再发布。

表面剥落候选训练集可通过 `python scripts/prepare_cif_spalling.py` 准备。该脚本从 [CIF 数据集](https://huggingface.co/datasets/ibm-research/cif-dataset) 的固定版本读取独立 Spalling 标注，按来源训练、验证划分生成单类别框和负样本，并保存文件哈希与来源清单。该数据许可为 CDLA-Permissive-2.0，场景主要为现代基础设施；对古城墙砖石的识别效果需结合项目资料核验。

执行 `python scripts/prepare_spalling_tiles.py` 可从该数据集生成局部裁剪数据，保留原图的训练与验证划分，用于训练较小的病害目标。局部裁剪的验证结果不能直接代表整图识别效果。

裁剪时可传入 `--background-tiles 1`，从训练正样本图中额外选择不包含已标注剥落框的局部背景，帮助学习易混淆的表面纹理；验证集不增加此类裁剪。

视觉语言模型使用兼容 Chat Completions 的图像输入接口。使用外部服务时，配置 .env 中的 VLM_BASE_URL（包含 /v1 等服务前缀）、VLM_MODEL 和 VLM_API_KEY，并仅使用基础配置启动：`docker compose -f docker-compose.yml up -d --build --wait`。支持不要求 API Key 的本地服务。输出保存于识别尝试中，可在管理端审核页面与报告查看。未配置显示 DISABLED，失败显示 FAILED；不会生成代替模型的分析文本。

本地 NVIDIA GPU 可使用 Docker 内的 Ollama 和 Qwen3-VL 2B：

```powershell
docker compose -f docker-compose.yml -f docker-compose.vision.yml up -d vision
docker compose -f docker-compose.yml -f docker-compose.vision.yml exec vision ollama pull qwen3-vl:2b-instruct
docker compose -f docker-compose.yml -f docker-compose.vision.yml up -d --build --wait
```

模型保存在 citywall_vision-data 卷中，识别端通过容器网络调用服务，宿主接口仅绑定 127.0.0.1:11434。使用 `docker-compose.vision.yml` 时识别端固定连接该容器，不使用 .env 中的 VLM_BASE_URL 和 VLM_MODEL。下载需要代理时，通过 VLM_DOWNLOAD_PROXY 指定容器可访问的 HTTPS 代理地址，例如 http://host.docker.internal:7897。

## 验证

完整软件验收使用隔离的 Docker 项目，执行单元检查、前端构建、数据库集成测试及业务闭环，结束后自动删除测试容器与数据卷：

```powershell
C:\environment\citywall-python312\Scripts\python.exe -m pip install -r scripts/requirements-test.txt
.\scripts\verify.ps1
```

可通过 -Python、-MySQLPort、-APIPort、-WebPort 指定本机环境。-SkipBuildChecks 仅执行数据库集成和业务联调，适合构建检查已经通过后的部署复验。测试 PDF 保存在 C:\code\tmp 的独立目录。

分项检查：

```powershell
cd server
go test ./...
go vet ./...
cd ../web
npm ci
npm run build
cd ../recognizer
C:\environment\citywall-python312\Scripts\python.exe -m pytest tests -q
```

业务验收脚本会写入数据，由 verify.ps1 在隔离环境创建所需账号并统一执行。单独执行脚本时必须准备相同的测试数据；e2e_evidence.py 需要额外的 inspector_other 巡检账号，密码为 Insp123。CITY_WALL_API_BASE 指定测试服务，默认 http://127.0.0.1:8080/api/v1。

真实模型联调使用 `scripts/verify_recognition.py --image 图片路径 --require-vision`，需安装测试依赖、部署专用权重并启用视觉模型。脚本创建明确标记的公开数据集演示任务，核验候选、辅助分析、人工复核、任务完成及 PDF 下载，将证据 JSON 和报告保存到 C:\code\tmp\citywall-model-e2e。该用例不代表现场病害判断或模型质量评估。

接口定义见 [OpenAPI](server/api/openapi.yaml)，架构与边界见 [系统架构](docs/architecture.md)。
