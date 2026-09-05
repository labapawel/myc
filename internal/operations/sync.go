package operations

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SyncStatus opisuje wynik zestawienia pary plików z lewego i prawego katalogu.
type SyncStatus string

const (
	StatusIdentical  SyncStatus = "IDENTICAL"
	StatusLeftNewer  SyncStatus = "LEFT_NEWER"
	StatusRightNewer SyncStatus = "RIGHT_NEWER"
	StatusLeftOnly   SyncStatus = "LEFT_ONLY"
	StatusRightOnly  SyncStatus = "RIGHT_ONLY"
	StatusSizeDiff   SyncStatus = "SIZE_DIFF"
)

// SyncAction to operacja do wykonania na parze plików.
type SyncAction string

const (
	ActionCopyToRight SyncAction = "COPY_TO_RIGHT"
	ActionCopyToLeft  SyncAction = "COPY_TO_LEFT"
	ActionDeleteLeft  SyncAction = "DELETE_LEFT"
	ActionDeleteRight SyncAction = "DELETE_RIGHT"
	ActionSkip        SyncAction = "SKIP"
)

// SyncItem opisuje jedną parę plików o wspólnej ścieżce względnej RelPath.
type SyncItem struct {
	RelPath   string
	LeftPath  string
	RightPath string
	LeftSize  int64
	RightSize int64
	LeftMod   time.Time
	RightMod  time.Time
	Status    SyncStatus
	Action    SyncAction
}

// syncFileMeta to minimalna informacja o pliku potrzebna do porównania.
type syncFileMeta struct {
	size int64
	mod  time.Time
}

// CompareDirectories rekurencyjnie skanuje leftDir i rightDir, zestawia pliki
// według ścieżek względnych i ustala status wraz z domyślną akcją:
//
//	LEFT_NEWER, LEFT_ONLY   -> COPY_TO_RIGHT
//	RIGHT_NEWER, RIGHT_ONLY -> COPY_TO_LEFT
//	IDENTICAL               -> SKIP
//	SIZE_DIFF               -> kopiowanie nowszej strony (przy remisie czasów lewej)
//
// Porównanie opiera się na metadanych (rozmiar i czas modyfikacji z dokładnością
// do sekund); zawartość plików nie jest hashowana. Wynik jest posortowany po RelPath.
func CompareDirectories(leftDir, rightDir string) ([]*SyncItem, error) {
	leftFiles, err := scanDirFiles(leftDir)
	if err != nil {
		return nil, fmt.Errorf("myc: skanowanie lewego katalogu %s: %w", leftDir, err)
	}
	rightFiles, err := scanDirFiles(rightDir)
	if err != nil {
		return nil, fmt.Errorf("myc: skanowanie prawego katalogu %s: %w", rightDir, err)
	}

	items := make([]*SyncItem, 0, len(leftFiles)+len(rightFiles))
	for rel, lm := range leftFiles {
		item := &SyncItem{
			RelPath:   rel,
			LeftPath:  filepath.Join(leftDir, filepath.FromSlash(rel)),
			RightPath: filepath.Join(rightDir, filepath.FromSlash(rel)),
			LeftSize:  lm.size,
			LeftMod:   lm.mod,
		}
		if rm, ok := rightFiles[rel]; ok {
			item.RightSize = rm.size
			item.RightMod = rm.mod
			decideStatusAndAction(item)
		} else {
			item.Status = StatusLeftOnly
			item.Action = ActionCopyToRight
		}
		items = append(items, item)
	}
	for rel, rm := range rightFiles {
		if _, ok := leftFiles[rel]; ok {
			continue
		}
		items = append(items, &SyncItem{
			RelPath:   rel,
			LeftPath:  filepath.Join(leftDir, filepath.FromSlash(rel)),
			RightPath: filepath.Join(rightDir, filepath.FromSlash(rel)),
			RightSize: rm.size,
			RightMod:  rm.mod,
			Status:    StatusRightOnly,
			Action:    ActionCopyToLeft,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].RelPath < items[j].RelPath
	})
	return items, nil
}

