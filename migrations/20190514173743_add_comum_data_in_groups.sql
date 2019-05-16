
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE job_groups ADD COLUMN "created_at" bigint DEFAULT 0;
ALTER TABLE job_groups ADD COLUMN "context" JSONB NOT NULL DEFAULT '{}'::JSONB;
ALTER TABLE job_groups ADD COLUMN "metadata" JSONB NOT NULL DEFAULT '{}'::JSONB;
ALTER TABLE job_groups ADD COLUMN "template_name" text NOT NULL DEFAULT '';
ALTER TABLE job_groups ADD COLUMN "control_group" real;
ALTER TABLE job_groups ADD COLUMN "created_by" text;
ALTER TABLE job_groups ADD COLUMN "csv_path" text;
ALTER TABLE job_groups ADD COLUMN "localized" BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE job_groups ADD COLUMN "localized" BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE job_groups ADD COLUMN "past_time_strategy" TEXT;

ALTER TABLE job_groups DROP CONSTRAINT jobs_group_app_id_apps_id_foreign;
TRUNCATE job_groups;

INSERT INTO job_groups (id, app_id, created_at, context, metadata, template_name, control_group, created_by, csv_path, localized, past_time_strategy)
SELECT job_group_id as id, app_id, created_at, context, metadata, template_name, control_group, created_by, csv_path, localized, past_time_strategy 
FROM jobs GROUP BY job_group_id, app_id, created_at, context, metadata, template_name, control_group, created_by, csv_path, localized, past_time_strategy;

ALTER TABLE jobs DROP COLUMN "app_id";
ALTER TABLE jobs DROP COLUMN "created_at";
ALTER TABLE jobs DROP COLUMN "context";
ALTER TABLE jobs DROP COLUMN "metadata";
ALTER TABLE jobs DROP COLUMN "template_name";
ALTER TABLE jobs DROP COLUMN "control_group";
ALTER TABLE jobs DROP COLUMN "created_by";
ALTER TABLE jobs DROP COLUMN "csv_path";
ALTER TABLE jobs DROP COLUMN "localized";
ALTER TABLE jobs DROP COLUMN "past_time_strategy";

ALTER TABLE job_groups ALTER COLUMN created_at DROP DEFAULT;
ALTER TABLE job_groups ALTER COLUMN template_name DROP DEFAULT;

ALTER TABLE "job_groups"
ADD CONSTRAINT jobs_group_app_id_apps_id_foreign
FOREIGN KEY (app_id)
REFERENCES apps(id)
ON DELETE CASCADE
ON UPDATE CASCADE;
-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

ALTER TABLE jobs ADD COLUMN "app_id" uuid NOT NULL DEFAULT uuid_generate_v4();
ALTER TABLE jobs ADD COLUMN "created_at" bigint DEFAULT 0;
ALTER TABLE jobs ADD COLUMN "context" JSONB NOT NULL DEFAULT '{}'::JSONB;
ALTER TABLE jobs ADD COLUMN "metadata" JSONB NOT NULL DEFAULT '{}'::JSONB;
ALTER TABLE jobs ADD COLUMN "template_name" text NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN "control_group" real;
ALTER TABLE jobs ADD COLUMN "created_by" text;
ALTER TABLE jobs ADD COLUMN "csv_path" text;
ALTER TABLE jobs ADD COLUMN "localized" BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE jobs ADD COLUMN "past_time_strategy" TEXT;

UPDATE jobs
SET 
	app_id=sub.app_id,
	created_at=sub.created_at,
	context=sub.context,
	metadata=sub.metadata,
	template_name=sub.template_name,
	control_group=sub.control_group,
	created_by=sub.created_by,
	csv_path=sub.csv_path
	localized=sub.localized
	past_time_strategy=sub.past_time_strategy
FROM (SELECT id, app_id, created_at, context, metadata, template_name, control_group, created_by, csv_path, localized, past_time_strategy
      FROM job_groups) AS sub
WHERE jobs.job_group_id=sub.id;

ALTER TABLE job_groups ALTER COLUMN created_at DROP DEFAULT;
ALTER TABLE job_groups ALTER COLUMN template_name DROP DEFAULT;

ALTER TABLE job_groups DROP COLUMN "created_at";
ALTER TABLE job_groups DROP COLUMN "context";
ALTER TABLE job_groups DROP COLUMN "metadata";
ALTER TABLE job_groups DROP COLUMN "template_name";
ALTER TABLE job_groups DROP COLUMN "control_group";
ALTER TABLE job_groups DROP COLUMN "created_by";
ALTER TABLE job_groups DROP COLUMN "csv_path";
ALTER TABLE job_groups DROP COLUMN "localized";
ALTER TABLE job_groups DROP COLUMN "past_time_strategy";
