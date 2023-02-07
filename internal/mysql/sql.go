package mysqlparser

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/gangming/sql2struct/config"
	"github.com/gangming/sql2struct/internal/infra"
	"github.com/gangming/sql2struct/utils"
)

var tmpl = `// Code generated by sql2struct. https://github.com/gangming/sql2struct
package {{.Package}}

{{if .ContainsTimeField}}import "time" {{end}}

//{{ .UpperCamelCaseName}} {{.Table.Comment}}
type {{ .UpperCamelCaseName}} struct {
{{range .Fields}} {{.UpperCamelCaseName}}  {{.Type}} {{.Tag}} {{if .Comment}} // {{.Comment}} {{end}}
{{end}}}

// TableName the name of table in database
func (t *{{.UpperCamelCaseName}}) TableName() string {
    return "{{.Table.Name}}"
}
`
var MysqlType2GoType = map[string]string{
	"int":       "int64",
	"tinyint":   "uint8",
	"bigint":    "int64",
	"varchar":   "string",
	"char":      "string",
	"text":      "string",
	"date":      "time.Time",
	"time":      "time.Time",
	"datetime":  "time.Time",
	"timestamp": "time.Time",
	"json":      "string",
}

type Table struct {
	Name               string  `sql:"name"`
	UpperCamelCaseName string  `sql:"upper_camel_case_name"`
	Comment            string  `sql:"comment"`
	Fields             []Field `sql:"fields"`
	ContainsTimeField  bool    `sql:"contains_time"`
}
type Field struct {
	IsPK               bool   `json:"is_pk"`
	Name               string `json:"name"`
	UpperCamelCaseName string `json:"upper_camel_case_name"`
	Type               string `json:"type"`
	Comment            string `json:"comment"`
	DefaultValue       string `json:"default_value"`
	Tag                string `json:"tag"`
}

func ParseMysqlDDL(s string) (Table, error) {
	lines := strings.Split(s, "\n")
	var table Table
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "CREATE TABLE") {
			tableName := strings.Split(line, "`")[1]
			table.Name = tableName
			table.UpperCamelCaseName = utils.Underline2UpperCamelCase(tableName)
			continue
		}
		if strings.Contains(line, "ENGINE") && strings.Contains(line, "COMMENT=") {
			table.Comment = strings.Trim(strings.Split(line, "COMMENT='")[1], "'")
			fmt.Println(table.Comment)
			continue
		}
		if line[0] == '`' {
			field := Field{}
			field.Name = strings.Split(line, "`")[1]
			field.UpperCamelCaseName = utils.Underline2UpperCamelCase(field.Name)
			field.Type = strings.TrimRightFunc(strings.Split(line, " ")[1], func(r rune) bool {
				return r < 'a' || r > 'z'
			})
			field.Type = MysqlType2GoType[field.Type]
			if strings.Contains(field.Type, "time") {
				table.ContainsTimeField = true
			}
			if strings.Contains(line, "COMMENT") {
				field.Comment = strings.Trim(strings.Split(line, "COMMENT '")[1], "',")
			}
			if strings.Contains(line, "DEFAULT'") {
				field.DefaultValue = strings.Split(line, "DEFAULT ")[1]
			}
			if strings.Contains(line, "PRIMARY KEY") {
				field.IsPK = true
			}

			table.Fields = append(table.Fields, field)

		}

	}
	return table, nil
}
func (t *Table) GenerateCode() string {
	tableName := config.Cnf.TablePrefix + t.Name
	fromTmpl := tmpl
	fromTmpl = strings.Replace(fromTmpl, "{{.Package}}", "model", -1)
	fromTmpl = strings.Replace(fromTmpl, "{{.Table.Name}}", tableName, -1)
	fromTmpl = strings.Replace(fromTmpl, "{{.Table.Comment}}", t.Comment, -1)
	for i, field := range t.Fields {
		tag := "`" + config.Cnf.DBTag + ":\"column:" + field.Name + "\""
		if field.IsPK {
			tag += ";primary_key\" "
		}
		if field.DefaultValue != "" {
			tag += ";default:" + field.DefaultValue + "\""
		}
		if config.Cnf.WithJsonTag {
			tag += " json:\"" + field.Name + "\""
		}
		tag += "`"
		t.Fields[i].Tag = tag
	}
	tl := template.Must(template.New("tmpl").Parse(fromTmpl))
	var res bytes.Buffer
	err := tl.Execute(&res, t)
	if err != nil {
		panic(err)
	}
	return utils.CommonInitialisms(res.String())
}

func GetDDLs() ([]string, error) {
	var result []string
	tables := GetTables()
	for _, tableName := range tables {
		rows, err := infra.GetDB().Query("show create table " + tableName)
		if err != nil {
			panic(err)
		}

		if rows.Next() {
			var r string
			err := rows.Scan(&tableName, &r)
			if err != nil {
				panic(err)
			}
			result = append(result, r)
		}
	}

	return result, nil
}
func GetTables() []string {
	if len(config.Cnf.Tables) > 0 {
		return config.Cnf.Tables
	}
	var result []string
	rows, err := infra.GetDB().Query("show tables")
	if err != nil {
		panic(err)
	}

	for rows.Next() {
		var r string
		err := rows.Scan(&r)
		if err != nil {
			panic(err)
		}
		result = append(result, r)
	}
	return result
}
func Run() error {
	ddls, err := GetDDLs()
	if err != nil {
		return err
	}
	for _, ddl := range ddls {
		GenerateFile(ddl)
	}
	return nil
}
