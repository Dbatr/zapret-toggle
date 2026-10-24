// Package strategies читает список стратегий, который готовит system/convert.py.
package strategies

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Dir — каталог стратегий установленного zapret-dy.
const Dir = "/opt/zapret-dy/strategies"

// Strategy — одна стратегия: slug (имя экземпляра юнита) и оригинальное имя .bat у Flowseal.
type Strategy struct {
	Slug string
	Name string
}

var slugRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// ValidSlug проверяет slug той же регуляркой, что run.sh, firewall.sh и polkit-правило.
func ValidSlug(s string) bool {
	return slugRe.MatchString(s)
}

// Load читает <dir>/index.txt.
func Load(dir string) ([]Strategy, error) {
	f, err := os.Open(filepath.Join(dir, "index.txt"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// Parse разбирает строки вида "<slug>\t<имя>". Пустые строки пропускаются.
func Parse(r io.Reader) ([]Strategy, error) {
	var list []Strategy
	seen := map[string]bool{}
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		slug, name, ok := strings.Cut(line, "\t")
		slug, name = strings.TrimSpace(slug), strings.TrimSpace(name)
		if !ok || name == "" {
			name = slug
		}
		if !ValidSlug(slug) {
			return nil, fmt.Errorf("index.txt:%d: недопустимое имя стратегии %q", n, slug)
		}
		if seen[slug] {
			continue
		}
		seen[slug] = true
		list = append(list, Strategy{Slug: slug, Name: name})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("index.txt: нет ни одной стратегии")
	}
	return list, nil
}
