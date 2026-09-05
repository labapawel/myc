package operations

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// syncTestBase to stały punkt odniesienia dla czasów modyfikacji w testach.
var syncTestBase = time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)

// --- funkcje pomocnicze ------------------------------------------------------

func writeTestFile(t *testing.T, path, content string, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	if !mod.IsZero() {
		if err := os.Chtimes(path, mod, mod); err != nil {
			t.Fatalf("Chtimes(%s): %v", path, err)
		}
	}
}

func assertFileContentString(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s: zawartość = %q, oczekiwano %q", path, string(got), want)
	}
}

func itemsByRelPath(t *testing.T, items []*SyncItem) map[string]*SyncItem {
	t.Helper()
	m := make(map[string]*SyncItem, len(items))
	for _, it := range items {
		m[it.RelPath] = it
	}
	return m
}

func testRelOf(t *testing.T, root, path string) string {
	t.Helper()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("Rel(%s, %s): %v", root, path, err)
	}
	return filepath.ToSlash(rel)
}

// --- CompareDirectories ------------------------------------------------------

func TestCompareDirectories(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()

	writeTestFile(t, filepath.Join(left, "same.txt"), "hello", syncTestBase)
	writeTestFile(t, filepath.Join(right, "same.txt"), "hello", syncTestBase)

	writeTestFile(t, filepath.Join(left, "left_newer.txt"), "aaaaaa", syncTestBase.Add(10*time.Minute))
	writeTestFile(t, filepath.Join(right, "left_newer.txt"), "bbbbbb", syncTestBase)

	writeTestFile(t, filepath.Join(left, "right_newer.txt"), "cccccc", syncTestBase)
	writeTestFile(t, filepath.Join(right, "right_newer.txt"), "dddddd", syncTestBase.Add(20*time.Minute))

	writeTestFile(t, filepath.Join(left, "size_diff.txt"), "short", syncTestBase.Add(30*time.Minute))
	writeTestFile(t, filepath.Join(right, "size_diff.txt"), "much longer content", syncTestBase)

	writeTestFile(t, filepath.Join(left, "docs", "guide.md"), "# przewodnik", syncTestBase)
	writeTestFile(t, filepath.Join(right, "notes", "todo.txt"), "kup mleko", syncTestBase)

	items, err := CompareDirectories(left, right)
	if err != nil {
		t.Fatalf("CompareDirectories: %v", err)
	}
	if len(items) != 6 {
		t.Fatalf("liczba pozycji = %d, oczekiwano 6 (%+v)", len(items), items)
	}

	// Wynik powinien być posortowany po RelPath.
	paths := make([]string, 0, len(items))
	for _, it := range items {
		paths = append(paths, it.RelPath)
	}
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	if !reflect.DeepEqual(paths, sorted) {
		t.Errorf("kolejność = %v, oczekiwano %v", paths, sorted)
	}

	byPath := itemsByRelPath(t, items)

	type expect struct {
		status SyncStatus
		action SyncAction
	}
	for rel, exp := range map[string]expect{
		"same.txt":        {StatusIdentical, ActionSkip},
		"left_newer.txt":  {StatusLeftNewer, ActionCopyToRight},
		"right_newer.txt": {StatusRightNewer, ActionCopyToLeft},
		"size_diff.txt":   {StatusSizeDiff, ActionCopyToRight},
		"docs/guide.md":   {StatusLeftOnly, ActionCopyToRight},
		"notes/todo.txt":  {StatusRightOnly, ActionCopyToLeft},
	} {
		item, ok := byPath[rel]
		if !ok {
			t.Errorf("brak pozycji dla %s", rel)
			continue
		}
		if item.Status != exp.status {
			t.Errorf("%s: status = %s, oczekiwano %s", rel, item.Status, exp.status)
		}
		if item.Action != exp.action {
			t.Errorf("%s: action = %s, oczekiwano %s", rel, item.Action, exp.action)
		}
		if item.LeftPath != filepath.Join(left, filepath.FromSlash(rel)) {
			t.Errorf("%s: LeftPath = %q", rel, item.LeftPath)
		}
		if item.RightPath != filepath.Join(right, filepath.FromSlash(rel)) {
			t.Errorf("%s: RightPath = %q", rel, item.RightPath)
		}
	}

	sd := byPath["size_diff.txt"]
	if sd.LeftSize != int64(len("short")) || sd.RightSize != int64(len("much longer content")) {
		t.Errorf("size_diff.txt: rozmiary = %d/%d", sd.LeftSize, sd.RightSize)
	}
	if got := byPath["same.txt"]; !got.LeftMod.Equal(syncTestBase) || !got.RightMod.Equal(syncTestBase) {
		t.Errorf("same.txt: czasy = %v/%v", got.LeftMod, got.RightMod)
	}
}

