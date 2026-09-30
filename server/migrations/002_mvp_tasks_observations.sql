-- =========================================================================
-- 古城墙病害巡检系统 · 数据库迁移 002（MVP：任务、现场记录、审核、审计）
-- 依据 04 文档迁移顺序第二步。先建任务与现场记录相关表，再建图片/识别（003）。
-- =========================================================================

-- 巡检任务
CREATE TABLE IF NOT EXISTS tasks (
    id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    title          VARCHAR(128)    NOT NULL,
    assignee_id    BIGINT UNSIGNED NOT NULL COMMENT '负责人（巡检员）',
    due_at         DATETIME(3)     NULL,
    description    VARCHAR(512)    NULL,
    status         VARCHAR(32)     NOT NULL DEFAULT 'DRAFT'
                   COMMENT 'DRAFT/ASSIGNED/IN_PROGRESS/PENDING_REVIEW/RETURNED/COMPLETED/CANCELLED',
    revision       BIGINT          NOT NULL DEFAULT 1 COMMENT '乐观锁版本',
    dispatched_at  DATETIME(3)     NULL,
    completed_at   DATETIME(3)     NULL,
    cancelled_at   DATETIME(3)     NULL,
    cancel_reason  VARCHAR(255)    NULL,
    created_at     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_tasks_assignee_status (assignee_id, status, due_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 任务点（草稿阶段登记 point_id，派发时填快照）
CREATE TABLE IF NOT EXISTS task_points (
    id                    BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    task_id               BIGINT UNSIGNED NOT NULL,
    point_id              BIGINT UNSIGNED NOT NULL,
    sequence_no           INT             NOT NULL,
    snapshot_name         VARCHAR(128)    NULL,
    snapshot_segment_id   BIGINT UNSIGNED NULL,
    snapshot_latitude     DECIMAL(10, 7)  NULL,
    snapshot_longitude    DECIMAL(10, 7)  NULL,
    snapshot_radius_m     DECIMAL(10, 2)  NULL,
    snapshot_location_hint VARCHAR(255)   NULL,
    created_at            DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_task_points_task_point (task_id, point_id),
    UNIQUE KEY uk_task_points_task_seq (task_id, sequence_no)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 现场记录
CREATE TABLE IF NOT EXISTS observations (
    id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    source              VARCHAR(32)     NOT NULL COMMENT 'TASK/VOLUNTARY',
    task_point_id       BIGINT UNSIGNED NULL,
    reporter_id         BIGINT UNSIGNED NOT NULL,
    segment_id          BIGINT UNSIGNED NULL,
    point_id            BIGINT UNSIGNED NULL,
    location_text       VARCHAR(512)    NOT NULL,
    latitude            DECIMAL(10, 7)  NULL,
    longitude           DECIMAL(10, 7)  NULL,
    accuracy_m          DECIMAL(10, 2)  NULL COMMENT '设备定位精度',
    distance_m          DECIMAL(10, 2)  NULL COMMENT '服务端按快照计算的距离',
    captured_at         DATETIME(3)     NULL COMMENT '客户端采集时间',
    inspection_outcome  VARCHAR(32)     NULL COMMENT 'NORMAL/SUSPECTED/UNDETERMINED/UNREACHABLE',
    status              VARCHAR(32)     NOT NULL DEFAULT 'DRAFT'
                        COMMENT 'DRAFT/SUBMITTED/RETURNED/ACCEPTED/REJECTED',
    revision            BIGINT          NOT NULL DEFAULT 1,
    submitted_at        DATETIME(3)     NULL,
    reviewed_at         DATETIME(3)     NULL,
    created_at          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_observations_task_point (task_point_id),
    KEY idx_observations_source_status (source, status, created_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 记录与图片关系（图片元数据在 003）
CREATE TABLE IF NOT EXISTS observation_images (
    observation_id BIGINT UNSIGNED NOT NULL,
    image_id       BIGINT UNSIGNED NOT NULL,
    image_role     VARCHAR(32)     NOT NULL COMMENT 'OVERVIEW/DETAIL',
    PRIMARY KEY (observation_id, image_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 提交快照
CREATE TABLE IF NOT EXISTS observation_submissions (
    id                       BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    observation_id           BIGINT UNSIGNED NOT NULL,
    submission_no            INT             NOT NULL,
    observation_revision     BIGINT          NOT NULL,
    conclusion_revision_id   BIGINT UNSIGNED NULL,
    location_snapshot_json   TEXT            NOT NULL,
    image_ids_snapshot_json  TEXT            NOT NULL,
    submitted_at             DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_obs_submission_no (observation_id, submission_no)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 审核决定
CREATE TABLE IF NOT EXISTS review_decisions (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    submission_id BIGINT UNSIGNED NOT NULL,
    reviewer_id   BIGINT UNSIGNED NOT NULL,
    decision      VARCHAR(32)     NOT NULL COMMENT 'ACCEPT/RETURN/REJECT',
    disposition   VARCHAR(255)    NULL COMMENT '受理说明（可选）',
    reason        VARCHAR(512)    NULL COMMENT '退回/驳回理由',
    created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_review_decisions_submission (submission_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 审计事件
CREATE TABLE IF NOT EXISTS audit_events (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    entity_type VARCHAR(32)     NOT NULL,
    entity_id   BIGINT UNSIGNED NOT NULL,
    action      VARCHAR(64)     NOT NULL,
    actor_id    BIGINT UNSIGNED NOT NULL,
    old_status  VARCHAR(32)     NULL,
    new_status  VARCHAR(32)     NULL,
    revision    BIGINT          NULL,
    reason      VARCHAR(512)    NULL,
    created_at  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_audit_entity (entity_type, entity_id),
    KEY idx_audit_actor (actor_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
