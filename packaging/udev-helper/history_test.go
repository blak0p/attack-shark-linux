package helper_test

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"testing"

	packaged "github.com/blak0p/attack-shark-linux/packaging/udev"
)

func TestHistoryListGrammarAndBound(t *testing.T) {
	a, b := strings.Repeat("0", 64), strings.Repeat("f", 64)
	cases := map[string]string{"empty": "", "duplicate": a + "," + a, "unsorted": b + "," + a, "uppercase": strings.Repeat("A", 64), "short": a[:63], "trailing": a + ",", "space": a + ", " + b}
	var many []string
	for i := 0; i < 17; i++ {
		many = append(many, fmt.Sprintf("%064x", i))
	}
	cases["too-many"] = strings.Join(many, ",")
	for name, history := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			wire := nextPolicy(f, history, packaged.Policy())
			initial := snapshot(t, f.root)
			expect(t, f, wire, "", "error: invalid policy\n", 1, "1.2.3")
			unchanged(t, f, initial)
			untouched(t, f, nil)
		})
	}
	t.Run("maximum", func(t *testing.T) {
		f := setup(t)
		put(t, f.target, packaged.Policy(), 0644)
		history := []string{fmt.Sprintf("%x", sha256.Sum256([]byte(packaged.Policy())))}
		for i := 0; i < 15; i++ {
			history = append(history, strings.Repeat("1", 62)+fmt.Sprintf("%02d", i))
		}
		sort.Strings(history)
		payload := packaged.Policy() + "# Future official release with maximum history.\n"
		expect(t, f, nextPolicy(f, strings.Join(history, ","), payload), "installed\n", "", 0, "1.2.3")
	})
}