func TestCompareDirectoriesMissingDir(t *testing.T) {
	if _, err := CompareDirectories(t.TempDir(), filepath.Join(t.TempDir(), "brak")); err == nil {
		t.Fatal("oczekiwano błędu dla nieistniejącego katalogu")
	}
}

// --- ExecuteSync -------------------------------------------------------------

func TestExecuteSync(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()

	writeTestFile(t, filepath.Join(left, "same.txt"), "hello", syncTestBase)
	writeTestFile(t, filepath.Join(right, "same.txt"), "hello", syncTestBase)

	writeTestFile(t, filepath.Join(left, "left_newer.txt"), "aaaaaa", syncTestBase.Add(10*time.Minute))
	writeTestFile(t, filepath.Join(right, "left_newer.txt"), "bbbbbb", syncTestBase)

	writeTestFile(t, filepath.Join(left, "right_newer.txt"), "cccccc", syncTestBase)
	writeTestFile(t, filepath.Join(right, "right_newer.txt"), "dddddd", syncTestBase.Add(20*time.Minute))

	writeTestFile(t, filepath.Join(left, "docs", "guide.md"), "# przewodnik", syncTestBase)
	writeTestFile(t, filepath.Join(right, "notes", "todo.txt"), "kup mleko", syncTestBase)

	items, err := CompareDirectories(left, right)
	if err != nil {
		t.Fatalf("CompareDirectories: %v", err)
	}

	var progress []int
	err = ExecuteSync(items, func(current, total int, item *SyncItem) {
		if total != len(items) {
			t.Errorf("postęp: total = %d, oczekiwano %d", total, len(items))
		}
		if item == nil {
			t.Error("postęp: item = nil")
		}
		progress = append(progress, current)
	})
	if err != nil {
		t.Fatalf("ExecuteSync: %v", err)
	}
	if !reflect.DeepEqual(progress, []int{1, 2, 3, 4, 5}) {
		t.Errorf("postęp = %v, oczekiwano [1 2 3 4 5]", progress)
	}

	// Kopie w obie strony, łącznie z automatycznie utworzonymi podkatalogami.
	assertFileContentString(t, filepath.Join(right, "left_newer.txt"), "aaaaaa")
	assertFileContentString(t, filepath.Join(left, "right_newer.txt"), "dddddd")
	assertFileContentString(t, filepath.Join(right, "docs", "guide.md"), "# przewodnik")
	assertFileContentString(t, filepath.Join(left, "notes", "todo.txt"), "kup mleko")

	// Brak pozostałości plików tymczasowych.
	for _, dir := range []string{left, right} {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".myc-sync-tmp") {
				t.Errorf("pozostał plik tymczasowy: %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("WalkDir(%s): %v", dir, err)
		}
	}

	// Po synchronizacji oba katalogi powinny być identyczne.
	after, err := CompareDirectories(left, right)
	if err != nil {
		t.Fatalf("CompareDirectories po sync: %v", err)
	}
	if len(after) != 5 {
		t.Fatalf("liczba pozycji po sync = %d, oczekiwano 5", len(after))
	}
	for _, it := range after {
		if it.Status != StatusIdentical {
			t.Errorf("%s: status po sync = %s, oczekiwano %s", it.RelPath, it.Status, StatusIdentical)
		}
	}
}

