CREATE TABLE "wallet_pre_consume_records" ("id" bigserial,"request_id" varchar(64) NOT NULL,"user_id" bigint NOT NULL,"quota" bigint NOT NULL,"actual_quota" bigint NOT NULL DEFAULT 0,"status" varchar(16) NOT NULL,"created_at" bigint NOT NULL,"updated_at" bigint NOT NULL,PRIMARY KEY ("id"));
CREATE INDEX IF NOT EXISTS "idx_wallet_pre_consume_records_updated_at" ON "wallet_pre_consume_records" ("updated_at");
CREATE INDEX IF NOT EXISTS "idx_wallet_hold_user_status" ON "wallet_pre_consume_records" ("user_id","status");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_wallet_pre_consume_records_request_id" ON "wallet_pre_consume_records" ("request_id");
