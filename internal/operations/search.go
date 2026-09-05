package operations

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SearchFilter określa kryteria wyszukiwania plików.
type SearchFilter struct {
	NamePattern   string    // wzorzec nazwy pliku (filepath.Match), np. "*.go", "*test*"; pusty = dowolna nazwa
	Content       string    // szukany tekst w zawartości pliku; pusty = bez przeszukiwania treści
	CaseSensitive bool      // czy dopasowanie Content ma rozróżniać wielkość liter
	MinSize       int64     // minimalny rozmiar pliku w bajtach
	MaxSize       int64     // maksymalny rozmiar pliku w bajtach; 0 = bez limitu
	Since         time.Time // tylko pliki modyfikowane nie wcześniej niż ta data; zero = bez limitu
}

// SearchMatch opisuje pojedyncze trafienie. Gdy filtr nie obejmuje zawartości,
// LineNumber wynosi 0, a LineText jest pusty.
type SearchMatch struct {
	Path       string
	Size       int64
	ModTime    time.Time
	LineNumber int
	LineText   string
}

// errSearchCancelled oznacza przerwanie wyszukiwania przez kanał cancel.
var errSearchCancelled = errors.New("myc: wyszukiwanie anulowane")

// maxScanLineSize to górny limit długości jednej linii czytanej z pliku.
const maxScanLineSize = 1024 * 1024

// SearchFiles rekurewnie przeszukuje rootDir (kolejność leksykograficzna,
// bez podążania za dowiązaniami symbolicznymi). Pliki są filtrowane po nazwie
// (NamePattern dopasowywany do podstawowej nazwy pliku), rozmiarze i dacie
// modyfikacji. Jeżeli Content nie jest puste, zawartość pliku jest skanowana
// linia po linii (bufio.Scanner) i dla każdej pasującej linii raportowane jest
// osobne trafienie. Zamknięcie kanału cancel przerywa wyszukiwanie z błędem
// errSearchCancelled.
func SearchFiles(rootDir string, filter SearchFilter, onMatch func(match SearchMatch), cancel <-chan struct{}) error {
	if onMatch == nil {
		return errors.New("myc: onMatch nie może być nil")
	}

	pattern := filter.NamePattern
	if pattern == "" {
		pattern = "*"
	}
	// Walidacja wzorca przed rozpoczęciem spaceru po katalogach.
	if _, err := filepath.Match(pattern, ""); err != nil {
		return fmt.Errorf("myc: nieprawidłowy wzorzec nazwy %q: %w", pattern, err)
	}

	needle := filter.Content
	if !filter.CaseSensitive {
		needle = strings.ToLower(needle)
	}

	return filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if cancelled(cancel) {
			return errSearchCancelled
		}
		if d.IsDir() {
			return nil
		}

		// Wzorzec został już zweryfikowany powyżej, więc tu błąd jest niemożliwy.
		if matched, _ := filepath.Match(pattern, d.Name()); !matched {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if !passesMetadataFilters(info, filter) {
			return nil
		}

		if needle == "" {
			onMatch(SearchMatch{
				Path:    path,
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})
			return nil
		}
		return scanFileLines(path, info, needle, filter.CaseSensitive, onMatch, cancel)
	})
}

// passesMetadataFilters sprawdza kryteria rozmiaru i daty modyfikacji.
func passesMetadataFilters(info fs.FileInfo, filter SearchFilter) bool {
	if info.Size() < filter.MinSize {
		return false
	}
	if filter.MaxSize > 0 && info.Size() > filter.MaxSize {
		return false
	}
	if !filter.Since.IsZero() && info.ModTime().Before(filter.Since) {
		return false
	}
	return true
}

// scanFileLines szuka needle w kolejnych liniach path i raportuje każde trafienie.
func scanFileLines(path string, info fs.FileInfo, needle string, caseSensitive bool, onMatch func(SearchMatch), cancel <-chan struct{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanLineSize)

	lineNo := 0
	for scanner.Scan() {
		if cancelled(cancel) {
			return errSearchCancelled
		}
		lineNo++
		line := scanner.Text()
		haystack := line
		if !caseSensitive {
			haystack = strings.ToLower(haystack)
		}
		if !strings.Contains(haystack, needle) {
			continue
		}
		onMatch(SearchMatch{
			Path:       path,
			Size:       info.Size(),
			ModTime:    info.ModTime(),
			LineNumber: lineNo,
			LineText:   line,
		})
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("myc: odczyt %s: %w", path, err)
	}
	return nil
}

// cancelled nieblokująco raportuje, czy kanał cancel został zamknięty.
func cancelled(cancel <-chan struct{}) bool {
	if cancel == nil {
		return false
	}
	select {
	case <-cancel:
		return true
	default:
		return false
	}
}