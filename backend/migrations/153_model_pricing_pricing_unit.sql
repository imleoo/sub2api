-- Add billing unit for model pricing rows.
ALTER TABLE model_pricings
  ADD COLUMN IF NOT EXISTS pricing_unit varchar(20) NOT NULL DEFAULT 'token';

ALTER TABLE model_pricings
  DROP CONSTRAINT IF EXISTS model_pricings_pricing_unit_check;

ALTER TABLE model_pricings
  ADD CONSTRAINT model_pricings_pricing_unit_check
  CHECK (pricing_unit IN ('token', 'second'));
