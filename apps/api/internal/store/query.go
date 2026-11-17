package store

import (
	"context"
	"fmt"
	"strings"

	"dofusdb/api/internal/game"
)

// StatFilter restricts items by the best roll of one of their stats.
type StatFilter struct {
	StatID int
	// Op is "gte" or "lte".
	Op    string
	Value int
}

// ItemQuery describes an equipment search.
type ItemQuery struct {
	Lang    string
	Search  string
	Slots   []string
	TypeIDs []int
	// ExcludeTypeIDs drops item types, such as prysmaradites for the Dofus
	// positions that cannot hold one.
	ExcludeTypeIDs []int
	LevelMin       int
	LevelMax       int
	InSet          *bool
	SetID          int
	// Passive keeps only items with a passive effect (Dofus, legendary items).
	Passive bool
	Stats   []StatFilter
	// Sort is "level", "name" or "stat"; SortStatID applies to "stat".
	Sort       string
	SortStatID int
	Desc       bool
	Limit      int
	Offset     int
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// QueryItems returns the ids of the matching items, in order, and the total
// number of matches before pagination.
func (s *Store) QueryItems(ctx context.Context, q ItemQuery) (ids []int, total int, err error) {
	var where []string
	var args []any

	joins := "JOIN texts n ON n.kind = 'item' AND n.id = i.id AND n.lang = ?"
	joinArgs := []any{q.Lang}

	if term := game.Fold(q.Search); term != "" {
		for _, word := range strings.Fields(term) {
			where = append(where, `n.search LIKE ? ESCAPE '\'`)
			escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(word)
			args = append(args, "%"+escaped+"%")
		}
	}
	if len(q.Slots) > 0 {
		where = append(where, "i.slot IN ("+placeholders(len(q.Slots))+")")
		for _, slot := range q.Slots {
			args = append(args, slot)
		}
	}
	if len(q.TypeIDs) > 0 {
		where = append(where, "i.type_id IN ("+placeholders(len(q.TypeIDs))+")")
		for _, id := range q.TypeIDs {
			args = append(args, id)
		}
	}
	if len(q.ExcludeTypeIDs) > 0 {
		where = append(where, "i.type_id NOT IN ("+placeholders(len(q.ExcludeTypeIDs))+")")
		for _, id := range q.ExcludeTypeIDs {
			args = append(args, id)
		}
	}
	if q.LevelMin > 0 {
		where = append(where, "i.level >= ?")
		args = append(args, q.LevelMin)
	}
	if q.LevelMax > 0 {
		where = append(where, "i.level <= ?")
		args = append(args, q.LevelMax)
	}
	if q.InSet != nil {
		if *q.InSet {
			where = append(where, "i.set_id > 0")
		} else {
			where = append(where, "i.set_id = 0")
		}
	}
	if q.Passive {
		where = append(where, "EXISTS (SELECT 1 FROM texts p WHERE p.kind = 'passive_name' AND p.id = i.id)")
	}
	if q.SetID > 0 {
		where = append(where, "i.set_id = ?")
		args = append(args, q.SetID)
	}
	for _, f := range q.Stats {
		switch f.Op {
		case "gte":
			where = append(where, "EXISTS (SELECT 1 FROM item_stats f WHERE f.item_id = i.id AND f.stat_id = ? AND f.max_value >= ?)")
		case "lte":
			// An item without the stat counts as zero.
			if f.Value >= 0 {
				where = append(where, "NOT EXISTS (SELECT 1 FROM item_stats f WHERE f.item_id = i.id AND f.stat_id = ? AND f.max_value > ?)")
			} else {
				where = append(where, "EXISTS (SELECT 1 FROM item_stats f WHERE f.item_id = i.id AND f.stat_id = ? AND f.max_value <= ?)")
			}
		default:
			return nil, 0, fmt.Errorf("unknown stat filter operator %q", f.Op)
		}
		args = append(args, f.StatID, f.Value)
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}

	countSQL := "SELECT COUNT(*) FROM items i " + joins + whereSQL
	countArgs := append(append([]any{}, joinArgs...), args...)
	if err := s.db.QueryRowContext(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	direction := "ASC"
	if q.Desc {
		direction = "DESC"
	}
	var order string
	switch q.Sort {
	case "name":
		order = "n.search " + direction + ", i.id"
	case "stat":
		joins += " LEFT JOIN item_stats o ON o.item_id = i.id AND o.stat_id = ?"
		joinArgs = append(joinArgs, q.SortStatID)
		order = "COALESCE(o.max_value, 0) " + direction + ", i.level DESC, i.id"
	default:
		order = "i.level " + direction + ", n.search, i.id"
	}

	// A page holds 50 items unless asked otherwise, and never more than 200.
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	limit = min(limit, 200)
	listSQL := "SELECT i.id FROM items i " + joins + whereSQL + " ORDER BY " + order + " LIMIT ? OFFSET ?"
	listArgs := append(append(append([]any{}, joinArgs...), args...), limit, q.Offset)
	rows, err := s.db.QueryContext(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	ids = make([]int, 0, limit)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	return ids, total, rows.Err()
}
