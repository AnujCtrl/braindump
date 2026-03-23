package main

import (
	"fmt"
	"time"
	"os"
	"github.com/anujp/braindump/internal/core"
)

func main() {
	dir, _ := os.MkdirTemp("", "debug")
	defer os.RemoveAll(dir)
	
	store := core.NewStore(dir)
	created := time.Now().AddDate(0, 0, -2)
	sc := time.Now().Add(-25 * time.Hour)
	
	todo := &core.Todo{
		ID: "aaa111", Text: "Old active item", Source: "cli", Status: "active",
		Created: created, StatusChanged: sc,
	}
	
	md := todo.ToMarkdown()
	fmt.Printf("ToMarkdown: %s\n", md)
	
	if err := store.AddTodo(todo); err != nil {
		fmt.Printf("AddTodo error: %v\n", err)
		return
	}
	
	date := created.Format("2006-01-02")
	todos, err := store.ReadDay(date)
	if err != nil {
		fmt.Printf("ReadDay error: %v\n", err)
		return
	}
	
	for _, t := range todos {
		fmt.Printf("Read back - ID:%s Status:%s SC:%v zero:%v\n", 
			t.ID, t.Status, t.StatusChanged, t.StatusChanged.IsZero())
	}
	
	stales, err := core.FindStaleItems(store)
	if err != nil {
		fmt.Printf("FindStaleItems error: %v\n", err)
		return
	}
	fmt.Printf("Stale items: %d\n", len(stales))
	
	cutoff := time.Now().Add(-24 * time.Hour)
	fmt.Printf("cutoff: %v\n", cutoff)
	fmt.Printf("sc before cutoff: %v\n", sc.Before(cutoff))
}
