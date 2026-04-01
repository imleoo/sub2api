package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// KeywordStat holds the schema definition for the keyword statistics entity.
// 存储从用户提示词中提取的关键词频次统计（不存储原始提示词内容）。
type KeywordStat struct {
	ent.Schema
}

func (KeywordStat) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "keyword_stats"},
	}
}

func (KeywordStat) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("api_key_id"),
		field.Int64("group_id"),
		field.String("keyword").MaxLen(64),
		field.Int("count").Default(1),
		field.String("period").MaxLen(10), // 例如 "2024-01" 月份维度
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{
				dialect.Postgres: "timestamptz",
			}),
	}
}

func (KeywordStat) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "period"),
		index.Fields("keyword", "period"),
		index.Fields("created_at"),
	}
}
