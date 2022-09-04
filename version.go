package optimisticlock

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"reflect"
	"strconv"
	"sync"

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
		return strconv.AppendInt(nil, v.Int64, 10), nil
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
	switch stmt.ReflectValue.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < stmt.ReflectValue.Len(); i++ {
			v.setVersionColumn(stmt, stmt.ReflectValue.Index(i))
		}
	case reflect.Struct:
		v.setVersionColumn(stmt, stmt.ReflectValue)
	}
}

func (v VersionCreateClause) setVersionColumn(stmt *gorm.Statement, reflectValue reflect.Value) {
	var value int64 = 1
	if val, zero := v.Field.ValueOf(stmt.Context, reflectValue); !zero {
		if version, ok := val.(Version); ok {
			value = version.Int64
		}
	}
	v.Field.Set(stmt.Context, reflectValue, value)
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
	if _, ok := stmt.Clauses["version_enabled"]; ok {
		return
	}

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

	// struct to map. version column is int64, but need set its value to string
	dv := reflect.ValueOf(stmt.Dest)
	if reflect.Indirect(dv).Kind() == reflect.Struct {
		selectColumns, restricted := stmt.SelectAndOmitColumns(false, true)

		sd, _ := schema.Parse(stmt.Dest, &sync.Map{}, stmt.DB.NamingStrategy)
		d := make(map[string]interface{})
		for _, field := range sd.Fields {
			if field.DBName == v.Field.DBName {
				continue
			}

			if v, ok := selectColumns[field.DBName]; (ok && v) || (!ok && (!restricted || (!stmt.SkipHooks && field.AutoUpdateTime > 0))) {
				val, isZero := field.ValueOf(stmt.Context, dv)
				if (ok || !isZero) && field.Updatable {
					d[field.DBName] = val
				}
			}
		}

		stmt.Dest = d
	}

	stmt.SetColumn(v.Field.DBName, clause.Expr{SQL: stmt.Quote(v.Field.DBName) + "+1"}, true)
	stmt.Clauses["version_enabled"] = clause.Clause{}
}
