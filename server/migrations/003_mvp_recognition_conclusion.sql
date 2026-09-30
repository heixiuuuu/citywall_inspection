-- =========================================================================
-- 古城墙病害巡检系统 · 数据库迁移 003（MVP：图片、识别尝试、人工结论）
-- 依据 04 文档迁移顺序第三步。依赖 001（users 等）与 002（observations）。
-- =========================================================================

-- 图片元数据（文件存受控目录，库中只存引用）
CREATE TABLE IF NOT EXISTS images (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    storage_key VARCHAR(255)    NOT NULL COMMENT '服务端生成的相对路径 key',
    sha256      CHAR(64)        NOT NULL,
    media_type  VARCHAR(64)     NOT NULL,
    byte_size   BIGINT          NOT NULL,
    width       INT             NOT NULL DEFAULT 0,
    height      INT             NOT NULL DEFAULT 0,
    captured_at DATETIME(3)     NULL,
    uploaded_by BIGINT UNSIGNED NOT NULL,
    created_at  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_images_storage_key (storage_key)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 识别尝试（每次重试新建，不覆盖历史）
CREATE TABLE IF NOT EXISTS recognition_attempts (
    id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    image_id       BIGINT UNSIGNED NOT NULL,
    attempt_no     INT             NOT NULL,
    status         VARCHAR(32)     NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING/SUCCEEDED/FAILED',
    model_version  VARCHAR(128)    NULL,
    schema_version VARCHAR(32)     NULL,
    latency_ms     INT             NULL,
    error_code     VARCHAR(64)     NULL COMMENT 'RECOGNITION_TIMEOUT 等',
    started_at     DATETIME(3)     NOT NULL,
    finished_at    DATETIME(3)     NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_attempts_image_no (image_id, attempt_no)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 候选检测（原图像素坐标，左上角原点）
CREATE TABLE IF NOT EXISTS detections (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    attempt_id    BIGINT UNSIGNED NOT NULL,
    detection_key VARCHAR(64)     NOT NULL,
    class_code    VARCHAR(64)     NOT NULL,
    confidence    DECIMAL(6, 5)   NOT NULL,
    x1            INT             NOT NULL,
    y1            INT             NOT NULL,
    x2            INT             NOT NULL,
    y2            INT             NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_detections_attempt_key (attempt_id, detection_key)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 人工结论版本（revision_no 为不可变历史序号）
CREATE TABLE IF NOT EXISTS conclusion_revisions (
    id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    observation_id BIGINT UNSIGNED NOT NULL,
    revision_no    INT             NOT NULL,
    outcome        VARCHAR(32)     NOT NULL,
    description    VARCHAR(512)    NULL,
    priority_code  VARCHAR(32)     NULL,
    author_id      BIGINT UNSIGNED NOT NULL,
    created_at     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_conclusion_obs_no (observation_id, revision_no)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 人工结论项（确认/修改/删除候选或补录）
CREATE TABLE IF NOT EXISTS conclusion_items (
    id                      BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    conclusion_revision_id  BIGINT UNSIGNED NOT NULL,
    image_id                BIGINT UNSIGNED NOT NULL,
    source_detection_id     BIGINT UNSIGNED NULL,
    decision                VARCHAR(32)     NOT NULL COMMENT 'CONFIRM/MODIFY/REJECT/ADD',
    final_class_code        VARCHAR(64)     NULL,
    final_box_json          TEXT            NULL,
    description             VARCHAR(512)    NULL,
    PRIMARY KEY (id),
    KEY idx_conclusion_items_revision (conclusion_revision_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