func TestExecuteSyncDeleteActions(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()
	leftFile := filepath.Join(left, "stary.txt")
	rightFile := filepath.Join(right, "przestarzaly.txt")
	writeTestFile(t, leftFile, "a", syncTestBase)
	writeTestFile(t, rightFile, "b", syncTestBase)

	items := []*SyncItem{
		{RelPath: "stary.txt", LeftPath: leftFile, RightPath: rightFile, Action: ActionDeleteLeft},
		{RelPath: "przestarzaly.txt", LeftPath: leftFile, RightPath: rightFile, Action: ActionDeleteRight},
	}
	if err := ExecuteSync(items, nil); err != nil {
		t.Fatalf("ExecuteSync: %v", err)
	}
	if _, err := os.Stat(leftFile); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("lewy plik powinien być usunięty, err = %v", err)
	}
	if _, err := os.Stat(rightFile); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("prawy plik powinien być usunięty, err = %v", err)
	}
}

func TestExecuteSyncErrors(t *testing.T) {
	t.Run("missing-source", func(t *testing.T) {
		left := t.TempDir()
		right := t.TempDir()
		items := []*SyncItem{{
			RelPath:   "duch.txt",
			LeftPath:  filepath.Join(left, "duch.txt"),
			RightPath: filepath.Join(right, "duch.txt"),
			Action:    ActionCopyToRight,
		}}
		if err := ExecuteSync(items, nil); err == nil {
			t.Error("oczekiwano błędu dla nieistniejącego pliku źródłowego")
		}
	})

	t.Run("unknown-action", func(t *testing.T) {
		items := []*SyncItem{{RelPath: "x.txt", Action: SyncAction("NIC")}}
		if err := ExecuteSync(items, nil); err == nil {
			t.Error("oczekiwano błędu dla nieznanej akcji")
		}
	})
}

// --- SearchFiles -------------------------------------------------------------

func setupSearchTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n\n// TODO: popraw obsługę błędów\nfunc main() {}\n", syncTestBase)
	writeTestFile(t, filepath.Join(root, "utils", "helper.go"), "package utils\n\n// Todo: refaktoryzacja\nfunc Help() {}\n", syncTestBase)
	writeTestFile(t, filepath.Join(root, "notes.txt"), "lista zakupów\nmleko\nchleb\n", syncTestBase)
	writeTestFile(t, filepath.Join(root, "small.dat"), "12345", syncTestBase)               // 5 bajtów
	writeTestFile(t, filepath.Join(root, "big.dat"), strings.Repeat("x", 5000), syncTestBase) // 5000 bajtów
	writeTestFile(t, filepath.Join(root, "old.log"), "stary log\n", syncTestBase.Add(-48*time.Hour))
	return root
}

func TestSearchFilesByNamePattern(t *testing.T) {
	root := setupSearchTree(t)

	var got []string
	err := SearchFiles(root, SearchFilter{NamePattern: "*.go"}, func(m SearchMatch) {
		got = append(got, testRelOf(t, root, m.Path))
	}, nil)
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	sort.Strings(got)
	want := []string{"main.go", "utils/helper.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("trafienia = %v, oczekiwano %v", got, want)
	}
}

func TestSearchFilesEmptyPatternMatchesAll(t *testing.T) {
	root := setupSearchTree(t)

	count := 0
	err := SearchFiles(root, SearchFilter{}, func(m SearchMatch) {
		count++
		if m.LineNumber != 0 || m.LineText != "" {
			t.Errorf("bez filtra zawartości oczekiwano pustej linii, dostałem %d/%q", m.LineNumber, m.LineText)
		}
	}, nil)
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if count != 6 {
		t.Errorf("liczba trafień = %d, oczekiwano 6", count)
	}
}

