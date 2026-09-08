package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// FileSource loads and saves segment configuration from/to a JSON file.
type FileSource struct {
	path string
}

// NewFileSource creates a config source from the given file path.
func NewFileSource(path string) *FileSource {
	return &FileSource{path: path}
}

// Load reads and parses the config file into a Snapshot.
func (fs *FileSource) Load() (*model.Snapshot, error) {
	data, err := os.ReadFile(fs.path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var snap model.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Execution order comes from dependsOn; the evaluator topologically sorts.
	if err := rejectLegacyOrder(data); err != nil {
		return nil, err
	}
	if err := rejectLegacyExpressionKeys(data); err != nil {
		return nil, err
	}
	if err := checkRuleNameUniqueness(&snap); err != nil {
		return nil, err
	}

	return &snap, nil
}

// rejectLegacyOrder fails a config that still carries the removed "order"
// field. Ignoring it silently would leave a stale config subtly misordered
// rather than loudly broken.
func rejectLegacyOrder(data []byte) error {
	var probe struct {
		Layers []struct {
			Name  string `json:"name"`
			Order *int   `json:"order"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil // the primary unmarshal already reported anything fatal
	}

	var stale []string
	for _, l := range probe.Layers {
		if l.Order != nil {
			stale = append(stale, l.Name)
		}
	}
	if len(stale) > 0 {
		return fmt.Errorf(
			"layers still declare the removed %q field: %s — declare execution order with %q instead",
			"order", strings.Join(stale, ", "), "dependsOn")
	}
	return nil
}

// rejectLegacyExpressionKeys fails a config still using the old "expression"
// vocabulary, which split into two names: a rule's test is a `condition`, and a
// named expr-lang value is an entry in `computed` whose source is a `formula`.
//
// This has to be checked explicitly. Unmarshalling ignores unknown fields, so a
// stale `expression` on a rule would leave it with no condition and no children
// — evaluating false forever — and a stale `expressions` list would silently
// drop every computed field.
func rejectLegacyExpressionKeys(data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil // the primary unmarshal already reported anything fatal
	}

	found := map[string]bool{}
	var walk func(node interface{})
	walk = func(node interface{}) {
		switch n := node.(type) {
		case map[string]interface{}:
			for key, child := range n {
				if key == "expression" || key == "expressions" {
					found[key] = true
				}
				walk(child)
			}
		case []interface{}:
			for _, child := range n {
				walk(child)
			}
		}
	}
	walk(raw)

	if len(found) == 0 {
		return nil
	}

	var stale []string
	for _, key := range []string{"expression", "expressions"} {
		if found[key] {
			stale = append(stale, strconv.Quote(key))
		}
	}
	return fmt.Errorf(
		"config still uses the removed %s field(s): a rule's test is now %q, and a named "+
			"expr-lang value is an entry in %q whose source is %q",
		strings.Join(stale, " and "), "condition", "computed", "formula")
}

// checkRuleNameUniqueness enforces that the names of reported checks are unique
// across the whole config.
//
// A reported failure identifies itself by rule name alone, so the name is the
// stable public contract and must not collide. This is a property of the
// persisted collection rather than of any single rule's meaning, which is why
// it lives in the config source and not in domain validation — a
// database-backed source would get the same guarantee from a unique index.
//
// Applied by both Load and Save, which is what makes that comparison honest: an
// index constrains every write, not only every read. Save used to skip it, so
// the admin API could persist a collision that the next Load refused.
//
// Only top-level rules are checked. A checklist item is one rule; the tree
// beneath it builds that item's condition and never reports on its own, so the
// branches of an And/Or need no name at all.
func checkRuleNameUniqueness(snap *model.Snapshot) error {
	seen := make(map[string]string) // ruleName -> where it was first defined
	var errs []string

	for _, layer := range snap.Layers {
		for _, seg := range layer.Segments {
			if seg.Strategy != model.StrategyChecklist {
				continue
			}
			// Keyed by the layer's stable key, not its friendly name: the name is
			// optional, so this read "layer \"\"" for any layer without one.
			where := fmt.Sprintf("layer %q segment %q", layer.Key, seg.ID)

			for i := range seg.Rules {
				name := seg.Rules[i].RuleName
				switch prev, dup := seen[name]; {
				case name == "":
					errs = append(errs, fmt.Sprintf("%s: checklist rule with empty ruleName", where))
				case dup:
					errs = append(errs, fmt.Sprintf(
						"duplicate checklist ruleName %q in %s (already defined in %s)", name, where, prev))
				default:
					seen[name] = where
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("config rule name check failed:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// Save atomically writes the snapshot to disk (write tmp then rename).
// Stamps last_modified here in the repository layer since timestamps
// are a file-persistence concern, not needed by other storage backends.
func (fs *FileSource) Save(snap *model.Snapshot) error {
	// Enforced on the way in as well as on the way out. Load has always
	// rejected a duplicate; Save did not, so the admin API could write config
	// this very source would then refuse to read — the file on disk parted
	// company with the snapshot being served, and the failure surfaced later as
	// a reload the watcher could not apply or a process that would not start.
	//
	// This is the unique index the doc comment on the check describes: a
	// constraint the store applies to every write, whoever makes it.
	//
	// Before anything is marshalled or written, so a rejected save leaves the
	// file exactly as it was. AdminUseCase.commitSnapshot saves before swapping
	// the store, so the in-memory snapshot is left alone too.
	if err := checkRuleNameUniqueness(snap); err != nil {
		return err
	}

	now := time.Now().UTC()
	snap.LastModified = &now

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	dir := filepath.Dir(fs.path)
	tmp, err := os.CreateTemp(dir, "segments-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, fs.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}
