通过这份代码，知道如何通过 Reflect 的灵活操作来完成 ORM

```
// 每一列
type Field struct {
	Name string
	Type string
	Tag  string
}

// Schema represents a table of database
type Schema struct {
	Model      interface{} // 对应的接口
	Name       string // 表名
	Fields     []*Field // 多个列的指针
	FieldNames []string // 列的名
	fieldMap   map[string]*Field // 通过FieldName 找到 列
}

type Session struct {
	db       *sql.DB
	dialect  dialect.Dialect // 适应不同DB 引擎的改造方法
	refTable *schema.Schema // 对应的Schema
	sql      strings.Builder // SQL语句
	sqlVars  []interface{}  // SQL变量
}

type Engine struct {
	db      *sql.DB
	dialect dialect.Dialect
}

// Clause contains SQL conditions
type Clause struct {
	sql     map[Type]string
	sqlVars map[Type][]interface{}
}


func Parse(dest interface{}, d dialect.Dialect) *Schema {
	//modelType := reflect.ValueOf(dest).Elem().Type()
	modelType := reflect.Indirect(reflect.ValueOf(dest)).Type()
	var tableName string
	t, ok := dest.(ITableName)
	if !ok {
		tableName = modelType.Name()
	} else {
		tableName = t.TableName()
	}
	schema := &Schema{
		Model:    dest,
		Name:     tableName,
		fieldMap: make(map[string]*Field),
	}

	for i := 0; i < modelType.NumField(); i++ {
		p := modelType.Field(i)
		if !p.Anonymous && ast.IsExported(p.Name) {
			field := &Field{
				Name: p.Name,
				Type: d.DataTypeOf(reflect.Indirect(reflect.New(p.Type))),
			}
			if v, ok := p.Tag.Lookup("geeorm"); ok {
				field.Tag = v
			}
			schema.Fields = append(schema.Fields, field)
			schema.FieldNames = append(schema.FieldNames, p.Name)
			schema.fieldMap[p.Name] = field
		}
	}
	return schema
}

schema := Parse(&User{}, TestDial)

// Find gets all eligible records
func (s *Session) Find(values interface{}) error {
	destSlice := reflect.Indirect(reflect.ValueOf(values))
	// destSlice.Type().Elem() 获取切片的单个元素的类型 destType，使用 reflect.New() 方法创建一个 destType 的实例，作为 Model() 的入参，映射出表结构 RefTable()。
	destType := destSlice.Type().Elem()
	table := s.Model(reflect.New(destType).Elem().Interface()).RefTable()

	s.clause.Set(clause.SELECT, table.Name, table.FieldNames)
	sql, vars := s.clause.Build(clause.SELECT, clause.WHERE, clause.ORDERBY, clause.LIMIT)
	rows, err := s.Raw(sql, vars...).QueryRows()
	if err != nil {
		return err
	}

	for rows.Next() {
		dest := reflect.New(destType).Elem()
		var values []interface{}
		for _, name := range table.FieldNames {
			// FieldByName("X")表示查询struct中元素名为X的值, 此时要找到他的类型,就要Addr().Interface()
			values = append(values, dest.FieldByName(name).Addr().Interface())
		}
		if err := rows.Scan(values...); err != nil {
			return err
		}
        // 这样设置的
		destSlice.Set(reflect.Append(destSlice, dest))
	}

	return rows.Close()
}

func (s *Session) First(value interface{}) error {
	dest := reflect.Indirect(reflect.ValueOf(value))
	destSlice := reflect.New(reflect.SliceOf(dest.Type())).Elem()
	if err := s.Limit(1).Find(destSlice.Addr().Interface()); err != nil {
		return err
	}
	if destSlice.Len() == 0 {
		return errors.New("NOT FOUND")
	}
	dest.Set(destSlice.Index(0))
	return nil
}
```

// 链式操作

```
// Limit adds limit condition to clause
func (s *Session) Limit(num int) *Session {
	s.clause.Set(clause.LIMIT, num)
	return s
}

```

Hook 操作 
```
// CallMethod calls the registered hooks
func (s *Session) CallMethod(method string, value interface{}) {
	fm := reflect.ValueOf(s.RefTable().Model).MethodByName(method)
	if value != nil {
		fm = reflect.ValueOf(value).MethodByName(method)
	}
	param := []reflect.Value{reflect.ValueOf(s)}
	if fm.IsValid() {
		if v := fm.Call(param); len(v) > 0 {
			if err, ok := v[0].Interface().(error); ok {
				log.Error(err)
			}
		}
	}
	return
}

```

Transaction 事务

```
// CommonDB is a minimal function set of db
type CommonDB interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
}

var _ CommonDB = (*sql.DB)(nil)
var _ CommonDB = (*sql.Tx)(nil)

// DB returns tx if a tx begins. otherwise return *sql.DB
func (s *Session) DB() CommonDB {
	if s.tx != nil {
		return s.tx
	}
	return s.db
}

// Exec raw sql with sqlVars
func (s *Session) Exec() (result sql.Result, err error) {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVars)
	if result, err = s.DB().Exec(s.sql.String(), s.sqlVars...); err != nil {
		log.Error(err)
	}
	return
}

// Begin a transaction
func (s *Session) Begin() (err error) {
	log.Info("transaction begin")
	if s.tx, err = s.db.Begin(); err != nil {
		log.Error(err)
		return
	}
	return
}

// Commit a transaction
func (s *Session) Commit() (err error) {
	log.Info("transaction commit")
	if err = s.tx.Commit(); err != nil {
		log.Error(err)
	}
	return
}

// Rollback a transaction
func (s *Session) Rollback() (err error) {
	log.Info("transaction rollback")
	if err = s.tx.Rollback(); err != nil {
		log.Error(err)
	}
	return

// 逻辑函数
type TxFunc func(*session.Session) (interface{}, error)

// 事务的Wrapper
func (engine *Engine) Transaction(f TxFunc) (result interface{}, err error) {
	s := engine.NewSession()
	if err := s.Begin(); err != nil {
		return nil, err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = s.Rollback()
			panic(p) // re-throw panic after Rollback
		} else if err != nil {
			_ = s.Rollback() // err is non-nil; don't change it
		} else {
			err = s.Commit() // err is nil; if Commit returns error update err
		}
	}()

	return f(s)
}

// 实际调用逻辑
_, err := engine.Transaction(func(s *session.Session) (result interface{}, err error) {
        _ = s.Model(&User{}).CreateTable()
        _, err = s.Insert(&User{"Tom", 18})
        return nil, errors.New("Error")
})
```
