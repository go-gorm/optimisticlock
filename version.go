package optimisticlock

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type Version sql.NullInt64

func (v *Version) Scan(value interface{}) error {
	return (*sql.NullInt64)(v).Scan(value)
}

func (v Version) Value() (driver.Value, error) {
	if !v.Valid {
		return nil, nil
	}
	return v.Int64, nil
}

func (v *Version) UnmarshalJSON(bytes []byte) error {
	if string(bytes) == "null" {
		v.Valid = false
		return nil
	}
	err := json.Unmarshal(bytes, &v.Int64)
	if err == nil {
		v.Valid = true
	}
	return err
}

func (v Version) MarshalJSON() ([]byte, error) {
	if v.Valid {
		return []byte(strconv.FormatInt(v.Int64, 10)), nil
	}
	return []byte("null"), nil
}

func (v *Version) CreateClauses(field *schema.Field) []clause.Interface {
	return []clause.Interface{VersionCreateClause{Field: field}}
}

type VersionCreateClause struct {
	Field *schema.Field
}

func (v VersionCreateClause) Name() string {
	return ""
}

func (v VersionCreateClause) Build(clause.Builder) {
}

func (v VersionCreateClause) MergeClause(*clause.Clause) {
}

func (v VersionCreateClause) ModifyStatement(stmt *gorm.Statement) {
	var value int64 = 1
	if val, zero := v.Field.ValueOf(stmt.Context, stmt.ReflectValue); !zero {
		if version, ok := val.(Version); ok {
			value = version.Int64
		}
	}
	stmt.SetColumn(v.Field.DBName, value)
}

func (v *Version) UpdateClauses(field *schema.Field) []clause.Interface {
	return []clause.Interface{VersionUpdateClause{Field: field}}
}

type VersionUpdateClause struct {
	Field *schema.Field
}

func (v VersionUpdateClause) Name() string {
	return ""
}

func (v VersionUpdateClause) Build(clause.Builder) {
}

func (v VersionUpdateClause) MergeClause(*clause.Clause) {
}

func (v VersionUpdateClause) ModifyStatement(stmt *gorm.Statement) {
	if _, ok := stmt.Clauses["version_enabled"]; !ok {
		if c, ok := stmt.Clauses["WHERE"]; ok {
			if where, ok := c.Expression.(clause.Where); ok && len(where.Exprs) > 1 {
				for _, expr := range where.Exprs {
					if orCond, ok := expr.(clause.OrConditions); ok && len(orCond.Exprs) == 1 {
						where.Exprs = []clause.Expression{clause.And(where.Exprs...)}
						c.Expression = where
						stmt.Clauses["WHERE"] = c
						break
					}
				}
			}
		}

		if val, zero := v.Field.ValueOf(stmt.Context, stmt.ReflectValue); !zero {
			if version, ok := val.(Version); ok {
				stmt.AddClause(clause.Where{Exprs: []clause.Expression{
					clause.Eq{Column: clause.Column{Table: clause.CurrentTable, Name: v.Field.DBName}, Value: version.Int64},
				}})
			}
		}

		// struct to map
		dv := reflect.ValueOf(stmt.Dest)
		if dv.Kind() == reflect.Ptr {
			dv = dv.Elem()
		}
		if dv.Kind() == reflect.Struct {
			d := make(map[string]interface{})
			for i := 0; i < dv.NumField(); i++ {
				field := dv.Type().Field(i)
				if dv.Field(i).IsZero() || field.Name == v.Field.Name {
					continue
				}

				fv := reflect.ValueOf(dv.Field(i).Interface())
				if fv.Kind() != reflect.Struct {
					d[field.Name] = dv.Field(i).Interface()
					continue
				}

				// expand nested struct
				if field.Anonymous {
					for j := 0; j < fv.NumField(); j++ {
						if fv.Field(j).IsZero() {
							continue
						}

						d[fv.Type().Field(j).Name] = fv.Field(j).Interface()
					}
				}

				// implementation driver.Valuer interface
				valuer, ok := fv.Interface().(driver.Valuer)
				if !ok {
					continue
				}

				if value, err := valuer.Value(); err == nil {
					d[field.Name] = value
				}
			}

			stmt.Dest = d
		}

		stmt.SetColumn(v.Field.DBName, clause.Expr{SQL: fmt.Sprintf("`%s`+1", v.Field.DBName)}, true)
		stmt.Clauses["version_enabled"] = clause.Clause{}
	}
}
