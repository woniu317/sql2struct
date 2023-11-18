package table

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/starfishs/sql2struct/config"
	"github.com/starfishs/sql2struct/utils"
)

var tmpl = `
package {{.Package}}

import (
	"context"
	{{if .ContainsTimeField}}"time" {{end}}
	{{if .ContainsNullTimeField}}

	"gopkg.in/guregu/null.v3"
	{{end}}

	"code.yunzhanghu.com/infra/kit/sqlx"
)

const table{{.UpperCamelCaseName}} = "{{.Name}}"

{{if .Comment}}// {{ .UpperCamelCaseName}} {{ .Comment}} {{end}}
type {{ .UpperCamelCaseName}} struct {
{{range .Fields}} {{.UpperCamelCaseName}}  {{.Type}} {{.Tag}} {{if .Comment}} // {{.Comment}} {{end}}
{{end}}}

// Insert{{.UpperCamelCaseName}} 插入数据
func Insert{{.UpperCamelCaseName}}(ctx context.Context, db sqlx.DB, {{.UpperCamelCaseNameParam}} *{{.UpperCamelCaseName}}) error {
	return sqlx.NamedInsertContext(ctx, db, table{{.UpperCamelCaseName}}, {{.UpperCamelCaseNameParam}})
}

// BulkInsert{{.UpperCamelCaseName}} 批量插入数据
func BulkInsert{{.UpperCamelCaseName}}(ctx context.Context, db sqlx.DB, {{.UpperCamelCaseNameParam}}s []*{{.UpperCamelCaseName}}) error {
	return sqlx.BulkInsertContext(ctx, db, table{{.UpperCamelCaseName}}, Slice2Interface({{.UpperCamelCaseNameParam}}s)...)
}

`

type Table struct {
	Package                 string  `sql:"package"`
	Name                    string  `sql:"name"`
	UpperCamelCaseName      string  `sql:"upper_camel_case_name"`
	UpperCamelCaseNameParam string  `sql:"upper_camel_case_name_param"`
	Comment                 string  `sql:"comment"`
	Fields                  []Field `sql:"fields"`
	ContainsTimeField       bool    `sql:"contains_time"`
	ContainsNullTimeField   bool    `sql:"contains_null_time"`
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

func (t *Table) GenerateCode() string {
	fromTmpl := tmpl
	for i, field := range t.Fields {
		tag := "`" + config.Cnf.DBTag + ":\"" + field.Name + "\""
		if field.IsPK {
			tag = strings.TrimRight(tag, "\"") + ";primary_key\""
		}
		if field.DefaultValue != "" {
			tag = strings.TrimRight(tag, "\"") + ";default:" + field.DefaultValue + "\""
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

func (t *Table) GenerateFile() error {
	dir := config.Cnf.OutputDir
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
	fileName := filepath.Join(dir, strings.ToLower(t.Name)+".go")
	fd, err := os.Create(fileName)

	if err != nil {
		panic(err)
	}
	defer fd.Close()
	_, err = fd.Write([]byte(t.GenerateCode()))
	if err != nil {
		return err
	}

	_, err = exec.Command("goimports", "-l", "-w", dir).Output()
	if err != nil {
		utils.PrintRed(err.Error())
	}
	_, err = exec.Command("gofmt", "-l", "-w", dir).Output()
	if err != nil {
		utils.PrintRed(err.Error())
	}
	utils.PrintGreen(fileName + " generate success")
	return nil
}

func FilterTables(tables []string) []string {
	if len(config.Cnf.TableRegexs) == 0 {
		return tables
	}
	var res []string
	for _, table := range tables {
		for _, r := range config.Cnf.TableRegexs {
			if utils.IsMatch(r, table) {
				res = append(res, table)
			}
		}
	}
	return res
}

func NameIsMatch(table string) bool {
	if len(config.Cnf.TableRegexs) == 0 {
		return true
	}
	for _, r := range config.Cnf.TableRegexs {
		if utils.IsMatch(r, table) {
			return true
		}
	}
	return false
}
