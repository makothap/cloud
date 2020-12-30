package pg

import (
	"strings"

	"github.com/go-pg/pg/v10/orm"
)

type Event struct {
	tableName      struct{} `pg:"_"`
	Version        uint64   `pg:",pk,unique:unique_event"`
	AggregateID    int64    `pg:",pk,unique:unique_event"`
	AggregateIDStr string
	Data           []byte
	EventType      string
}

type Snapshot struct {
	tableName   struct{} `pg:"_"`
	AggregateID int64    `pg:",pk,unique"`
	Version     uint64
}

func eventTableName(groupID string) string {
	return strings.ToLower("e_" + strings.ReplaceAll(groupID, "-", ""))
}

func eventTable(q *orm.Query, groupID string) *orm.Query {
	return q.Table(eventTableName(groupID))
}

func snapshotTableName(groupID string) string {
	return strings.ToLower("s_" + strings.ReplaceAll(groupID, "-", ""))
}

func snapshotTable(q *orm.Query, groupID string) *orm.Query {
	return q.Table(snapshotTableName(groupID))
}
