package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

func main() {
	var (
		paragraphIDStr   = flag.String("paragraph", "", "Paragraph UUID to update (required)")
		effectiveDateStr = flag.String("effective-date", "", "New effective date (YYYY-MM-DD)")
		supersededStr    = flag.String("superseded", "", "Set superseded status (true/false)")
		reason           = flag.String("reason", "", "Reason for the change (required)")
		actorFlag        = flag.String("actor", os.Getenv("GUIDANCE_ACTOR"), "Actor identifier (defaults to GUIDANCE_ACTOR env)")
	)
	flag.Parse()

	if strings.TrimSpace(*paragraphIDStr) == "" {
		log.Fatal("--paragraph is required")
	}
	if strings.TrimSpace(*reason) == "" {
		log.Fatal("--reason is required")
	}
	if strings.TrimSpace(*actorFlag) == "" {
		log.Fatal("actor is required (set --actor or GUIDANCE_ACTOR env)")
	}

	updateEffective := strings.TrimSpace(*effectiveDateStr) != ""
	updateSuperseded := strings.TrimSpace(*supersededStr) != ""
	if !updateEffective && !updateSuperseded {
		log.Fatal("provide at least one of --effective-date or --superseded")
	}

	paragraphID, err := uuid.Parse(strings.TrimSpace(*paragraphIDStr))
	if err != nil {
		log.Fatalf("invalid paragraph uuid: %v", err)
	}

	var effectiveDate *time.Time
	if updateEffective {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*effectiveDateStr))
		if err != nil {
			log.Fatalf("invalid effective date format: %v", err)
		}
		effectiveDate = &parsed
	}

	var superseded *bool
	if updateSuperseded {
		value, err := parseBool(*supersededStr)
		if err != nil {
			log.Fatalf("invalid superseded value: %v", err)
		}
		superseded = &value
	}

	dbURL := os.Getenv("DB_URL")
	if strings.TrimSpace(dbURL) == "" {
		log.Fatal("DB_URL environment variable must be set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	if err := runUpdate(db, paragraphID, effectiveDate, superseded, strings.TrimSpace(*actorFlag), strings.TrimSpace(*reason)); err != nil {
		log.Fatalf("update failed: %v", err)
	}

	log.Println("metadata update committed")
}

func runUpdate(db *sql.DB, paragraphID uuid.UUID, effectiveDate *time.Time, superseded *bool, actor, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentEffective sql.NullTime
	var currentSuperseded sql.NullBool
	row := tx.QueryRowContext(ctx, `
        select effective_date, superseded
        from asc_paragraphs
        where id = $1
        for update
    `, paragraphID)
	if err := row.Scan(&currentEffective, &currentSuperseded); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("paragraph %s not found", paragraphID)
		}
		return err
	}

	before := map[string]interface{}{
		"effective_date": nullTimeToString(currentEffective),
		"superseded":     nullBoolToBool(currentSuperseded),
	}

	clauses := make([]string, 0, 2)
	args := []interface{}{paragraphID}
	argPos := 2

	if effectiveDate != nil {
		clauses = append(clauses, fmt.Sprintf("effective_date = $%d", argPos))
		args = append(args, *effectiveDate)
		argPos++
	}
	if superseded != nil {
		clauses = append(clauses, fmt.Sprintf("superseded = $%d", argPos))
		args = append(args, *superseded)
		argPos++
	}

	if len(clauses) == 0 {
		return errors.New("no fields to update")
	}

	query := fmt.Sprintf(`
        update asc_paragraphs
        set %s,
            updated_at = now()
        where id = $1
    `, strings.Join(clauses, ",\n            "))

	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("update paragraph: %w", err)
	}

	var newEffective sql.NullTime
	var newSuperseded sql.NullBool
	row = tx.QueryRowContext(ctx, `
        select effective_date, superseded
        from asc_paragraphs
        where id = $1
    `, paragraphID)
	if err := row.Scan(&newEffective, &newSuperseded); err != nil {
		return fmt.Errorf("re-read paragraph: %w", err)
	}

	after := map[string]interface{}{
		"effective_date": nullTimeToString(newEffective),
		"superseded":     nullBoolToBool(newSuperseded),
	}

	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("marshal before: %w", err)
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("marshal after: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
        insert into guidance_audit (
            paragraph_id,
            change_type,
            actor,
            before_state,
            after_state,
            reason
        ) values ($1, $2, $3, $4, $5, $6)
    `, paragraphID, "metadata_update", actor, beforeJSON, afterJSON, reason); err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "t", "1", "yes", "y":
		return true, nil
	case "false", "f", "0", "no", "n":
		return false, nil
	default:
		return false, fmt.Errorf("cannot parse %q as bool", value)
	}
}

func nullTimeToString(v sql.NullTime) interface{} {
	if !v.Valid {
		return nil
	}
	return v.Time.Format(time.RFC3339)
}

func nullBoolToBool(v sql.NullBool) interface{} {
	if !v.Valid {
		return nil
	}
	return v.Bool
}
