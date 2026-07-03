CREATE DATABASE IF NOT EXISTS reward_db DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE reward_db;

CREATE TABLE IF NOT EXISTS projects (
    id          BIGINT       NOT NULL AUTO_INCREMENT,
    name        VARCHAR(128) NOT NULL,
    description TEXT,
    created_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS payment_configs (
    id              BIGINT       NOT NULL AUTO_INCREMENT,
    pay_address     VARCHAR(128) NOT NULL,
    voucher_type    VARCHAR(64)  NOT NULL,
    payment_account VARCHAR(128) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_payment_configs_pay_address (pay_address)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS finance_docs (
    doc_id              VARCHAR(64)  NOT NULL,
    project_id          BIGINT       NOT NULL,
    description         TEXT,
    application_detail  JSON         NOT NULL,
    creator             VARCHAR(128) NOT NULL,
    status              VARCHAR(32)  NOT NULL DEFAULT 'DRAFT',
    remark              VARCHAR(512) NOT NULL DEFAULT '',
    created_at          DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at          DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (doc_id),
    KEY idx_finance_docs_project_id (project_id),
    KEY idx_finance_docs_status (status),
    CONSTRAINT fk_finance_docs_project FOREIGN KEY (project_id) REFERENCES projects (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO payment_configs (pay_address, voucher_type, payment_account) VALUES
    ('0xabc123wallet001', 'crypto', 'ACC-CRYPTO-001'),
    ('0xdef456wallet002', 'crypto', 'ACC-CRYPTO-002'),
    ('bank-usd-main-001', 'cash', 'ACC-CASH-001')
ON DUPLICATE KEY UPDATE
    voucher_type = VALUES(voucher_type),
    payment_account = VALUES(payment_account);
