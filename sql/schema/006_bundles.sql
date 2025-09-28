-- +goose Up
CREATE TABLE bundles (
    ID UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    Bundle_Name CITEXT NOT NULL,
    Is_Active BOOL NOT NULL DEFAULT TRUE,
    Company_ID UUID NOT NULL,
    Created_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Updated_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_company_id
        FOREIGN KEY (Company_ID)
        REFERENCES companies(ID)
        ON DELETE CASCADE,
    CONSTRAINT UQ_bundle_company_id UNIQUE (ID, Company_ID),
    CONSTRAINT UQ_bundle_name_per_company UNIQUE (Company_ID, Bundle_Name),
    CONSTRAINT CHK_bundle_name_not_blank CHECK (BTRIM(Bundle_Name) <> '')
);

CREATE TRIGGER update_bundles_updated_at BEFORE UPDATE ON bundles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE INDEX idx_bundles_company_id ON bundles (Company_ID);
CREATE INDEX idx_bundles_company_active ON bundles (Company_ID) WHERE Is_Active;

CREATE TABLE bundle_products (
    Bundle_ID UUID NOT NULL,
    Product_ID UUID NOT NULL,
    Company_ID UUID NOT NULL,
    PRIMARY KEY (Bundle_ID, Product_ID),
    CONSTRAINT fk_bundle_products_bundle_company
        FOREIGN KEY (Bundle_ID, Company_ID)
        REFERENCES bundles(ID, Company_ID)
        ON DELETE CASCADE,
    CONSTRAINT fk_bundle_products_product_company
        FOREIGN KEY (Product_ID, Company_ID)
        REFERENCES products(ID, Company_ID)
        ON DELETE CASCADE,
    CONSTRAINT fk_bundle_products_company_id
        FOREIGN KEY (Company_ID)
        REFERENCES companies(ID)
        ON DELETE CASCADE,
    CONSTRAINT UQ_bundle_products_company UNIQUE (Company_ID, Bundle_ID, Product_ID)
);

CREATE INDEX idx_bundle_products_product_id ON bundle_products (Product_ID);
CREATE INDEX idx_bundle_products_company_bundle ON bundle_products (Company_ID, Bundle_ID);
CREATE INDEX idx_bundle_products_company_product ON bundle_products (Company_ID, Product_ID);

-- +goose Down
DROP INDEX IF EXISTS idx_bundle_products_company_product;
DROP INDEX IF EXISTS idx_bundle_products_company_bundle;
DROP INDEX IF EXISTS idx_bundle_products_product_id;
DROP INDEX IF EXISTS idx_bundles_company_active;
DROP INDEX IF EXISTS idx_bundles_company_id;
DROP TRIGGER IF EXISTS update_bundles_updated_at ON bundles;
DROP TABLE bundle_products;
DROP TABLE bundles;
