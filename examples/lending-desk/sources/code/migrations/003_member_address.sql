ALTER TABLE members
    ADD COLUMN address_street   VARCHAR(200),
    ADD COLUMN address_city     VARCHAR(100),
    ADD COLUMN address_postcode VARCHAR(20),
    ADD CONSTRAINT members_address CHECK (
        (address_street IS NULL AND address_city IS NULL AND address_postcode IS NULL)
        OR (address_street IS NOT NULL AND address_city IS NOT NULL)
    );
