package tempts

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestFilterWorkflowHistoriesBundle(t *testing.T) {
	makeBundle := func(name string, ids ...string) []byte {
		data := historiesData{WorkflowName: name}
		for _, id := range ids {
			data.Histories = append(data.Histories, historyWithMetadata{
				WorkflowID:   id,
				RunID:        "run-" + id,
				HistoryBytes: []byte("fake"),
			})
		}
		b, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	t.Run("keeps only compatible executions", func(t *testing.T) {
		bundle := makeBundle("MyWorkflow", "wf-1", "wf-2", "wf-3")

		filtered, count, err := FilterWorkflowHistoriesBundle(bundle, func(single []byte) error {
			var d historiesData
			if err := json.Unmarshal(single, &d); err != nil {
				return err
			}
			if d.Histories[0].WorkflowID == "wf-2" {
				return fmt.Errorf("incompatible")
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("expected 2 compatible, got %d", count)
		}

		var result historiesData
		if err := json.Unmarshal(filtered, &result); err != nil {
			t.Fatal(err)
		}
		if result.WorkflowName != "MyWorkflow" {
			t.Fatalf("expected workflow name MyWorkflow, got %s", result.WorkflowName)
		}
		if len(result.Histories) != 2 {
			t.Fatalf("expected 2 histories, got %d", len(result.Histories))
		}
		if result.Histories[0].WorkflowID != "wf-1" || result.Histories[1].WorkflowID != "wf-3" {
			t.Fatalf("unexpected workflow IDs: %s, %s", result.Histories[0].WorkflowID, result.Histories[1].WorkflowID)
		}
	})

	t.Run("returns error when all executions fail", func(t *testing.T) {
		bundle := makeBundle("MyWorkflow", "wf-1", "wf-2")

		_, n, err := FilterWorkflowHistoriesBundle(bundle, func(single []byte) error {
			return fmt.Errorf("incompatible")
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if n != 2 {
			t.Fatalf("expected total count 2, got %d", n)
		}
	})

	t.Run("returns error on malformed JSON", func(t *testing.T) {
		_, _, err := FilterWorkflowHistoriesBundle([]byte("not json"), func(single []byte) error {
			return nil
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("keeps all when all pass", func(t *testing.T) {
		bundle := makeBundle("MyWorkflow", "wf-1", "wf-2", "wf-3")

		filtered, count, err := FilterWorkflowHistoriesBundle(bundle, func(single []byte) error {
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if count != 3 {
			t.Fatalf("expected 3 compatible, got %d", count)
		}

		var result historiesData
		if err := json.Unmarshal(filtered, &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Histories) != 3 {
			t.Fatalf("expected 3 histories, got %d", len(result.Histories))
		}
	})
}
