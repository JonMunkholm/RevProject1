-- +goose Up
CREATE TABLE performance_obligations (
    ID UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    Performance_Obligations_Name CITEXT NOT NULL,
    Contract_ID UUID NOT NULL,
    Created_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Updated_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Start_Date TIMESTAMP NOT NULL,
    End_Date TIMESTAMP NOT NULL,
    Functional_Currency VARCHAR(3) NOT NULL,
    Discount NUMERIC(6, 5) NOT NULL,
    Transaction_Price BIGINT NOT NULL,
    CONSTRAINT fk_performance_obligations_contract_id
        FOREIGN KEY (Contract_ID)
        REFERENCES contracts(ID)
        ON DELETE CASCADE
);

CREATE TRIGGER update_performance_obligations_updated_at BEFORE UPDATE ON performance_obligations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Join table for Product to Performance Obligation
CREATE TABLE product_performance_obligations (
    Product_ID UUID NOT NULL,
    Performance_Obligations_ID UUID NOT NULL,
    PRIMARY KEY (Product_ID, Performance_Obligations_ID),
    CONSTRAINT fk_ppo_product_id
        FOREIGN KEY (Product_ID)
        REFERENCES products(ID)
        ON DELETE CASCADE,
    CONSTRAINT fk_ppo_performance_obligations_id
        FOREIGN KEY (Performance_Obligations_ID)
        REFERENCES performance_obligations(ID)
        ON DELETE CASCADE
);

-- Join table for Bundle to Performance Obligation
CREATE TABLE bundle_performance_obligations (
    Bundle_ID UUID NOT NULL,
    Performance_Obligations_ID UUID NOT NULL,
    PRIMARY KEY (Bundle_ID, Performance_Obligations_ID),
    CONSTRAINT fk_bpo_bundle_id
        FOREIGN KEY (Bundle_ID)
        REFERENCES bundles(ID)
        ON DELETE CASCADE,
    CONSTRAINT fk_bpo_performance_obligations_id
        FOREIGN KEY (Performance_Obligations_ID)
        REFERENCES performance_obligations(ID)
        ON DELETE CASCADE
);

-- +goose Down
DROP TABLE bundle_performance_obligations;
DROP TABLE product_performance_obligations;
DROP TABLE performance_obligations;
