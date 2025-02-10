use shibuya;

-- For backwards compatibility, we need to make all existing plan as jmeter
ALTER TABLE plan ADD COLUMN kind VARCHAR(50) NOT NULL DEFAULT "jmeter";
