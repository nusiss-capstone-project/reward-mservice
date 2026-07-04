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
    unit            VARCHAR(32)  NOT NULL,
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
    UNIQUE KEY uk_finance_docs_project_id (project_id),
    KEY idx_finance_docs_status (status),
    CONSTRAINT fk_finance_docs_project FOREIGN KEY (project_id) REFERENCES projects (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS project_budget (
    id               BIGINT         NOT NULL AUTO_INCREMENT,
    finance_doc_id   VARCHAR(64)    NOT NULL,
    project_id       BIGINT         NOT NULL,
    voucher_type     VARCHAR(64)    NOT NULL,
    unit             VARCHAR(32)    NOT NULL,
    total_amount     DECIMAL(20, 8) NOT NULL DEFAULT 0,
    available_amount DECIMAL(20, 8) NOT NULL DEFAULT 0,
    withhold_amount  DECIMAL(20, 8) NOT NULL DEFAULT 0,
    issued_amount    DECIMAL(20, 8) NOT NULL DEFAULT 0,
    refund_amount    DECIMAL(20, 8) NOT NULL DEFAULT 0,
    created_at       DATETIME(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at       DATETIME(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_project_budget_doc (finance_doc_id, voucher_type, unit),
    UNIQUE KEY uk_project_budget_project (project_id, voucher_type, unit),
    CONSTRAINT fk_project_budget_project FOREIGN KEY (project_id) REFERENCES projects (id),
    CONSTRAINT fk_project_budget_finance_doc FOREIGN KEY (finance_doc_id) REFERENCES finance_docs (doc_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS finance_payments (
    payment_id          VARCHAR(64)    NOT NULL,
    finance_doc_id      VARCHAR(64)    NOT NULL,
    payment_address     VARCHAR(128)   NOT NULL,
    amount              DECIMAL(20, 8) NOT NULL,
    payment_status      VARCHAR(32)    NOT NULL,
    offsetted_amount    DECIMAL(20, 8) NOT NULL DEFAULT 0,
    offsetting_amount   DECIMAL(20, 8) NOT NULL DEFAULT 0,
    refunded_amount     DECIMAL(20, 8) NOT NULL DEFAULT 0,
    refunding_amount    DECIMAL(20, 8) NOT NULL DEFAULT 0,
    created_at          DATETIME(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at          DATETIME(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (payment_id),
    KEY idx_finance_payments_doc_id (finance_doc_id),
    CONSTRAINT fk_finance_payments_doc FOREIGN KEY (finance_doc_id) REFERENCES finance_docs (doc_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS issue_requests (
    id               BIGINT         NOT NULL AUTO_INCREMENT,
    project_id       BIGINT         NOT NULL,
    voucher_type     VARCHAR(64)    NOT NULL,
    unit             VARCHAR(32)    NOT NULL,
    amount           DECIMAL(20, 8) NOT NULL,
    request_status   VARCHAR(32)    NOT NULL DEFAULT 'DRAFT',
    expense_type     VARCHAR(32)    NOT NULL,
    creator          VARCHAR(128)   NOT NULL,
    remark           VARCHAR(512)   NOT NULL DEFAULT '',
    created_at       DATETIME(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at       DATETIME(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_issue_requests_project_id (project_id),
    KEY idx_issue_requests_status (request_status),
    CONSTRAINT fk_issue_requests_project FOREIGN KEY (project_id) REFERENCES projects (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS issue_budget (
    id               BIGINT         NOT NULL AUTO_INCREMENT,
    issue_request_id BIGINT         NOT NULL,
    voucher_type     VARCHAR(64)    NOT NULL,
    unit             VARCHAR(32)    NOT NULL,
    available_amount DECIMAL(20, 8) NOT NULL DEFAULT 0,
    total_amount     DECIMAL(20, 8) NOT NULL DEFAULT 0,
    issued_amount    DECIMAL(20, 8) NOT NULL DEFAULT 0,
    status           VARCHAR(32)    NOT NULL DEFAULT 'ONGOING',
    created_at       DATETIME(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at       DATETIME(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_issue_budget_request (issue_request_id),
    CONSTRAINT fk_issue_budget_request FOREIGN KEY (issue_request_id) REFERENCES issue_requests (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO payment_configs (pay_address, voucher_type, unit, payment_account) VALUES
    ('0xabc123wallet001', 'crypto', 'USD', 'ACC-CRYPTO-001'),
    ('0xdef456wallet002', 'crypto', 'USD', 'ACC-CRYPTO-002'),
    ('bank-usd-main-001', 'cash', 'USD', 'ACC-CASH-001')
ON DUPLICATE KEY UPDATE
    voucher_type = VALUES(voucher_type),
    unit = VALUES(unit),
    payment_account = VALUES(payment_account);