func TestSearchFilesByContent(t *testing.T) {
	root := setupSearchTree(t)

	type hit struct {
		path string
		line int
		text string
	}
	var got []hit
	err := SearchFiles(root, SearchFilter{Content: "todo"}, func(m SearchMatch) {
		got = append(got, hit{testRelOf(t, root, m.Path), m.LineNumber, m.LineText})
	}, nil)
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}

	want := []hit{
		{"main.go", 3, "// TODO: popraw obsługę błędów"},
		{"utils/helper.go", 3, "// Todo: refaktoryzacja"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("trafienia = %+v, oczekiwano %+v", got, want)
	}
}

func TestSearchFilesContentCaseSensitive(t *testing.T) {
	root := setupSearchTree(t)

	count := 0
	err := SearchFiles(root, SearchFilter{Content: "todo", CaseSensitive: true}, func(m SearchMatch) {
		count++
	}, nil)
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if count != 0 {
		t.Errorf("case-sensitive \"todo\": liczba trafień = %d, oczekiwano 0", count)
	}

	count = 0
	err = SearchFiles(root, SearchFilter{Content: "TODO", CaseSensitive: true}, func(m SearchMatch) {
		count++
	}, nil)
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if count != 1 { // tylko main.go zawiera "TODO" wielkimi literami
		t.Errorf("case-sensitive \"TODO\": liczba trafień = %d, oczekiwano 1", count)
	}
}

func TestSearchFilesBySize(t *testing.T) {
	root := setupSearchTree(t)

	var names []string
	err := SearchFiles(root, SearchFilter{MinSize: 1000}, func(m SearchMatch) {
		names = append(names, testRelOf(t, root, m.Path))
	}, nil)
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if !reflect.DeepEqual(names, []string{"big.dat"}) {
		t.Errorf("MinSize=1000: trafienia = %v, oczekiwano [big.dat]", names)
	}

	names = nil
	err = SearchFiles(root, SearchFilter{MaxSize: 6}, func(m SearchMatch) {
		names = append(names, testRelOf(t, root, m.Path))
	}, nil)
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if !reflect.DeepEqual(names, []string{"small.dat"}) {
		t.Errorf("MaxSize=6: trafienia = %v, oczekiwano [small.dat]", names)
	}
}

func TestSearchFilesByModTime(t *testing.T) {
	root := setupSearchTree(t)

	var names []string
	err := SearchFiles(root, SearchFilter{Since: syncTestBase.Add(-time.Hour)}, func(m SearchMatch) {
		names = append(names, testRelOf(t, root, m.Path))
	}, nil)
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	sort.Strings(names)
	want := []string{"big.dat", "main.go", "notes.txt", "small.dat", "utils/helper.go"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("trafienia = %v, oczekiwano %v", names, want)
	}
}

func TestSearchFilesMultipleMatchesInFile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "list.txt"), "pierwsza\nmatch tutaj\nnic\ni znowu match\n", syncTestBase)

	var lines []int
	err := SearchFiles(root, SearchFilter{Content: "match"}, func(m SearchMatch) {
		lines = append(lines, m.LineNumber)
	}, nil)
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if !reflect.DeepEqual(lines, []int{2, 4}) {
		t.Errorf("linie = %v, oczekiwano [2 4]", lines)
	}
}

func TestSearchFilesCancelled(t *testing.T) {
	root := setupSearchTree(t)
	cancel := make(chan struct{})
	close(cancel)

	err := SearchFiles(root, SearchFilter{NamePattern: "*"}, func(m SearchMatch) {
		t.Error("onMatch nie powinno być wywołane po anulowaniu")
	}, cancel)
	if !errors.Is(err, errSearchCancelled) {
		t.Fatalf("err = %v, oczekiwano errSearchCancelled", err)
	}
}

func TestSearchFilesNilCallback(t *testing.T) {
	if err := SearchFiles(t.TempDir(), SearchFilter{}, nil, nil); err == nil {
		t.Fatal("oczekiwano błędu dla nil onMatch")
	}
}

func TestSearchFilesInvalidPattern(t *testing.T) {
	if err := SearchFiles(t.TempDir(), SearchFilter{NamePattern: "["}, func(SearchMatch) {}, nil); err == nil {
		t.Fatal("oczekiwano błędu dla nieprawidłowego wzorca")
	}
}