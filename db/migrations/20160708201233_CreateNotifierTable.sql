-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE notifiers (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  app_id UUID NOT NULL REFERENCES apps (id),
  service varchar(5) NOT NULL CHECK (service <> ''),
  created_at bigint NOT NULL,
  updated_at bigint NULL
);
CREATE UNIQUE INDEX unique_notifier_app_service ON notifiers (app_id, (lower(service)));

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE notifiers;