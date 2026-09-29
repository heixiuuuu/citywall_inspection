-- =========================================================================
-- 古城墙病害巡检系统 · 数据库迁移 001
-- 说明：迁移必须是显式 SQL 文件并按版本顺序执行，不依赖启动时无审查地修改表。
-- 本文件覆盖 04 文档「迁移顺序」第一步：用户 / 角色 / 区段 / 点位。
-- 后续 002（任务与现场记录）、003（识别与人工结论）、004（问题与处置）、
-- 005（报告与幂等）按同风格追加。
-- 执行方式：mysql -u <user> -p <dbname> < migrations/001_init.sql
-- =========================================================================

-- 用户
CREATE TABLE IF NOT EXISTS users (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username      VARCHAR(64)     NULL COMMENT 'PC 账号，唯一',
    password_hash VARCHAR(255)    NULL COMMENT 'PC 密码哈希',
    wechat_openid VARCHAR(128)    NULL COMMENT '小程序微信身份，唯一',
    display_name  VARCHAR(64)     NOT NULL COMMENT '显示名（历史记录快照用）',
    status        VARCHAR(32)     NOT NULL DEFAULT 'ACTIVE' COMMENT 'ACTIVE/DISABLED',
    created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_users_username (username),
    UNIQUE KEY uk_users_wechat (wechat_openid)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci
  COMMENT = '业务用户：PC 账号或小程序身份，至少一种身份';

-- 用户角色
CREATE TABLE IF NOT EXISTS user_roles (
    user_id     BIGINT UNSIGNED NOT NULL,
    role_code   VARCHAR(32)     NOT NULL COMMENT 'INSPECTOR/ADMIN/REVIEWER',
    created_at  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (user_id, role_code)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci
  COMMENT = '管理员兼审核员时赋两种角色；提交人仍不可自审';

-- 城墙区段
CREATE TABLE IF NOT EXISTS wall_segments (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code        VARCHAR(32)     NOT NULL COMMENT '区段编码',
    name        VARCHAR(128)    NOT NULL,
    description VARCHAR(512)    NULL,
    active      TINYINT(1)      NOT NULL DEFAULT 1 COMMENT '停用不删除历史',
    created_at  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_wall_segments_code (code)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 巡检点位
CREATE TABLE IF NOT EXISTS inspection_points (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    segment_id    BIGINT UNSIGNED NOT NULL,
    code          VARCHAR(32)     NOT NULL,
    name          VARCHAR(128)    NOT NULL,
    latitude      DECIMAL(10, 7)  NOT NULL COMMENT 'WGS84 纬度',
    longitude     DECIMAL(10, 7)  NOT NULL COMMENT 'WGS84 经度',
    radius_m      DECIMAL(10, 2)  NOT NULL DEFAULT 0 COMMENT '打卡容差，仅提示阈值，异常交人工审核',
    location_hint VARCHAR(255)    NULL COMMENT '现场位置说明',
    active        TINYINT(1)      NOT NULL DEFAULT 1 COMMENT '停用不影响历史记录',
    created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_inspection_points_code (code),
    KEY idx_inspection_points_segment (segment_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
