package sdkgo

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gen"
	"gorm.io/gorm"
)

type MKDB struct {
	*gorm.DB
}

type QuerySDK struct {
	Query   string
	Order   []string
	Select  []string
	Joins   []string
	Preload []string
	Args    []any
}

type ExecQuery func(db *gorm.DB, model any, q QuerySDK) error

func NewMKDB(db *gorm.DB) *MKDB {
	return &MKDB{DB: db}
}

func (db *MKDB) GenTable(table string) {
	g := gen.NewGenerator(gen.Config{
		OutPath: "sdk",
		Mode:    gen.WithDefaultQuery,
	})
	g.UseDB(db.DB)
	g.GenerateModel(table)
	g.Execute()
}

func (db *MKDB) ExecQuery(model any, handler ExecQuery, q QuerySDK) error {
	destValue := reflect.ValueOf(model)
	if destValue.Kind() != reflect.Ptr {
		return fmt.Errorf("model must be a pointer")
	}

	if len(q.Select) > 0 {
		db.DB = db.DB.Select(strings.Join(q.Select, ", "))
	}

	if len(q.Joins) > 0 {
		for _, j := range q.Joins {
			db.DB = db.DB.Joins(j)
		}
	}

	if len(q.Preload) > 0 {
		for _, p := range q.Preload {
			db.DB = db.DB.Preload(p)
		}
	}

	if len(q.Order) > 0 {
		for _, o := range q.Order {
			db.DB = db.DB.Order(o)
		}
	}

	if err := handler(db.DB, model, q); err != nil {
		return err
	}
	return nil
}

func WhereQuery(db *gorm.DB, model any, q QuerySDK) error {
	if err := db.
		Where(q.Query, q.Args...).
		Find(model).
		Error; err != nil {
		return fmt.Errorf("failed to fetch model: %v", err)
	}
	return nil
}

func CustomQuery(db *gorm.DB, model any, p QuerySDK) error {
	if p.Query == "" {
		return fmt.Errorf("empty query")
	}

	destValue := reflect.ValueOf(model)
	if destValue.Kind() != reflect.Ptr {
		return fmt.Errorf("model must be a pointer")
	}

	elem := destValue.Elem()
	switch elem.Kind() {
	case reflect.Struct:
		if err := db.Raw(p.Query, p.Args...).First(model).Error; err != nil {
			return fmt.Errorf("error executing query: %v", err)
		}
	case reflect.Slice:
		if err := db.Raw(p.Query, p.Args...).Find(model).Error; err != nil {
			return fmt.Errorf("error executing query: %v", err)
		}
	default:
		return fmt.Errorf("model must be a pointer to a struct or a slice")
	}

	return nil
}

func NewGorm(host, user, password, database, timeZone, schema string, port int) *MKDB {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=%s search_path=%s client_encoding=UTF8",
		host, port, user, password, database, timeZone, schema)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect to the database:", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("failed to get *sql.DB from GORM:", err)
	}
	err = sqlDB.PingContext(ctx)
	if err != nil {
		log.Fatal("failed to ping database:", err)
	}
	fmt.Println("Connected to Gorm and schema is set up.")
	return &MKDB{DB: db}
}