// scanDirFiles zbiera mapę: ścieżka względna (z ukośnikami) -> metadane pliku.
// Katalogi nie są zwracane; podkatalogi po stronie docelowej powstają przy kopiowaniu.
func scanDirFiles(root string) (map[string]syncFileMeta, error) {
	files := make(map[string]syncFileMeta)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = syncFileMeta{size: info.Size(), mod: info.ModTime()}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// decideStatusAndAction ustala status i akcję dla pliku obecnego po obu stronach.
func decideStatusAndAction(item *SyncItem) {
	// Porównujemy z dokładnością do sekund: nie każdy system plików
	// przechowuje ułamki sekund czasu modyfikacji.
	leftMod := item.LeftMod.Truncate(time.Second)
	rightMod := item.RightMod.Truncate(time.Second)

	if item.LeftSize != item.RightSize {
		item.Status = StatusSizeDiff
		switch {
		case leftMod.After(rightMod):
			item.Action = ActionCopyToRight
		case rightMod.After(leftMod):
			item.Action = ActionCopyToLeft
		default: // różne rozmiary, remis czasów – preferujemy stronę lewą
			item.Action = ActionCopyToRight
		}
		return
	}

	switch {
	case leftMod.After(rightMod):
		item.Status = StatusLeftNewer
		item.Action = ActionCopyToRight
	case rightMod.After(leftMod):
		item.Status = StatusRightNewer
		item.Action = ActionCopyToLeft
	default:
		item.Status = StatusIdentical
		item.Action = ActionSkip
	}
}

// ExecuteSync wykonuje akcje zapisane w items (kopiowanie lub usuwanie plików,
// z automatycznym tworzeniem podkatalogów docelowych). onProgress (jeśli nie nil)
// jest wywoływany przed wykonaniem każdej pozycji; current liczy się od 1,
// a total jest równe len(items).
func ExecuteSync(items []*SyncItem, onProgress func(current, total int, item *SyncItem)) error {
	total := len(items)
	for i, item := range items {
		if onProgress != nil {
			onProgress(i+1, total, item)
		}
		if err := executeSyncItem(item); err != nil {
			return err
		}
	}
	return nil
}

func executeSyncItem(item *SyncItem) error {
	switch item.Action {
	case ActionCopyToRight:
		if err := syncCopyFile(item.LeftPath, item.RightPath); err != nil {
			return fmt.Errorf("myc: %s: kopiowanie %s -> %s: %w", item.RelPath, item.LeftPath, item.RightPath, err)
		}
	case ActionCopyToLeft:
		if err := syncCopyFile(item.RightPath, item.LeftPath); err != nil {
			return fmt.Errorf("myc: %s: kopiowanie %s -> %s: %w", item.RelPath, item.RightPath, item.LeftPath, err)
		}
	case ActionDeleteLeft:
		if err := os.Remove(item.LeftPath); err != nil {
			return fmt.Errorf("myc: %s: usuwanie %s: %w", item.RelPath, item.LeftPath, err)
		}
	case ActionDeleteRight:
		if err := os.Remove(item.RightPath); err != nil {
			return fmt.Errorf("myc: %s: usuwanie %s: %w", item.RelPath, item.RightPath, err)
		}
	case ActionSkip, "":
		// celowo pomijamy
	default:
		return fmt.Errorf("myc: %s: nieznana akcja %q", item.RelPath, item.Action)
	}
	return nil
}

// syncCopyFile kopiuje src do dst, tworząc w razie potrzeby katalogi nadrzędne.
// Zapis odbywa się do pliku tymczasowego i kończy rename'em, więc przerwane
// kopiowanie nie uszkodzi istniejącego pliku docelowego. Kopiowane są także
// uprawnienia i czas modyfikacji, dzięki czemu powtórne porównanie katalogów
// zgłosi status IDENTICAL.
func syncCopyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("źródło %s jest katalogiem", src)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("tworzenie katalogu docelowego: %w", err)
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".myc-sync-tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chtimes(tmp, info.ModTime(), info.ModTime()); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}