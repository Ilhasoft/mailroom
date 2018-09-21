package models

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/juju/errors"
	"github.com/nyaruka/goflow/assets"
)

// Field is our mailroom type for contact field types
type Field struct {
	f struct {
		UUID      FieldUUID        `json:"uuid"`
		Key       string           `json:"key"`
		Name      string           `json:"name"`
		FieldType assets.FieldType `json:"field_type"`
	}
}

// UUID returns the UUID of this field
func (f *Field) UUID() FieldUUID { return f.f.UUID }

// Key returns the key for this field
func (f *Field) Key() string { return f.f.Key }

// Name returns the name for this field
func (f *Field) Name() string { return f.f.Name }

// Type returns the value type for this field
func (f *Field) Type() assets.FieldType { return f.f.FieldType }

// loadFields loads the assets for the passed in db
func loadFields(ctx context.Context, db sqlx.Queryer, orgID OrgID) ([]assets.Field, error) {
	rows, err := db.Queryx(selectFieldsSQL, orgID)
	if err != nil {
		return nil, errors.Annotatef(err, "error querying fields for org: %d", orgID)
	}
	defer rows.Close()

	fields := make([]assets.Field, 0, 10)
	for rows.Next() {
		field := &Field{}
		err = readJSONRow(rows, &field.f)
		if err != nil {
			return nil, errors.Annotate(err, "error reading field")
		}
		fields = append(fields, field)
	}

	return fields, nil
}

const selectFieldsSQL = `
SELECT ROW_TO_JSON(f) FROM (SELECT
	uuid,
	key,
	label as name,
	(SELECT CASE value_type
		WHEN 'T' THEN 'text' 
		WHEN 'N' THEN 'number'
		WHEN 'D' THEN 'datetime'
		WHEN 'S' THEN 'state'
		WHEN 'I' THEN 'district'
		WHEN 'W' THEN 'ward'
	END) as field_type
FROM 
	contacts_contactfield 
WHERE 
	org_id = $1 AND 
	is_active = TRUE AND
	field_type = 'U'
ORDER BY
	key ASC
) f;
`