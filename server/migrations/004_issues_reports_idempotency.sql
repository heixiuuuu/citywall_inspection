-- =========================================================================
-- 古城墙病害巡检系统 · 数据库迁移 004（问题跟踪、报告、幂等记录）
-- 依据 04 文档第 5/6 节。依赖 001~003（users/observations/images 等）。
-- =========================================================================

-- 病害问题（与任务生命周期独立；来源经 issue_observations 追溯）
CREATE TABLE IF NOT EXISTS issues (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    title           VARCHAR(128)    NOT NULL,
    segment_id      BIGINT UNSIGNED NULL,
    point_id        BIGINT UNSIGNED NULL,
    location_text   VARCHAR(512)    NOT NULL,
    description     VARCHAR(1024)   NULL,
    priority_code   VARCHAR(32)     NOT NULL DEFAULT 'WATCH'
                    COMMENT '人工业务优先级 WATCH/PROCESS/URGENT',
    priority_reason VARCHAR(512)    NULL,
    status          VARCHAR(32)     NOT NULL DEFAULT 'UNASSIGNED'
                    COMMENT 'UNASSIGNED/OPEN/IN_PROGRESS/MONITORING/PENDING_VERIFICATION/CLOSED',
    assignee_id     BIGINT UNSIGNED NULL,
    revision        BIGINT          NOT NULL DEFAULT 1,
    opened_at       DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    closed_at       DATETIME(3)     NULL,
    close_reason    VARCHAR(512)    NULL,
    created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_issues_status_assignee (status, assignee_id, opened_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 问题与现场记录关联（一次审核可关联多个 Issue；一个问题可关联多个记录）
CREATE TABLE IF NOT EXISTS issue_observations (
    issue_id            BIGINT UNSIGNED NOT NULL,
    observation_id      BIGINT UNSIGNED NOT NULL,
    review_decision_id  BIGINT UNSIGNED NOT NULL,
    conclusion_item_id  BIGINT UNSIGNED NULL,
    link_reason         VARCHAR(512)    NULL,
    linked_by           BIGINT UNSIGNED NOT NULL,
    linked_at           DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (issue_id, observation_id),
    KEY idx_issue_observations_obs (observation_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 问题处置动作（建议/进展/观察/申请核验/核验通过/退回/重开，只追加不改历史）
CREATE TABLE IF NOT EXISTS issue_actions (
    id                   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    issue_id             BIGINT UNSIGNED NOT NULL,
    actor_id             BIGINT UNSIGNED NOT NULL,
    action_type          VARCHAR(32)     NOT NULL
                         COMMENT 'ADVICE/PROGRESS/OBSERVE/REQUEST_VERIFY/VERIFY_PASS/VERIFY_RETURN/REOPEN',
    content              VARCHAR(1024)   NOT NULL,
    planned_follow_up_at DATETIME(3)     NULL,
    created_at           DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_issue_actions_issue (issue_id, created_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 问题处置动作附件（照片证据；读取时校验 Issue 权限）
CREATE TABLE IF NOT EXISTS issue_action_images (
    issue_action_id BIGINT UNSIGNED NOT NULL,
    image_id        BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (issue_action_id, image_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 报告（生成时保存数据快照；PDF 与快照对应同一时刻）
CREATE TABLE IF NOT EXISTS reports (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    scope_type    VARCHAR(32)     NOT NULL COMMENT 'TASK/DATE_RANGE',
    task_id       BIGINT UNSIGNED NULL,
    range_start   DATETIME(3)     NULL,
    range_end     DATETIME(3)     NULL,
    generated_by  BIGINT UNSIGNED NOT NULL,
    snapshot_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    snapshot_json MEDIUMTEXT      NOT NULL,
    file_key      VARCHAR(255)    NULL,
    status        VARCHAR(32)     NOT NULL DEFAULT 'GENERATING' COMMENT 'GENERATING/READY/FAILED',
    error_code    VARCHAR(64)     NULL,
    created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    finished_at   DATETIME(3)     NULL,
    PRIMARY KEY (id),
    KEY idx_reports_scope (scope_type, task_id),
    KEY idx_reports_range (range_start, range_end)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 幂等记录（派发/提交/审核/关闭/上传等重复会造成副作用的动作）
CREATE TABLE IF NOT EXISTS idempotency_records (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    actor_id      BIGINT UNSIGNED NOT NULL,
    operation     VARCHAR(64)     NOT NULL,
    idem_key      VARCHAR(128)    NOT NULL,
    request_hash  CHAR(64)        NOT NULL,
    response_ref  VARCHAR(255)    NULL,
    status        VARCHAR(32)     NOT NULL DEFAULT 'COMPLETED' COMMENT 'COMPLETED/CONFLICT',
    expires_at    DATETIME(3)     NULL,
    created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_idempotency_actor_op_key (actor_id, operation, idem_key)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
